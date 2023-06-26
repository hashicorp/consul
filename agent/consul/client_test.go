// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/balancer"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
)

func testClientConfig(t *testing.T) (string, *Config) {
	dir := testutil.TempDir(t, "consul")
	config := DefaultConfig()

	ports := freeport.GetN(t, 2)
	config.Datacenter = "dc1"
	config.DataDir = dir
	config.NodeName = uniqueNodeName(t.Name())
	config.RPCAddr = &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: ports[0],
	}
	config.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfLANConfig.MemberlistConfig.BindPort = ports[1]
	config.SerfLANConfig.MemberlistConfig.ProbeTimeout = 200 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.ProbeInterval = time.Second
	config.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond
	return dir, config
}

func testClient(t *testing.T) (string, *Client) {
	return testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
	})
}

func testClientDC(t *testing.T, dc string) (string, *Client) {
	return testClientWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.NodeName = uniqueNodeName(t.Name())
	})
}

func testClientWithConfigWithErr(t *testing.T, cb func(c *Config)) (string, *Client, error) {
	dir, config := testClientConfig(t)
	if cb != nil {
		cb(config)
	}

	// Apply config to copied fields because many tests only set the old
	// values.
	config.ACLResolverSettings.ACLsEnabled = config.ACLsEnabled
	config.ACLResolverSettings.NodeName = config.NodeName
	config.ACLResolverSettings.Datacenter = config.Datacenter
	config.ACLResolverSettings.EnterpriseMeta = *config.AgentEnterpriseMeta()

	client, err := NewClient(config, newDefaultDeps(t, config))
	return dir, client, err
}

func testClientWithConfig(t *testing.T, cb func(c *Config)) (string, *Client) {
	dir, client, err := testClientWithConfigWithErr(t, cb)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, client
}

func TestClient_StartStop(t *testing.T) {
	t.Parallel()
	dir, client := testClient(t)
	defer os.RemoveAll(dir)

	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClient_JoinLAN(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	// Try to join
	joinLAN(t, c1, s1)
	testrpc.WaitForTestAgent(t, c1.RPC, "dc1")
	retry.Run(t, func(r *retry.R) {
		if got, want := c1.router.GetLANManager().NumServers(), 1; got != want {
			r.Fatalf("got %d servers want %d", got, want)
		}
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
	})
}

func TestClient_LANReap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)

	dir2, c1 := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.SerfFloodInterval = 100 * time.Millisecond
		c.SerfLANConfig.ReconnectTimeout = 250 * time.Millisecond
		c.SerfLANConfig.TombstoneTimeout = 250 * time.Millisecond
		c.SerfLANConfig.ReapInterval = 500 * time.Millisecond
	})
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	joinLAN(t, c1, s1)
	testrpc.WaitForLeader(t, c1.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.LANMembersInAgentPartition(), 2)
		require.Len(r, c1.LANMembersInAgentPartition(), 2)
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		server := c1.router.FindLANServer()
		require.NotNil(r, server)
		require.Equal(r, s1.config.NodeName, server.Name)
	})

	// shutdown the second dc
	s1.Shutdown()

	retry.Run(t, func(r *retry.R) {
		require.Len(r, c1.LANMembersInAgentPartition(), 1)
		server := c1.router.FindLANServer()
		require.Nil(r, server)
	})
}

func TestClient_JoinLAN_Invalid(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClientDC(t, "other")
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	if _, err := c1.JoinLAN([]string{joinAddrLAN(s1)}, nil); err == nil {
		t.Fatal("should error")
	}

	time.Sleep(50 * time.Millisecond)
	if len(s1.LANMembersInAgentPartition()) != 1 {
		t.Fatalf("should not join")
	}
	if len(c1.LANMembersInAgentPartition()) != 1 {
		t.Fatalf("should not join")
	}
}

func TestClient_JoinWAN_Invalid(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClientDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	if _, err := c1.JoinLAN([]string{joinAddrWAN(s1)}, nil); err == nil {
		t.Fatal("should error")
	}

	time.Sleep(50 * time.Millisecond)
	if len(s1.WANMembers()) != 1 {
		t.Fatalf("should not join")
	}
	if len(c1.LANMembersInAgentPartition()) != 1 {
		t.Fatalf("should not join")
	}
}

