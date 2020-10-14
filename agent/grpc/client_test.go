package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/agent/grpc/resolver"
	"github.com/hashicorp/consul/agent/metadata"
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
	wrapper := func(_ string, conn net.Conn) (net.Conn, error) {
		called = true
		return conn, nil
	}
	dial := newDialer(builder, wrapper)
	ctx := context.Background()
	conn, err := dial(ctx, lis.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.True(t, called, "expected TLSWrapper to be called")
}

// TODO: integration test TestNewDialer with TLS and rcp server, when the rpc
// exists as an isolated component.

func TestClientConnPool_IntegrationWithGRPCResolver_Failover(t *testing.T) {
	count := 4
	cfg := resolver.Config{Scheme: newScheme(t.Name())}
	res := resolver.NewServerResolverBuilder(cfg)
	resolver.RegisterWithGRPC(res)
	pool := NewClientConnPool(res, nil)

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

func newScheme(n string) string {
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return strings.ToLower(s)
}

func TestClientConnPool_IntegrationWithGRPCResolver_Rebalance(t *testing.T) {
	count := 5
	cfg := resolver.Config{Scheme: newScheme(t.Name())}
	res := resolver.NewServerResolverBuilder(cfg)
	resolver.RegisterWithGRPC(res)
	pool := NewClientConnPool(res, nil)

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

	cfg := resolver.Config{Scheme: newScheme(t.Name())}
	res := resolver.NewServerResolverBuilder(cfg)
	resolver.RegisterWithGRPC(res)
	pool := NewClientConnPool(res, nil)

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
