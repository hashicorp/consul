package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/grpc/internal/testservice"
	"github.com/hashicorp/consul/agent/grpc/resolver"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/stretchr/testify/require"
)

func TestNewDialer(t *testing.T) {
	// TODO: conn is closed on errors
	// TODO: with TLS enabled
}

func TestClientConnPool_IntegrationWithGRPCResolver_Failover(t *testing.T) {
	count := 4
	cfg := resolver.Config{Datacenter: "dc1", Scheme: newScheme(t.Name())}
	res := resolver.NewServerResolverBuilder(cfg, fakeNodes{num: count})
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

type fakeNodes struct {
	num int
}

func (n fakeNodes) NumNodes() int {
	return n.num
}

func TestClientConnPool_IntegrationWithGRPCResolver_MultiDC(t *testing.T) {
	dcs := []string{"dc1", "dc2", "dc3"}

	cfg := resolver.Config{Datacenter: "dc1", Scheme: newScheme(t.Name())}
	res := resolver.NewServerResolverBuilder(cfg, fakeNodes{num: 1})
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
