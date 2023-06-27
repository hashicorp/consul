// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/tcpproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/grpc-internal/balancer"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	"github.com/hashicorp/consul/agent/grpc-middleware/testutil/testservice"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

// useTLSForDcAlwaysTrue tell GRPC to always return the TLS is enabled
func useTLSForDcAlwaysTrue(_ string) bool {
	return true
}

func TestNewDialer_WithTLSWrapper(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis.Close))

	builder := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	builder.AddServer(types.AreaLAN, &metadata.Server{
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
	lis1, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis1.Close))

	lis2, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(logError(t, lis2.Close))

	// Send all of the traffic to dc2's server
	var p tcpproxy.Proxy
	gwAddr := ipaddr.FormatAddressPort("127.0.0.1", freeport.GetOne(t))
	p.AddRoute(gwAddr, tcpproxy.To(lis2.Addr().String()))
	p.AddStopACMESearch(gwAddr)
	require.NoError(t, p.Start())
	defer func() {
		p.Close()
		p.Wait()
	}()

	builder := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	builder.AddServer(types.AreaWAN, &metadata.Server{
		Name:       "server-1",
		ID:         "ID1",
		Datacenter: "dc1",
		Addr:       lis1.Addr(),
		UseTLS:     true,
	})
	builder.AddServer(types.AreaWAN, &metadata.Server{
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
	res := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)

	tlsConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		InternalRPC: tlsutil.ProtocolConfig{
			VerifyIncoming: true,
			CAFile:         "../../test/hostname/CertAuth.crt",
			CertFile:       "../../test/hostname/Alice.crt",
			KeyFile:        "../../test/hostname/Alice.key",
			VerifyOutgoing: true,
		},
	}, hclog.New(nil))
	require.NoError(t, err)

	srv := newSimpleTestServer(t, "server-1", "dc1", tlsConf)

	md := srv.Metadata()
	res.AddServer(types.AreaLAN, md)
	t.Cleanup(srv.shutdown)

	{
		// Put a duplicate instance of this on the WAN that will
		// fail if we accidentally use it.
		srv := newPanicTestServer(t, hclog.Default(), "server-1", "dc1", nil)
		res.AddServer(types.AreaWAN, srv.Metadata())
		t.Cleanup(srv.shutdown)
	}

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
	gwAddr := ipaddr.FormatAddressPort("127.0.0.1", freeport.GetOne(t))

	res := resolver.NewServerResolverBuilder(newConfig(t, "dc2", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)

	tlsConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		InternalRPC: tlsutil.ProtocolConfig{
			VerifyIncoming:       true,
			CAFile:               "../../test/hostname/CertAuth.crt",
			CertFile:             "../../test/hostname/Bob.crt",
			KeyFile:              "../../test/hostname/Bob.key",
			VerifyOutgoing:       true,
			VerifyServerHostname: true,
		},
		Domain:   "consul",
		NodeName: "bob",
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
	res.AddServer(types.AreaWAN, md)
	t.Cleanup(srv.shutdown)

	clientTLSConf, err := tlsutil.NewConfigurator(tlsutil.Config{
		InternalRPC: tlsutil.ProtocolConfig{
			VerifyIncoming:       true,
			CAFile:               "../../test/hostname/CertAuth.crt",
			CertFile:             "../../test/hostname/Betty.crt",
			KeyFile:              "../../test/hostname/Betty.key",
			VerifyOutgoing:       true,
			VerifyServerHostname: true,
		},
		Domain:   "consul",
		NodeName: "betty",
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
	res := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		{
			srv := newSimpleTestServer(t, name, "dc1", nil)
			res.AddServer(types.AreaLAN, srv.Metadata())
			t.Cleanup(srv.shutdown)
		}
		{
			// Put a duplicate instance of this on the WAN that will
			// fail if we accidentally use it.
			srv := newPanicTestServer(t, hclog.Default(), name, "dc1", nil)
			res.AddServer(types.AreaWAN, srv.Metadata())
			t.Cleanup(srv.shutdown)
		}
	}

	conn, err := pool.ClientConn("dc1")
	require.NoError(t, err)
	client := testservice.NewSimpleClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	first, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)

	res.RemoveServer(types.AreaLAN, &metadata.Server{ID: first.ServerName, Datacenter: "dc1"})

	resp, err := client.Something(ctx, &testservice.Req{})
	require.NoError(t, err)
	require.NotEqual(t, resp.ServerName, first.ServerName)
}

func TestClientConnPool_ForwardToLeader_Failover(t *testing.T) {
	count := 3
	res := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)
	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	var servers []testServer
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("server-%d", i)
		{
			srv := newSimpleTestServer(t, name, "dc1", nil)
			res.AddServer(types.AreaLAN, srv.Metadata())
			servers = append(servers, srv)
			t.Cleanup(srv.shutdown)
		}
		{
			// Put a duplicate instance of this on the WAN that will
			// fail if we accidentally use it.
			srv := newPanicTestServer(t, hclog.Default(), name, "dc1", nil)
			res.AddServer(types.AreaWAN, srv.Metadata())
			t.Cleanup(srv.shutdown)
		}
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

func newConfig(t *testing.T, dc, agentType string) resolver.Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return resolver.Config{
		Datacenter: dc,
		AgentType:  agentType,
		Authority:  strings.ToLower(s),
	}
}

func TestClientConnPool_IntegrationWithGRPCResolver_MultiDC(t *testing.T) {
	dcs := []string{"dc1", "dc2", "dc3"}

	res := resolver.NewServerResolverBuilder(newConfig(t, "dc1", "server"))
	bb := balancer.NewBuilder(res.Authority(), testutil.Logger(t))
	registerWithGRPC(t, res, bb)

	pool := NewClientConnPool(ClientConnPoolConfig{
		Servers:               res,
		UseTLSForDC:           useTLSForDcAlwaysTrue,
		DialingFromServer:     true,
		DialingFromDatacenter: "dc1",
	})

	for _, dc := range dcs {
		name := "server-0-" + dc
		srv := newSimpleTestServer(t, name, dc, nil)
		if dc == "dc1" {
			res.AddServer(types.AreaLAN, srv.Metadata())
			// Put a duplicate instance of this on the WAN that will
			// fail if we accidentally use it.
			srvBad := newPanicTestServer(t, hclog.Default(), name, dc, nil)
			res.AddServer(types.AreaWAN, srvBad.Metadata())
			t.Cleanup(srvBad.shutdown)
		} else {
			res.AddServer(types.AreaWAN, srv.Metadata())
		}
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

func registerWithGRPC(t *testing.T, rb *resolver.ServerResolverBuilder, bb *balancer.Builder) {
	resolver.Register(rb)
	bb.Register()
	t.Cleanup(func() {
		resolver.Deregister(rb.Authority())
		bb.Deregister()
	})
}
