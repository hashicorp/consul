package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/agent/grpc/resolver"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/tlsutil"
)

func TestNewDialer_WithTLSWrapper(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis.Close))

	builder := resolver.NewServerResolverBuilder(resolver.Config{})
	builder.AddServer(&metadata.Server{
		Name:       "server-1",
		ID:         "ID1",
		Datacenter: "dc1",
		Addr:       lis.Addr(),
		UseTLS:     true,
	})

	var called bool
	wrapper := func(_ string, conn net.Conn) (func(net.Conn) (net.Conn, error), bool) {
		called = true
		fn := func(conn net.Conn) (net.Conn, error) {
			return conn, nil
		}
		return fn, true
	}
	dial := newDialer(builder, wrapper)
	ctx := context.Background()
	conn, err := dial(ctx, lis.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.True(t, called, "expected TLSWrapper to be called")
}

func TestNewDialer_IntegrationWithTLSEnabledHandler(t *testing.T) {
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)

	srv := newTestServer(t, "server-1", "dc1")
	tlsConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		VerifyIncoming: true,
		VerifyOutgoing: true,
		CAFile:         "../../test/hostname/CertAuth.crt",
		CertFile:       "../../test/hostname/Alice.crt",
		KeyFile:        "../../test/hostname/Alice.key",
	}, hclog.New(nil))
	require.NoError(t, err)
	srv.rpc.tlsConf = tlsConf

	res.AddServer(srv.Metadata())
	t.Cleanup(srv.shutdown)

	pool := NewClientConnPool(res, TLSWrapper(tlsConf.OutgoingRPCWrapper()))

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.Equal(t, "server-1", resp.ServerName)
	require.True(t, atomic.LoadInt32(&srv.rpc.tlsConnEstablished) > 0)
}

// noOpTLSWrapper Generate a TLVWrapper that does not encypt anything, but is not nil
func noOpTLSWrapper() func(dc string, conn net.Conn) (func(net.Conn) (net.Conn, error), bool) {
	return func(dc string, conn net.Conn) (func(net.Conn) (net.Conn, error), bool) {
		return func(net.Conn) (net.Conn, error) {
			return conn, nil
		}, false
	}
}

func TestClientConnPool_IntegrationWithGRPCResolver_Failover(t *testing.T) {
	count := 4
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(res, noOpTLSWrapper())

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		srv := newTestServer(t, name, "dc1")
		res.AddServer(srv.Metadata())
		t.Cleanup(srv.shutdown)
	}

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	first, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)

	res.RemoveServer(&metadata.Server{ID: first.ServerName, Datacenter: "dc1"})

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.NotEqual(t, resp.ServerName, first.ServerName)
}

func newConfig(t *testing.T) resolver.Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return resolver.Config{Scheme: strings.ToLower(s)}
}

func TestClientConnPool_IntegrationWithGRPCResolver_Rebalance(t *testing.T) {
	count := 5
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(res, noOpTLSWrapper())

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		srv := newTestServer(t, name, "dc1")
		res.AddServer(srv.Metadata())
		t.Cleanup(srv.shutdown)
	}

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	first, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)

	t.Run("rebalance a different DC, does nothing", func(t *testing.T) {
		res.NewRebalancer("dc-other")()

		resp, err := client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		require.Equal(t, resp.ServerName, first.ServerName)
	})

	t.Run("rebalance the dc", func(t *testing.T) {
		// Rebalance is random, but if we repeat it a few times it should give us a
		// new server.
		attempts := 100
		for i := 0; i < attempts; i++ {
			res.NewRebalancer("dc1")()

			resp, err := client.Something(ctx, &testservice.Req{})
			require.NoError(t, err)
			if resp.ServerName != first.ServerName {
				return
			}
		}
		t.Fatalf("server was not rebalanced after %v attempts", attempts)
	})
}

func TestClientConnPool_IntegrationWithGRPCResolver_MultiDC(t *testing.T) {
	dcs := []string{"dc1", "dc2", "dc3"}

	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(res, noOpTLSWrapper())

	for _, dc := range dcs {
		name := "server-0-" + dc
		srv := newTestServer(t, name, dc)
		res.AddServer(srv.Metadata())
		t.Cleanup(srv.shutdown)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	for _, dc := range dcs {
		conn, err := pool.ClientConn(dc)
		require.NoError(t, err)
		client := testservice.NewSimpleClient(conn)

		resp, err := client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		require.Equal(t, resp.Datacenter, dc)
	}
}
