package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/tcpproxy"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/agent/grpc/resolver"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/tlsutil"
)

// useTLSForDcAlwaysTrue tell GRPC to always return the TLS is enabled
func useTLSForDcAlwaysTrue(_ string) bool {
	return true
}

func TestNewDialer_WithTLSWrapper(t *testing.T) {
	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	lis, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(ports[0])))
	require.NoError(t, err)
	t.Cleanup(logError(t, lis.Close))

	builder := resolver.NewServerResolverBuilder(newConfig(t))
	builder.AddServer(&metadata.Server{
		Name:       "server-1",
		ID:         "ID1",
		Datacenter: "dc1",
		Addr:       lis.Addr(),
		UseTLS:     true,
	})

	var called bool
	wrapper := func(_ string, conn net.Conn) (net.Conn, error) {
		called = true
		return conn, nil
	}
	dial := newDialer(
		ClientConnPoolConfig{
			Servers:               builder,
			TLSWrapper:            wrapper,
			UseTLSForDC:           useTLSForDcAlwaysTrue,
			DialingFromServer:     true,
			DialingFromDatacenter: "dc1",
		},
		&gatewayResolverDep{},
	)
	ctx := context.Background()
	conn, err := dial(ctx, resolver.DCPrefix("dc1", lis.Addr().String()))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.True(t, called, "expected TLSWrapper to be called")
}

func TestNewDialer_WithALPNWrapper(t *testing.T) {
	ports := freeport.MustTake(3)
	defer freeport.Return(ports)

	var (
		s1addr = ipaddr.FormatAddressPort("127.0.0.1", ports[0])
		s2addr = ipaddr.FormatAddressPort("127.0.0.1", ports[1])
		gwAddr = ipaddr.FormatAddressPort("127.0.0.1", ports[2])
	)

	lis1, err := net.Listen("tcp", s1addr)
	require.NoError(t, err)
	t.Cleanup(logError(t, lis1.Close))

	lis2, err := net.Listen("tcp", s2addr)
	require.NoError(t, err)
	t.Cleanup(logError(t, lis2.Close))

	// Send all of the traffic to dc2's server
	var p tcpproxy.Proxy
	p.AddRoute(gwAddr, tcpproxy.To(s2addr))
	p.AddStopACMESearch(gwAddr)
	require.NoError(t, p.Start())
	defer func() {
		p.Close()
		p.Wait()
	}()

	builder := resolver.NewServerResolverBuilder(newConfig(t))
	builder.AddServer(&metadata.Server{
		Name:       "server-1",
		ID:         "ID1",
		Datacenter: "dc1",
		Addr:       lis1.Addr(),
		UseTLS:     true,
	})
	builder.AddServer(&metadata.Server{
		Name:       "server-2",
		ID:         "ID2",
		Datacenter: "dc2",
		Addr:       lis2.Addr(),
		UseTLS:     true,
	})

	var calledTLS bool
	wrapperTLS := func(_ string, conn net.Conn) (net.Conn, error) {
		calledTLS = true
		return conn, nil
	}
	var calledALPN bool
	wrapperALPN := func(_, _, _ string, conn net.Conn) (net.Conn, error) {
		calledALPN = true
		return conn, nil
	}
	gwResolverDep := &gatewayResolverDep{
		GatewayResolver: func(addr string) string {
			return gwAddr
		},
	}
	dial := newDialer(
		ClientConnPoolConfig{
			Servers:               builder,
			TLSWrapper:            wrapperTLS,
			ALPNWrapper:           wrapperALPN,
			UseTLSForDC:           useTLSForDcAlwaysTrue,
			DialingFromServer:     true,
			DialingFromDatacenter: "dc1",
		},
		gwResolverDep,
	)

	ctx := context.Background()
	conn, err := dial(ctx, resolver.DCPrefix("dc2", lis2.Addr().String()))
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	assert.False(t, calledTLS, "expected TLSWrapper not to be called")
	assert.True(t, calledALPN, "expected ALPNWrapper to be called")
}

