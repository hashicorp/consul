package consul

import (
	"bytes"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func testClientConfig(t *testing.T) (string, *Config) {
	dir := testutil.TempDir(t, "consul")
	config := DefaultConfig()

	ports := freeport.MustTake(2)

	returnPortsFn := func() {
		// The method of plumbing this into the client shutdown hook doesn't
		// cover all exit points, so we insulate this against multiple
		// invocations and then it's safe to call it a bunch of times.
		freeport.Return(ports)
		config.NotifyShutdown = nil // self-erasing
	}
	config.NotifyShutdown = returnPortsFn

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

func testClientWithConfig(t *testing.T, cb func(c *Config)) (string, *Client) {
	dir, config := testClientConfig(t)
	if cb != nil {
		cb(config)
	}
	client, err := NewClient(config)
	if err != nil {
		config.NotifyShutdown()
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
		if got, want := c1.routers.NumServers(), 1; got != want {
			r.Fatalf("got %d servers want %d", got, want)
		}
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembers()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
	})
}

func TestClient_LANReap(t *testing.T) {
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
		require.Len(r, s1.LANMembers(), 2)
		require.Len(r, c1.LANMembers(), 2)
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		server := c1.routers.FindServer()
		require.NotNil(t, server)
		require.Equal(t, s1.config.NodeName, server.Name)
	})

	// shutdown the second dc
	s1.Shutdown()

	retry.Run(t, func(r *retry.R) {
		require.Len(r, c1.LANMembers(), 1)
		server := c1.routers.FindServer()
		require.Nil(t, server)
	})
}