func TestClient_RPC(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try an RPC
	var out struct{}
	if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != structs.ErrNoServers {
		t.Fatalf("err: %v", err)
	}

	// Try to join
	joinLAN(t, c1, s1)

	// Check the members
	if len(s1.LANMembersInAgentPartition()) != 2 {
		t.Fatalf("bad len")
	}

	if len(c1.LANMembersInAgentPartition()) != 2 {
		t.Fatalf("bad len")
	}

	// RPC should succeed
	retry.Run(t, func(r *retry.R) {
		if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			r.Fatal("ping failed", err)
		}
	})
}

type leaderFailer struct {
	totalCalls int
	onceCalls  int
}

func (l *leaderFailer) Always(args struct{}, reply *struct{}) error {
	l.totalCalls++
	return structs.ErrNoLeader
}

func (l *leaderFailer) Once(args struct{}, reply *struct{}) error {
	l.totalCalls++
	l.onceCalls++

	switch {
	case l.onceCalls == 1:
		return structs.ErrNoLeader

	default:
		return nil
	}
}

func TestClient_RPC_Retry(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.RPCHoldTimeout = 2 * time.Second
	})
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
	})

	failer := &leaderFailer{}
	if err := s1.RegisterEndpoint("Fail", failer); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out struct{}
	if err := c1.RPC(context.Background(), "Fail.Always", struct{}{}, &out); !structs.IsErrNoLeader(err) {
		t.Fatalf("err: %v", err)
	}
	if got, want := failer.totalCalls, 2; got < want {
		t.Fatalf("got %d want >= %d", got, want)
	}
	if err := c1.RPC(context.Background(), "Fail.Once", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := failer.onceCalls, 2; got < want {
		t.Fatalf("got %d want >= %d", got, want)
	}
	if got, want := failer.totalCalls, 4; got < want {
		t.Fatalf("got %d want >= %d", got, want)
	}
}

func TestClient_RPC_Pool(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join.
	joinLAN(t, c1, s1)

	// Wait for both agents to finish joining
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
	})

	// Blast out a bunch of RPC requests at the same time to try to get
	// contention opening new connections.
	var wg sync.WaitGroup
	for i := 0; i < 150; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			var out struct{}
			retry.Run(t, func(r *retry.R) {
				if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
					r.Fatal("ping failed", err)
				}
			})
		}()
	}

	wg.Wait()
}

func TestClient_RPC_ConsulServerPing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	var servers []*Server
	const numServers = 5

	for n := 0; n < numServers; n++ {
		bootstrap := n == 0
		dir, s := testServerDCBootstrap(t, "dc1", bootstrap)
		defer os.RemoveAll(dir)
		defer s.Shutdown()

		servers = append(servers, s)
	}

	const numClients = 1
	clientDir, c := testClient(t)
	defer os.RemoveAll(clientDir)
	defer c.Shutdown()

	// Join all servers.
	for _, s := range servers {
		joinLAN(t, c, s)
	}

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, numServers)) })
	}

	// Sleep to allow Serf to sync, shuffle, and let the shuffle complete
	c.router.GetLANManager().ResetRebalanceTimer()
	time.Sleep(time.Second)

	if len(c.LANMembersInAgentPartition()) != numServers+numClients {
		t.Errorf("bad len: %d", len(c.LANMembersInAgentPartition()))
	}
	for _, s := range servers {
		if len(s.LANMembersInAgentPartition()) != numServers+numClients {
			t.Errorf("bad len: %d", len(s.LANMembersInAgentPartition()))
		}
	}

	// Ping each server in the list
	var pingCount int
	for range servers {
		time.Sleep(200 * time.Millisecond)
		m, s := c.router.FindLANRoute()
		ok, err := c.connPool.Ping(s.Datacenter, s.ShortName, s.Addr)
		if !ok {
			t.Errorf("Unable to ping server %v: %s", s.String(), err)
		}
		pingCount++

		// Artificially fail the server in order to rotate the server
		// list
		m.NotifyFailedServer(s)
	}

	if pingCount != numServers {
		t.Errorf("bad len: %d/%d", pingCount, numServers)
	}
}

func TestClient_RPC_TLS(t *testing.T) {
	t.Parallel()
	_, conf1 := testServerConfig(t)
	conf1.TLSConfig.InternalRPC.VerifyIncoming = true
	conf1.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	_, conf2 := testClientConfig(t)
	conf2.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf2)
	c1 := newClient(t, conf2)

	// Try an RPC
	var out struct{}
	if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != structs.ErrNoServers {
		t.Fatalf("err: %v", err)
	}

	// Try to join
	joinLAN(t, c1, s1)

	// Wait for joins to finish/RPC to succeed
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
		if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			r.Fatal("ping failed", err)
		}
	})
}