func TestNewDialer_IntegrationWithTLSEnabledHandler(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)

	tlsConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		VerifyIncoming: true,
		VerifyOutgoing: true,
		CAFile:         "../../test/hostname/CertAuth.crt",
		CertFile:       "../../test/hostname/Alice.crt",
		KeyFile:        "../../test/hostname/Alice.key",
	}, hclog.New(nil))
	require.NoError(t, err)

	srv := newSimpleTestServer(t, "server-1", "dc1", tlsConf)

	md := srv.Metadata()
	res.AddServer(md)
	t.Cleanup(srv.shutdown)

	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		TLSWrapper:            TLSWrapper(tlsConf.OutgoingRPCWrapper()),
		UseTLSForDC:           tlsConf.UseTLS,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.Equal(t, "server-1", resp.ServerName)
	require.True(t, atomic.LoadInt32(&srv.rpc.tlsConnEstablished) > 0)
	require.True(t, atomic.LoadInt32(&srv.rpc.alpnConnEstablished) == 0)
}

func TestNewDialer_IntegrationWithTLSEnabledHandler_viaMeshGateway(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	gwAddr := ipaddr.FormatAddressPort("127.0.0.1", ports[0])

	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)

	tlsConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		VerifyIncoming:       true,
		VerifyOutgoing:       true,
		VerifyServerHostname: true,
		CAFile:               "../../test/hostname/CertAuth.crt",
		CertFile:             "../../test/hostname/Bob.crt",
		KeyFile:              "../../test/hostname/Bob.key",
		Domain:               "consul",
		NodeName:             "bob",
	}, hclog.New(nil))
	require.NoError(t, err)

	srv := newSimpleTestServer(t, "bob", "dc1", tlsConf)

	// Send all of the traffic to dc1's server
	var p tcpproxy.Proxy
	p.AddRoute(gwAddr, tcpproxy.To(srv.addr.String()))
	p.AddStopACMESearch(gwAddr)
	require.NoError(t, p.Start())
	defer func() {
		p.Close()
		p.Wait()
	}()

	md := srv.Metadata()
	res.AddServer(md)
	t.Cleanup(srv.shutdown)

	clientTLSConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		VerifyIncoming:       true,
		VerifyOutgoing:       true,
		VerifyServerHostname: true,
		CAFile:               "../../test/hostname/CertAuth.crt",
		CertFile:             "../../test/hostname/Betty.crt",
		KeyFile:              "../../test/hostname/Betty.key",
		Domain:               "consul",
		NodeName:             "betty",
	}, hclog.New(nil))
	require.NoError(t, err)

	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		TLSWrapper:            TLSWrapper(clientTLSConf.OutgoingRPCWrapper()),
		ALPNWrapper:           ALPNWrapper(clientTLSConf.OutgoingALPNRPCWrapper()),
		UseTLSForDC:           tlsConf.UseTLS,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc2",
	})
	pool.SetGatewayResolver(func(addr string) string {
		return gwAddr
	})

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.Equal(t, "bob", resp.ServerName)
	require.True(t, atomic.LoadInt32(&srv.rpc.tlsConnEstablished) == 0)
	require.True(t, atomic.LoadInt32(&srv.rpc.alpnConnEstablished) > 0)
}

func TestClientConnPool_IntegrationWithGRPCResolver_Failover(t *testing.T) {
	count := 4
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		srv := newSimpleTestServer(t, name, "dc1", nil)
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

func TestClientConnPool_ForwardToLeader_Failover(t *testing.T) {
	count := 3
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	var servers []testServer
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		srv := newSimpleTestServer(t, name, "dc1", nil)
		res.AddServer(srv.Metadata())
		servers = append(servers, srv)
		t.Cleanup(srv.shutdown)
	}

	// Set the leader address to the first server.
	srv0 := servers[0].Metadata()
	res.UpdateLeaderAddr(srv0.Datacenter, srv0.Addr.String())

	conn, err := pool.ClientConnLeader()
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	first, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.Equal(t, first.ServerName, servers[0].name)

	// Update the leader address and make another request.
	srv1 := servers[1].Metadata()
	res.UpdateLeaderAddr(srv1.Datacenter, srv1.Addr.String())

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.Equal(t, resp.ServerName, servers[1].name)
}

func newConfig(t *testing.T) resolver.Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return resolver.Config{Authority: strings.ToLower(s)}
}

func TestClientConnPool_IntegrationWithGRPCResolver_Rebalance(t *testing.T) {
	count := 5
	res := resolver.NewServerResolverBuilder(newConfig(t))
	registerWithGRPC(t, res)
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		srv := newSimpleTestServer(t, name, "dc1", nil)
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
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	for _, dc := range dcs {
		name := "server-0-" + dc
		srv := newSimpleTestServer(t, name, dc, nil)
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

func registerWithGRPC(t *testing.T, b *resolver.ServerResolverBuilder) {
	resolver.Register(b)
	t.Cleanup(func() {
		resolver.Deregister(b.Authority())
	})
}
