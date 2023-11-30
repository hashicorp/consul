// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package balancer

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/grpc-middleware/testutil/testservice"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestBalancer(t *testing.T) {
	t.Run("remains pinned to the same server", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		server1 := runServer(t, "server1")
		server2 := runServer(t, "server2")

		target, authority, _ := stubResolver(t, server1, server2)

		balancerBuilder := NewBuilder(authority, testutil.Logger(t))
		balancerBuilder.Register()
		t.Cleanup(balancerBuilder.Deregister)

		conn := dial(t, target)
		client := testservice.NewSimpleClient(conn)

		var serverName string
		for i := 0; i < 5; i++ {
			rsp, err := client.Something(ctx, &testservice.Req{})
			require.NoError(t, err)

			if i == 0 {
				serverName = rsp.ServerName
			} else {
				require.Equal(t, serverName, rsp.ServerName)
			}
		}

		var pinnedServer, otherServer *server
		switch serverName {
		case server1.name:
			pinnedServer, otherServer = server1, server2
		case server2.name:
			pinnedServer, otherServer = server2, server1
		}
		require.Equal(t, 1,
			pinnedServer.openConnections(),
			"pinned server should have 1 connection",
		)
		require.Zero(t,
			otherServer.openConnections(),
			"other server should have no connections",
		)
	})

	t.Run("switches server on-error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		server1 := runServer(t, "server1")
		server2 := runServer(t, "server2")

		target, authority, _ := stubResolver(t, server1, server2)

		balancerBuilder := NewBuilder(authority, testutil.Logger(t))
		balancerBuilder.Register()
		t.Cleanup(balancerBuilder.Deregister)

		conn := dial(t, target)
		client := testservice.NewSimpleClient(conn)

		// Figure out which server we're talking to now, and which we should switch to.
		rsp, err := client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)

		var initialServer, otherServer *server
		switch rsp.ServerName {
		case server1.name:
			initialServer, otherServer = server1, server2
		case server2.name:
			initialServer, otherServer = server2, server1
		}

		// Next request should fail (we don't have retries configured).
		initialServer.err = status.Error(codes.ResourceExhausted, "rate limit exceeded")
		_, err = client.Something(ctx, &testservice.Req{})
		require.Error(t, err)

		// Following request should succeed (against the other server).
		rsp, err = client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		require.Equal(t, otherServer.name, rsp.ServerName)

		retry.Run(t, func(r *retry.R) {
			require.Zero(r,
				initialServer.openConnections(),
				"connection to previous server should have been torn down",
			)
		})
	})

	t.Run("rebalance changes the server", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		server1 := runServer(t, "server1")
		server2 := runServer(t, "server2")

		target, authority, _ := stubResolver(t, server1, server2)

		balancerBuilder := NewBuilder(authority, testutil.Logger(t))
		balancerBuilder.Register()
		t.Cleanup(balancerBuilder.Deregister)

		// Provide a custom prioritizer that causes Rebalance to choose whichever
		// server didn't get our first request.
		var otherServer *server
		balancerBuilder.shuffler = func(addrs []resolver.Address) {
			sort.Slice(addrs, func(a, b int) bool {
				return addrs[a].Addr == otherServer.addr
			})
		}

		conn := dial(t, target)
		client := testservice.NewSimpleClient(conn)

		// Figure out which server we're talking to now.
		rsp, err := client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)

		var initialServer *server
		switch rsp.ServerName {
		case server1.name:
			initialServer, otherServer = server1, server2
		case server2.name:
			initialServer, otherServer = server2, server1
		}

		// Trigger a rebalance.
		targetURL, err := url.Parse(target)
		require.NoError(t, err)
		balancerBuilder.Rebalance(resolver.Target{URL: *targetURL})

		// Following request should hit the other server.
		rsp, err = client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		require.Equal(t, otherServer.name, rsp.ServerName)

		retry.Run(t, func(r *retry.R) {
			require.Zero(r,
				initialServer.openConnections(),
				"connection to previous server should have been torn down",
			)
		})
	})

	t.Run("resolver removes the server", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		server1 := runServer(t, "server1")
		server2 := runServer(t, "server2")

		target, authority, res := stubResolver(t, server1, server2)

		balancerBuilder := NewBuilder(authority, testutil.Logger(t))
		balancerBuilder.Register()
		t.Cleanup(balancerBuilder.Deregister)

		conn := dial(t, target)
		client := testservice.NewSimpleClient(conn)

		// Figure out which server we're talking to now.
		rsp, err := client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		var initialServer, otherServer *server
		switch rsp.ServerName {
		case server1.name:
			initialServer, otherServer = server1, server2
		case server2.name:
			initialServer, otherServer = server2, server1
		}

		// Remove the server's address.
		res.UpdateState(resolver.State{
			Addresses: []resolver.Address{
				{Addr: otherServer.addr},
			},
		})

		// Following request should hit the other server.
		rsp, err = client.Something(ctx, &testservice.Req{})
		require.NoError(t, err)
		require.Equal(t, otherServer.name, rsp.ServerName)

		retry.Run(t, func(r *retry.R) {
			require.Zero(r,
				initialServer.openConnections(),
				"connection to previous server should have been torn down",
			)
		})

		// Remove the other server too.
		res.UpdateState(resolver.State{
			Addresses: []resolver.Address{},
		})

		_, err = client.Something(ctx, &testservice.Req{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "resolver produced no addresses")

		retry.Run(t, func(r *retry.R) {
			require.Zero(r,
				otherServer.openConnections(),
				"connection to other server should have been torn down",
			)
		})
	})
}