func newClient(t *testing.T, config *Config) *Client {
	t.Helper()

	client, err := NewClient(config, newDefaultDeps(t, config))
	require.NoError(t, err, "failed to create client")
	t.Cleanup(func() {
		client.Shutdown()
	})
	return client
}

func newTestResolverConfig(t *testing.T, suffix string, dc, agentType string) resolver.Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return resolver.Config{
		Datacenter: dc,
		AgentType:  agentType,
		Authority:  strings.ToLower(s) + "-" + suffix,
	}
}

func newDefaultDeps(t *testing.T, c *Config) Deps {
	t.Helper()

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   c.NodeName,
		Level:  testutil.TestLogLevel,
		Output: testutil.NewLogBuffer(t),
	})

	tls, err := tlsutil.NewConfigurator(c.TLSConfig, logger)
	require.NoError(t, err, "failed to create tls configuration")

	resolverBuilder := resolver.NewServerResolverBuilder(newTestResolverConfig(t, c.NodeName+"-"+c.Datacenter, c.Datacenter, "server"))
	resolver.Register(resolverBuilder)
	t.Cleanup(func() {
		resolver.Deregister(resolverBuilder.Authority())
	})

	balancerBuilder := balancer.NewBuilder(resolverBuilder.Authority(), testutil.Logger(t))
	balancerBuilder.Register()
	t.Cleanup(balancerBuilder.Deregister)

	r := router.NewRouter(
		logger,
		c.Datacenter,
		fmt.Sprintf("%s.%s", c.NodeName, c.Datacenter),
		grpc.NewTracker(resolverBuilder, balancerBuilder),
	)

	connPool := &pool.ConnPool{
		Server:           false,
		SrcAddr:          c.RPCSrcAddr,
		Logger:           logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}),
		MaxTime:          2 * time.Minute,
		MaxStreams:       4,
		TLSConfigurator:  tls,
		Datacenter:       c.Datacenter,
		DefaultQueryTime: c.DefaultQueryTime,
		MaxQueryTime:     c.MaxQueryTime,
		RPCHoldTimeout:   c.RPCHoldTimeout,
	}
	connPool.SetRPCClientTimeout(c.RPCClientTimeout)
	return Deps{
		EventPublisher:  stream.NewEventPublisher(10 * time.Second),
		Logger:          logger,
		TLSConfigurator: tls,
		Tokens:          new(token.Store),
		Router:          r,
		ConnPool:        connPool,
		GRPCConnPool: grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
			Servers:               resolverBuilder,
			TLSWrapper:            grpc.TLSWrapper(tls.OutgoingRPCWrapper()),
			UseTLSForDC:           tls.UseTLS,
			DialingFromServer:     true,
			DialingFromDatacenter: c.Datacenter,
		}),
		LeaderForwarder:          resolverBuilder,
		NewRequestRecorderFunc:   middleware.NewRequestRecorder,
		GetNetRPCInterceptorFunc: middleware.GetNetRPCInterceptor,
		EnterpriseDeps:           newDefaultDepsEnterprise(t, logger, c),
		XDSStreamLimiter:         limiter.NewSessionLimiter(),
	}
}

func TestClient_RPC_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, conf1 := testServerConfig(t)
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	_, conf2 := testClientConfig(t)
	conf2.RPCRateLimit = 2
	conf2.RPCMaxBurst = 2
	c1 := newClient(t, conf2)

	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != structs.ErrRPCRateExceeded {
			r.Fatalf("err: %v", err)
		}
	})
}

