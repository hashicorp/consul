package consul

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

func TestAutopilot_CleanupDeadServer(t *testing.T) {
	dir1, s1 := testServerDCBootstrap(t, "dc1", true)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	servers := []*Server{s1, s2, s3}

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Kill a non-leader server
	s2.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		alive := 0
		for _, m := range s1.LANMembers() {
			if m.Status == serf.StatusAlive {
				alive++
			}
		}
		return alive == 2, nil
	}, func(err error) {
		t.Fatalf("should have 2 alive members")
	})

	// Bring up and join a new server
	dir4, s4 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	if _, err := s4.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	servers[1] = s4

	// Make sure the dead server is removed and we're back to 3 total peers
	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}
}

func TestAutopilot_CleanupDeadServerPeriodic(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.AutopilotInterval = 100 * time.Millisecond
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

	servers := []*Server{s1, s2, s3, s4}

	// Join the servers to s1
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)

	for _, s := range servers[1:] {
		if _, err := s.JoinLAN([]string{addr}); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 4, nil
		}, func(err error) {
			t.Fatalf("should have 4 peers")
		})
	}

	// Kill a non-leader server
	s4.Shutdown()

	// Should be removed from the peers automatically
	for _, s := range []*Server{s1, s2, s3} {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}
}

func TestAutopilot_PromoteNonVoter(t *testing.T) {
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

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Wait for the new server to be added as a non-voter, but make sure
	// it doesn't get promoted to a voter even after ServerStabilizationTime,
	// because that would result in an even-numbered quorum count.
	testutil.WaitForResult(func() (bool, error) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return false, err
		}

		servers := future.Configuration().Servers

		if len(servers) != 2 {
			return false, fmt.Errorf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Nonvoter {
			return false, fmt.Errorf("bad: %v", servers)
		}
		health := s1.getServerHealth(string(servers[1].ID))
		if health == nil {
			return false, fmt.Errorf("nil health")
		}
		if !health.Healthy {
			return false, fmt.Errorf("bad: %v", health)
		}
		if time.Now().Sub(health.StableSince) < s1.config.AutopilotConfig.ServerStabilizationTime {
			return false, fmt.Errorf("stable period not elapsed")
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	// Now add another server and make sure they both get promoted to voters after stabilization
	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return false, err
		}

		servers := future.Configuration().Servers

		if len(servers) != 3 {
			return false, fmt.Errorf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Voter {
			return false, fmt.Errorf("bad: %v", servers)
		}
		if servers[2].Suffrage != raft.Voter {
			return false, fmt.Errorf("bad: %v", servers)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}