func stubResolver(t *testing.T, servers ...*server) (string, string, *manual.Resolver) {
	t.Helper()

	addresses := make([]resolver.Address, len(servers))
	for idx, s := range servers {
		addresses[idx] = resolver.Address{Addr: s.addr}
	}

	scheme := fmt.Sprintf("consul-%d-%d", time.Now().UnixNano(), rand.Int())

	r := manual.NewBuilderWithScheme(scheme)
	r.InitialState(resolver.State{Addresses: addresses})

	resolver.Register(r)
	t.Cleanup(func() { resolver.UnregisterForTesting(scheme) })

	authority, err := uuid.GenerateUUID()
	require.NoError(t, err)

	return fmt.Sprintf("%s://%s", scheme, authority), authority, r
}

func runServer(t *testing.T, name string) *server {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &server{
		name: name,
		addr: lis.Addr().String(),
	}

	gs := grpc.NewServer(
		grpc.StatsHandler(s),
	)
	testservice.RegisterSimpleServer(gs, s)
	go gs.Serve(lis)

	var once sync.Once
	s.shutdown = func() { once.Do(gs.Stop) }
	t.Cleanup(s.shutdown)

	return s
}

type server struct {
	name string
	addr string
	err  error

	c        int32
	shutdown func()
}

func (s *server) openConnections() int { return int(atomic.LoadInt32(&s.c)) }

func (*server) HandleRPC(context.Context, stats.RPCStats)                         {}
func (*server) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context { return ctx }
func (*server) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context   { return ctx }

func (s *server) HandleConn(_ context.Context, cs stats.ConnStats) {
	switch cs.(type) {
	case *stats.ConnBegin:
		atomic.AddInt32(&s.c, 1)
	case *stats.ConnEnd:
		atomic.AddInt32(&s.c, -1)
	}
}

func (*server) Flow(*testservice.Req, testservice.Simple_FlowServer) error { return nil }

func (s *server) Something(context.Context, *testservice.Req) (*testservice.Resp, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &testservice.Resp{ServerName: s.name}, nil
}

func dial(t *testing.T, target string) *grpc.ClientConn {
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(
			fmt.Sprintf(`{"loadBalancingConfig":[{"%s":{}}]}`, BuilderName),
		),
	)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf("error closing connection: %v", err)
		}
	})
	require.NoError(t, err)
	return conn
}