func TestClient_JoinLAN_Invalid(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClientDC(t, "other")
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	if _, err := c1.JoinLAN([]string{joinAddrLAN(s1)}); err == nil {
		t.Fatal("should error")
	}

	time.Sleep(50 * time.Millisecond)
	if len(s1.LANMembers()) != 1 {
		t.Fatalf("should not join")
	}
	if len(c1.LANMembers()) != 1 {
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
	if _, err := c1.JoinLAN([]string{joinAddrWAN(s1)}); err == nil {
		t.Fatal("should error")
	}

	time.Sleep(50 * time.Millisecond)
	if len(s1.WANMembers()) != 1 {
		t.Fatalf("should not join")
	}
	if len(c1.LANMembers()) != 1 {
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
	if err := c1.RPC("Status.Ping", struct{}{}, &out); err != structs.ErrNoServers {
		t.Fatalf("err: %v", err)
	}

	// Try to join
	joinLAN(t, c1, s1)

	// Check the members
	if len(s1.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	if len(c1.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	// RPC should succeed
	retry.Run(t, func(r *retry.R) {
		if err := c1.RPC("Status.Ping", struct{}{}, &out); err != nil {
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
		if err := c1.RPC("Status.Ping", struct{}{}, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
	})

	failer := &leaderFailer{}
	if err := s1.RegisterEndpoint("Fail", failer); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out struct{}
	if err := c1.RPC("Fail.Always", struct{}{}, &out); !structs.IsErrNoLeader(err) {
		t.Fatalf("err: %v", err)
	}
	if got, want := failer.totalCalls, 2; got < want {
		t.Fatalf("got %d want >= %d", got, want)
	}
	if err := c1.RPC("Fail.Once", struct{}{}, &out); err != nil {
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
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembers()), 2; got != want {
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
				if err := c1.RPC("Status.Ping", struct{}{}, &out); err != nil {
					r.Fatal("ping failed", err)
				}
			})
		}()
	}

	wg.Wait()
}

func TestClient_RPC_ConsulServerPing(t *testing.T) {
	t.Parallel()
	var servers []*Server
	var serverDirs []string
	const numServers = 5

	for n := 0; n < numServers; n++ {
		bootstrap := n == 0
		dir, s := testServerDCBootstrap(t, "dc1", bootstrap)
		defer os.RemoveAll(dir)
		defer s.Shutdown()

		servers = append(servers, s)
		serverDirs = append(serverDirs, dir)
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
	c.routers.ResetRebalanceTimer()
	time.Sleep(time.Second)

	if len(c.LANMembers()) != numServers+numClients {
		t.Errorf("bad len: %d", len(c.LANMembers()))
	}
	for _, s := range servers {
		if len(s.LANMembers()) != numServers+numClients {
			t.Errorf("bad len: %d", len(s.LANMembers()))
		}
	}

	// Ping each server in the list
	var pingCount int
	for range servers {
		time.Sleep(200 * time.Millisecond)
		s := c.routers.FindServer()
		ok, err := c.connPool.Ping(s.Datacenter, s.ShortName, s.Addr, s.Version, s.UseTLS)
		if !ok {
			t.Errorf("Unable to ping server %v: %s", s.String(), err)
		}
		pingCount++

		// Artificially fail the server in order to rotate the server
		// list
		c.routers.NotifyFailedServer(s)
	}

	if pingCount != numServers {
		t.Errorf("bad len: %d/%d", pingCount, numServers)
	}
}

func TestClient_RPC_TLS(t *testing.T) {
	t.Parallel()
	dir1, conf1 := testServerConfig(t)
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := NewServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, conf2 := testClientConfig(t)
	defer conf2.NotifyShutdown()
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	c1, err := NewClient(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try an RPC
	var out struct{}
	if err := c1.RPC("Status.Ping", struct{}{}, &out); err != structs.ErrNoServers {
		t.Fatalf("err: %v", err)
	}

	// Try to join
	joinLAN(t, c1, s1)

	// Wait for joins to finish/RPC to succeed
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembers()), 2; got != want {
			r.Fatalf("got %d client LAN members want %d", got, want)
		}
		if err := c1.RPC("Status.Ping", struct{}{}, &out); err != nil {
			r.Fatal("ping failed", err)
		}
	})
}

func TestClient_RPC_RateLimit(t *testing.T) {
	t.Parallel()
	dir1, conf1 := testServerConfig(t)
	s1, err := NewServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, conf2 := testClientConfig(t)
	defer conf2.NotifyShutdown()
	conf2.RPCRate = 2
	conf2.RPCMaxBurst = 2
	c1, err := NewClient(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := c1.RPC("Status.Ping", struct{}{}, &out); err != structs.ErrRPCRateExceeded {
			r.Fatalf("err: %v", err)
		}
	})
}

func TestClient_SnapshotRPC(t *testing.T) {
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
		if got, want := c1.routers.NumServers(), 1; got != want {
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
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, conf1 := testClientConfig(t)
	defer conf1.NotifyShutdown()
	conf1.RPCRate = 2
	conf1.RPCMaxBurst = 2
	c1, err := NewClient(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := c1.routers.NumServers(), 1; got != want {
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
	t.Parallel()
	dir1, conf1 := testServerConfig(t)
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := NewServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, conf2 := testClientConfig(t)
	defer conf2.NotifyShutdown()
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	c1, err := NewClient(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Wait for the leader
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to join.
	joinLAN(t, c1, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d server members want %d", got, want)
		}
		if got, want := len(c1.LANMembers()), 2; got != want {
			r.Fatalf("got %d client members want %d", got, want)
		}

		// Wait until we've got a healthy server.
		if got, want := c1.routers.NumServers(), 1; got != want {
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
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d server LAN members want %d", got, want)
		}
		if got, want := len(c1.LANMembers()), 2; got != want {
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

func TestClient_Encrypted(t *testing.T) {
	t.Parallel()
	dir1, c1 := testClient(t)
	defer os.RemoveAll(dir1)
	defer c1.Shutdown()

	key := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	dir2, c2 := testClientWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = key
	})
	defer os.RemoveAll(dir2)
	defer c2.Shutdown()

	if c1.Encrypted() {
		t.Fatalf("should not be encrypted")
	}
	if !c2.Encrypted() {
		t.Fatalf("should be encrypted")
	}
}

func TestClient_Reload(t *testing.T) {
	t.Parallel()
	dir1, c := testClientWithConfig(t, func(c *Config) {
		c.RPCRate = 500
		c.RPCMaxBurst = 5000
	})
	defer os.RemoveAll(dir1)
	defer c.Shutdown()

	limiter := c.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(500), limiter.Limit())
	require.Equal(t, 5000, limiter.Burst())

	c.config.RPCRate = 1000
	c.config.RPCMaxBurst = 10000

	require.NoError(t, c.ReloadConfig(c.config))
	limiter = c.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(1000), limiter.Limit())
	require.Equal(t, 10000, limiter.Burst())
}