func TestClient_SnapshotRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Wait for the leader
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to join.
	joinLAN(t, c1, s1)
	testrpc.WaitForLeader(t, c1.RPC, "dc1")

	// Wait until we've got a healthy server.
	retry.Run(t, func(r *retry.R) {
		if got, want := c1.router.GetLANManager().NumServers(), 1; got != want {
			r.Fatalf("got %d servers want %d", got, want)
		}
	})

	// Take a snapshot.
	var snap bytes.Buffer
	args := structs.SnapshotRequest{
		Datacenter: "dc1",
		Op:         structs.SnapshotSave,
	}
	if err := c1.SnapshotRPC(&args, bytes.NewReader([]byte("")), &snap, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Restore a snapshot.
	args.Op = structs.SnapshotRestore
	if err := c1.SnapshotRPC(&args, &snap, nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClient_SnapshotRPC_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServer(t)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	_, conf1 := testClientConfig(t)
	conf1.RPCRateLimit = 2
	conf1.RPCMaxBurst = 2
	c1 := newClient(t, conf1)

	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := c1.router.GetLANManager().NumServers(), 1; got != want {
			r.Fatalf("got %d servers want %d", got, want)
		}
	})

	retry.Run(t, func(r *retry.R) {
		var snap bytes.Buffer
		args := structs.SnapshotRequest{
			Datacenter: "dc1",
			Op:         structs.SnapshotSave,
		}
		if err := c1.SnapshotRPC(&args, bytes.NewReader([]byte("")), &snap, nil); err != structs.ErrRPCRateExceeded {
			r.Fatalf("err: %v", err)
		}
	})
}

func TestClient_SnapshotRPC_TLS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, conf1 := testServerConfig(t)
	conf1.TLSConfig.InternalRPC.VerifyIncoming = true
	conf1.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	_, conf2 := testClientConfig(t)
	conf2.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf2)
	c1 := newClient(t, conf2)

	// Wait for the leader
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to join.
	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d server members want %d", got, want)
		}
		if got, want := len(c1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d client members want %d", got, want)
		}

		// Wait until we've got a healthy server.
		if got, want := c1.router.GetLANManager().NumServers(), 1; got != want {
			r.Fatalf("got %d servers want %d", got, want)
		}
	})

	// Take a snapshot.
	var snap bytes.Buffer
	args := structs.SnapshotRequest{
		Datacenter: "dc1",
		Op:         structs.SnapshotSave,
	}
	if err := c1.SnapshotRPC(&args, bytes.NewReader([]byte("")), &snap, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Restore a snapshot.
	args.Op = structs.SnapshotRestore
	if err := c1.SnapshotRPC(&args, &snap, nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClientServer_UserEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	clientOut := make(chan serf.UserEvent, 2)
	dir1, c1 := testClientWithConfig(t, func(conf *Config) {
		conf.UserEventHandler = func(e serf.UserEvent) {
			clientOut <- e
		}
	})
	defer os.RemoveAll(dir1)
	defer c1.Shutdown()

	serverOut := make(chan serf.UserEvent, 2)
	dir2, s1 := testServerWithConfig(t, func(conf *Config) {
		conf.UserEventHandler = func(e serf.UserEvent) {
			serverOut <- e
		}
	})
	defer os.RemoveAll(dir2)
	defer s1.Shutdown()

	// Try to join
	joinLAN(t, c1, s1)

	// Wait for the leader
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Check the members
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
	})

	// Fire the user event
	codec := rpcClient(t, s1)
	event := structs.EventFireRequest{
		Name:       "foo",
		Datacenter: "dc1",
		Payload:    []byte("baz"),
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.EventFire", &event, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for all the events
	var clientReceived, serverReceived bool
	for i := 0; i < 2; i++ {
		select {
		case e := <-clientOut:
			switch e.Name {
			case "foo":
				clientReceived = true
			default:
				t.Fatalf("Bad: %#v", e)
			}

		case e := <-serverOut:
			switch e.Name {
			case "foo":
				serverReceived = true
			default:
				t.Fatalf("Bad: %#v", e)
			}

		case <-time.After(10 * time.Second):
			t.Fatalf("timeout")
		}
	}

	if !serverReceived || !clientReceived {
		t.Fatalf("missing events")
	}
}

func TestClient_ReloadConfig(t *testing.T) {
	_, cfg := testClientConfig(t)
	cfg.RPCRateLimit = rate.Limit(500)
	cfg.RPCMaxBurst = 5000
	deps := newDefaultDeps(t, &Config{NodeName: "node1", Datacenter: "dc1"})
	c, err := NewClient(cfg, deps)
	require.NoError(t, err)
	defer c.Shutdown()

	limiter := c.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(500), limiter.Limit())
	require.Equal(t, 5000, limiter.Burst())

	rc := ReloadableConfig{
		RPCRateLimit:         1000,
		RPCMaxBurst:          10000,
		RPCMaxConnsPerClient: 0,
	}
	require.NoError(t, c.ReloadConfig(rc))

	limiter = c.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(1000), limiter.Limit())
	require.Equal(t, 10000, limiter.Burst())
}

