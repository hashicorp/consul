package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/tlsutil"
)

type testServer struct {
	addr     net.Addr
	name     string
	dc       string
	shutdown func()
	rpc      *fakeRPCListener
}

func (s testServer) Metadata() *metadata.Server {
	return &metadata.Server{
		ID:         s.name,
		Name:       s.name + "." + s.dc,
		ShortName:  s.name,
		Datacenter: s.dc,
		Addr:       s.addr,
		UseTLS:     s.rpc.tlsConf != nil,
	}
}

func newTestServer(t *testing.T, name string, dc string, tlsConf *tlsutil.Configurator) testServer {
	addr := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	handler := NewHandler(addr, func(server *grpc.Server) {
		testservice.RegisterSimpleServer(server, &simple{name: name, dc: dc})
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	rpc := &fakeRPCListener{t: t, handler: handler, tlsConf: tlsConf}

	g := errgroup.Group{}
	g.Go(func() error {
		if err := rpc.listen(lis); err != nil {
			return fmt.Errorf("fake rpc listen error: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := handler.Run(); err != nil {
			return fmt.Errorf("grpc server error: %w", err)
		}
		return nil
	})
	return testServer{
		addr: lis.Addr(),
		name: name,
		dc:   dc,
		rpc:  rpc,
		shutdown: func() {
			rpc.shutdown = true
			if err := lis.Close(); err != nil {
				t.Logf("listener closed with error: %v", err)
			}
			if err := handler.Shutdown(); err != nil {
				t.Logf("grpc server shutdown: %v", err)
			}
			if err := g.Wait(); err != nil {
				t.Log(err)
			}
		},
	}
}

type simple struct {
	name string
	dc   string
}

func (s *simple) Flow(_ *testservice.Req, flow testservice.Simple_FlowServer) error {
	for flow.Context().Err() == nil {
		resp := &testservice.Resp{ServerName: "one", Datacenter: s.dc}
		if err := flow.Send(resp); err != nil {
			return err
		}
		time.Sleep(time.Millisecond)
	}
	return nil
}

func (s *simple) Something(_ context.Context, _ *testservice.Req) (*testservice.Resp, error) {
	return &testservice.Resp{ServerName: s.name, Datacenter: s.dc}, nil
}

// fakeRPCListener mimics agent/consul.Server.listen to handle the RPCType byte.
// In the future we should be able to refactor Server and extract this RPC
// handling logic so that we don't need to use a fake.
// For now, since this logic is in agent/consul, we can't easily use Server.listen
// so we fake it.
type fakeRPCListener struct {
	t                   *testing.T
	handler             *Handler
	shutdown            bool
	tlsConf             *tlsutil.Configurator
	tlsConnEstablished  int32
	alpnConnEstablished int32
}

func (f *fakeRPCListener) listen(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if f.shutdown {
				return nil
			}
			return err
		}

		go f.handleConn(conn)
	}
}

func (f *fakeRPCListener) handleConn(conn net.Conn) {
	if f.tlsConf != nil && f.tlsConf.MutualTLSCapable() {
		// See if actually this is native TLS multiplexed onto the old
		// "type-byte" system.

		peekedConn, nativeTLS, err := pool.PeekForTLS(conn)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("ERROR: failed to read first byte: %v\n", err)
			}
			conn.Close()
			return
		}

		if nativeTLS {
			f.handleNativeTLSConn(peekedConn)
			return
		}
		conn = peekedConn
	}

	buf := make([]byte, 1)

	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			fmt.Println("ERROR", err.Error())
		}
		conn.Close()
		return
	}
	typ := pool.RPCType(buf[0])

	switch typ {

	case pool.RPCGRPC:
		f.handler.Handle(conn)
		return

	case pool.RPCTLS:
		// occasionally we see a test client connecting to an rpc listener that
		// was created as part of another test, despite none of the tests running
		// in parallel.
		// Maybe some strange grpc behaviour? I'm not sure.
		if f.tlsConf == nil {
			fmt.Println("ERROR: tls is not configured")
			conn.Close()
			return
		}

		atomic.AddInt32(&f.tlsConnEstablished, 1)
		conn = tls.Server(conn, f.tlsConf.IncomingRPCConfig())
		f.handleConn(conn)

	default:
		fmt.Println("ERROR: unexpected byte", typ)
		conn.Close()
	}
}

func (f *fakeRPCListener) handleNativeTLSConn(conn net.Conn) {
	tlscfg := f.tlsConf.IncomingALPNRPCConfig(pool.RPCNextProtos)
	tlsConn := tls.Server(conn, tlscfg)

	// Force the handshake to conclude.
	if err := tlsConn.Handshake(); err != nil {
		fmt.Printf("ERROR: TLS handshake failed: %v", err)
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Time{})

	var (
		cs        = tlsConn.ConnectionState()
		nextProto = cs.NegotiatedProtocol
	)

	switch nextProto {
	case pool.ALPN_RPCGRPC:
		atomic.AddInt32(&f.alpnConnEstablished, 1)
		f.handler.Handle(tlsConn)

	default:
		fmt.Printf("ERROR: discarding RPC for unknown negotiated protocol %q\n", nextProto)
		conn.Close()
	}
}
