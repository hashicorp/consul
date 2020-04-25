package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestAutopilot_IdempotentShutdown(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, nil)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	retry.Run(t, func(r *retry.R) { r.Check(waitForLeader(s1)) })

	s1.autopilot.Start()
	s1.autopilot.Start()
	s1.autopilot.Start()
	s1.autopilot.Stop()
	s1.autopilot.Stop()
	s1.autopilot.Stop()
}

func TestAutopilot_CleanupDeadServer(t *testing.T) {
	t.Parallel()
	for i := 1; i <= 3; i++ {
		testCleanupDeadServer(t, i)
	}
}

func testCleanupDeadServer(t *testing.T, raftVersion int) {
	dc := "dc1"
	conf := func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = 5
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(raftVersion)
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	dir5, s5 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir5)
	defer s5.Shutdown()

	servers := []*Server{s1, s2, s3, s4, s5}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	joinLAN(t, s4, s1)
	joinLAN(t, s5, s1)

	for _, s := range servers {
		testrpc.WaitForLeader(t, s.RPC, dc)
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 5)) })
	}

	require := require.New(t)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	leaderIndex := -1
	for i, s := range servers {
		if s.IsLeader() {
			leaderIndex = i
			break
		}
	}
	require.NotEqual(leaderIndex, -1)

	// Shutdown two non-leader servers
	killed := make(map[string]struct{})
	for i, s := range servers {
		if i != leaderIndex {
			s.Shutdown()
			killed[string(s.config.NodeID)] = struct{}{}
		}
		if len(killed) == 2 {
			break
		}
	}

	retry.Run(t, func(r *retry.R) {
		alive := 0
		for _, m := range servers[leaderIndex].LANMembers() {
			if m.Status == serf.StatusAlive {
				alive++
			}
		}
		if alive != 3 {
			r.Fatalf("Expected three alive servers instead of %d", alive)
		}
	})

	// Make sure the dead servers are removed and we're back to 3 total peers
	for _, s := range servers {
		_, killed := killed[string(s.config.NodeID)]
		if !killed {
			retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
		}
	}
}

func TestAutopilot_CleanupDeadNonvoter(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Have s2 join and then shut it down immediately before it gets a chance to
	// be promoted to a voter.
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})
	s2.Shutdown()

	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1}))
	})
}

func TestAutopilot_CleanupDeadServerPeriodic(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
	}

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	dir5, s5 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir5)
	defer s5.Shutdown()

	servers := []*Server{s1, s2, s3, s4, s5}

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 5))
		}
	})

	// Kill a non-leader server
	s4.Shutdown()

	// Should be removed from the peers automatically
	servers = []*Server{s1, s2, s3, s5}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 4))
		}
	})
}

func TestAutopilot_RollingUpdate(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
	}

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	servers := []*Server{s1, s2, s3}
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Add one more server like we are doing a rolling update.
	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()
	joinLAN(t, s1, s4)
	servers = append(servers, s4)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Now kill one of the "old" nodes like we are doing a rolling update.
	s3.Shutdown()

	isVoter := func() bool {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		for _, s := range future.Configuration().Servers {
			if string(s.ID) == string(s4.config.NodeID) {
				return s.Suffrage == raft.Voter
			}
		}
		t.Fatalf("didn't find s4")
		return false
	}

	// Wait for s4 to stabilize, get promoted to a voter, and for s3 to be
	// removed.
	servers = []*Server{s1, s2, s4}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
		if !isVoter() {
			r.Fatalf("should be a voter")
		}
	})
}

func TestAutopilot_CleanupStaleRaftServer(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerDCBootstrap(t, "dc1", true)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	servers := []*Server{s1, s2, s3}

	// Join the servers to s1
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add s4 to peers directly
	addVoterFuture := s1.raft.AddVoter(raft.ServerID(s4.config.NodeID), raft.ServerAddress(joinAddrLAN(s4)), 0, 0)
	if err := addVoterFuture.Error(); err != nil {
		t.Fatal(err)
	}

	// Verify we have 4 peers
	peers, err := s1.numPeers()
	if err != nil {
		t.Fatal(err)
	}
	if peers != 4 {
		t.Fatalf("bad: %v", peers)
	}

	// Wait for s4 to be removed
	for _, s := range []*Server{s1, s2, s3} {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}
}

func TestAutopilot_PromoteNonVoter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.AutopilotConfig.ServerStabilizationTime = 200 * time.Millisecond
		c.ServerHealthInterval = 100 * time.Millisecond
		c.AutopilotInterval = 100 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	joinLAN(t, s2, s1)

	// Make sure we see it as a nonvoter initially. We wait until half
	// the stabilization period has passed.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Nonvoter {
			r.Fatalf("bad: %v", servers)
		}
		health := s1.autopilot.GetServerHealth(string(servers[1].ID))
		if health == nil {
			r.Fatal("nil health")
		}
		if !health.Healthy {
			r.Fatalf("bad: %v", health)
		}
		if time.Since(health.StableSince) < s1.config.AutopilotConfig.ServerStabilizationTime/2 {
			r.Fatal("stable period not elapsed")
		}
	})

	// Make sure it ends up as a voter.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Voter {
			r.Fatalf("bad: %v", servers)
		}
	})
}

func TestAutopilot_MinQuorum(t *testing.T) {
	dc := "dc1"
	closeMap := make(map[string]chan struct{})
	conf := func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = 4
		c.AutopilotConfig.MinQuorum = 3
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(2)
		c.AutopilotInterval = 100 * time.Millisecond
		//Let us know when a server is actually gone
		ch := make(chan struct{})
		c.NotifyShutdown = func() {
			t.Logf("%v is shutdown", c.NodeName)
			close(ch)
		}
		closeMap[c.NodeName] = ch
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	servers := map[string]*Server{s1.config.NodeName: s1,
		s2.config.NodeName: s2,
		s3.config.NodeName: s3,
		s4.config.NodeName: s4}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	joinLAN(t, s4, s1)

	//Differentiate between leader and server
	findStatus := func(leader bool) *Server {
		for _, mem := range servers {
			if mem.IsLeader() == leader {
				return mem
			}
			if !mem.IsLeader() == !leader {
				return mem
			}
		}

		return nil
	}
	testrpc.WaitForLeader(t, s1.RPC, dc)

	// Have autopilot take one into left
	dead := findStatus(false)
	if dead == nil {
		t.Fatalf("no members set")
	}
	dead.Shutdown()
	<-closeMap[dead.config.NodeName]
	retry.Run(t, func(r *retry.R) {
		leader := findStatus(true)
		if leader == nil {
			r.Fatalf("no members set")
		}
		for _, m := range leader.LANMembers() {
			if m.Name == dead.config.NodeName && m.Status != serf.StatusLeft {
				r.Fatalf("%v should be left, got %v", m.Name, m.Status.String())
			}
		}
	})

	delete(servers, dead.config.NodeName)
	//Autopilot should not take this one into left
	dead = findStatus(false)
	if err := dead.Shutdown(); err != nil {
		t.Fatalf("could not shut down %s, error %v", dead.config.NodeName, err)
	}
	<-closeMap[dead.config.NodeName]

	retry.Run(t, func(r *retry.R) {
		leader := findStatus(true)
		if leader == nil {
			r.Fatalf("no members set")
		}
		for _, m := range leader.LANMembers() {
			if m.Name == dead.config.NodeName && m.Status != serf.StatusFailed {
				r.Fatalf("%v should be failed, got %v", m.Name, m.Status.String())
			}
		}
	})

}