func TestClient_ShortReconnectTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	cluster := newTestCluster(t, &testClusterConfig{
		Datacenter: "dc1",
		Servers:    1,
		Clients:    2,
		ServerConf: func(c *Config) {
			c.SerfLANConfig.ReapInterval = 50 * time.Millisecond
		},
		ClientConf: func(c *Config) {
			c.SerfLANConfig.ReapInterval = 50 * time.Millisecond
			c.AdvertiseReconnectTimeout = 100 * time.Millisecond
		},
	})

	// shutdown the client
	cluster.Clients[1].Shutdown()

	// Now wait for it to be reaped. We set the advertised reconnect
	// timeout to 100ms so we are going to check every 50 ms and allow
	// up to 10x the time in the case of slow CI.
	require.Eventually(t,
		func() bool {
			return len(cluster.Servers[0].LANMembersInAgentPartition()) == 2 &&
				len(cluster.Clients[0].LANMembersInAgentPartition()) == 2
		},
		time.Second,
		50*time.Millisecond,
		"The client node was not reaped within the alotted time")
}

type waiter struct {
	duration time.Duration
}

func (w *waiter) Wait(struct{}, *struct{}) error {
	time.Sleep(w.duration)
	return nil
}

func TestClient_RPC_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s1 := testServerWithConfig(t)

	_, c1 := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.RPCClientTimeout = 10 * time.Millisecond
		c.DefaultQueryTime = 150 * time.Millisecond
		c.MaxQueryTime = 200 * time.Millisecond
		c.RPCHoldTimeout = 50 * time.Millisecond
	})
	defer c1.Shutdown()
	joinLAN(t, c1, s1)

	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := c1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
	})

	require.NoError(t, s1.RegisterEndpoint("Long", &waiter{duration: 100 * time.Millisecond}))
	require.NoError(t, s1.RegisterEndpoint("Short", &waiter{duration: 5 * time.Millisecond}))

	t.Run("non-blocking query times out after RPCClientTimeout", func(t *testing.T) {
		// Requests with QueryOptions have a default timeout of
		// RPCClientTimeout (10ms) so we expect the RPC call to timeout.
		var out struct{}
		err := c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{}, &out)
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc error making call: i/o deadline reached")
	})

	t.Run("non-blocking query succeeds", func(t *testing.T) {
		var out struct{}
		require.NoError(t, c1.RPC(context.Background(), "Short.Wait", &structs.NodeSpecificRequest{}, &out))
	})

	t.Run("check that deadline does not persist across calls", func(t *testing.T) {
		var out struct{}
		err := c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{}, &out)
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc error making call: i/o deadline reached")
		require.NoError(t, c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryIndex: 1,
			},
		}, &out))
	})

	t.Run("blocking query succeeds", func(t *testing.T) {
		var out struct{}
		require.NoError(t, c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryIndex: 1,
			},
		}, &out))
	})

	t.Run("blocking query with MaxQueryTime succeeds", func(t *testing.T) {
		var out struct{}
		// Although we set MaxQueryTime to 100ms, the client is adding maximum
		// jitter (100ms / 16 = 6.25ms) as well as RPCHoldTimeout (50ms).
		// Client waits 156.25ms while the server waits 106.25ms (artifically
		// adds maximum jitter) so the server will always return first.
		require.NoError(t, c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryIndex: 1,
				MaxQueryTime:  100 * time.Millisecond,
			},
		}, &out))
	})

	// This following scenario should not occur in practice since the server
	// should be aware of RPC timeouts and always return blocking queries before
	// the client closes the connection. But this is just a hypothetical case
	// to show waiter can fail since it does not consider QueryOptions.
	t.Run("blocking query with low MaxQueryTime fails", func(t *testing.T) {
		var out struct{}
		// Although we set MaxQueryTime to 20ms, the client is adding maximum
		// jitter (20ms / 16 = 1.25ms) as well as RPCHoldTimeout (50ms).
		// Client waits 71.25ms while the server waits 106.25ms (artifically
		// adds maximum jitter) so the client will error first.
		err := c1.RPC(context.Background(), "Long.Wait", &structs.NodeSpecificRequest{
			QueryOptions: structs.QueryOptions{
				MinQueryIndex: 1,
				MaxQueryTime:  20 * time.Millisecond,
			},
		}, &out)
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc error making call: i/o deadline reached")
	})
}
