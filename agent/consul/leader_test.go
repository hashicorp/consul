package consul

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestLeader_RegisterMember(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	joinLAN(t, c1, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Client should be registered
	state := s1.fsm.State()
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Should have a check
	_, checks, err := state.NodeChecks(nil, c1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("client missing check")
	}
	if checks[0].CheckID != structs.SerfCheckID {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Name != structs.SerfCheckName {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Status != api.HealthPassing {
		t.Fatalf("bad check: %v", checks[0])
	}

	// Server should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(s1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatalf("server not registered")
		}
	})

	// Service should be registered
	_, services, err := state.NodeServices(nil, s1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services.Services["consul"]; !ok {
		t.Fatalf("consul service not registered: %v", services)
	}
}

func TestLeader_FailedMember(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to join
	joinLAN(t, c1, s1)

	// Fail the member
	c1.Shutdown()

	// Should be registered
	state := s1.fsm.State()
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Should have a check
	_, checks, err := state.NodeChecks(nil, c1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("client missing check")
	}
	if checks[0].CheckID != structs.SerfCheckID {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Name != structs.SerfCheckName {
		t.Fatalf("bad check: %v", checks[0])
	}

	retry.Run(t, func(r *retry.R) {
		_, checks, err = state.NodeChecks(nil, c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if got, want := checks[0].Status, api.HealthCritical; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestLeader_LeftMember(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	joinLAN(t, c1, s1)

	state := s1.fsm.State()

	// Should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Node should leave
	c1.Leave()
	c1.Shutdown()

	// Should be deregistered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node != nil {
			r.Fatal("client still registered")
		}
	})
}
func TestLeader_ReapMember(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	joinLAN(t, c1, s1)

	state := s1.fsm.State()

	// Should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Simulate a node reaping
	mems := s1.LANMembers()
	var c1mem serf.Member
	for _, m := range mems {
		if m.Name == c1.config.NodeName {
			c1mem = m
			c1mem.Status = StatusReap
			break
		}
	}
	s1.reconcileCh <- c1mem

	// Should be deregistered; we have to poll quickly here because
	// anti-entropy will put it back.
	reaped := false
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if node == nil {
			reaped = true
			break
		}
	}
	if !reaped {
		t.Fatalf("client should not be registered")
	}
}

func TestLeader_ReapServer(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "allow"
		c.ACLEnforceVersion8 = true
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "allow"
		c.ACLEnforceVersion8 = true
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "allow"
		c.ACLEnforceVersion8 = true
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Try to join
	joinLAN(t, s1, s2)
	joinLAN(t, s1, s3)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")
	testrpc.WaitForLeader(t, s3.RPC, "dc1")
	state := s1.fsm.State()

	// s3 should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(s3.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// call reconcileReaped with a map that does not contain s3
	knownMembers := make(map[string]struct{})
	knownMembers[s1.config.NodeName] = struct{}{}
	knownMembers[s2.config.NodeName] = struct{}{}

	err := s1.reconcileReaped(knownMembers)

	if err != nil {
		t.Fatalf("Unexpected error :%v", err)
	}
	// s3 should be deregistered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(s3.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node != nil {
			r.Fatalf("server with id %v should not be registered", s3.config.NodeID)
		}
	})

}

func TestLeader_Reconcile_ReapMember(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register a non-existing member
	dead := structs.RegisterRequest{
		Datacenter: s1.config.Datacenter,
		Node:       "no-longer-around",
		Address:    "127.1.1.1",
		Check: &structs.HealthCheck{
			Node:    "no-longer-around",
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthCritical,
		},
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	var out struct{}
	if err := s1.RPC("Catalog.Register", &dead, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force a reconciliation
	if err := s1.reconcile(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Node should be gone
	state := s1.fsm.State()
	_, node, err := state.GetNode("no-longer-around")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("client registered")
	}
}

func TestLeader_Reconcile(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Join before we have a leader, this should cause a reconcile!
	joinLAN(t, c1, s1)

	// Should not be registered
	state := s1.fsm.State()
	_, node, err := state.GetNode(c1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("client registered")
	}

	// Should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})
}

func TestLeader_Reconcile_Races(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	joinLAN(t, c1, s1)

	// Wait for the server to reconcile the client and register it.
	state := s1.fsm.State()
	var nodeAddr string
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
		nodeAddr = node.Address
	})

	// Add in some metadata via the catalog (as if the agent synced it
	// there). We also set the serfHealth check to failing so the reconcile
	// will attempt to flip it back
	req := structs.RegisterRequest{
		Datacenter: s1.config.Datacenter,
		Node:       c1.config.NodeName,
		ID:         c1.config.NodeID,
		Address:    nodeAddr,
		NodeMeta:   map[string]string{"hello": "world"},
		Check: &structs.HealthCheck{
			Node:    c1.config.NodeName,
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthCritical,
			Output:  "",
		},
	}
	var out struct{}
	if err := s1.RPC("Catalog.Register", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force a reconcile and make sure the metadata stuck around.
	if err := s1.reconcile(); err != nil {
		t.Fatalf("err: %v", err)
	}
	_, node, err := state.GetNode(c1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node == nil {
		t.Fatalf("bad")
	}
	if hello, ok := node.Meta["hello"]; !ok || hello != "world" {
		t.Fatalf("bad")
	}

	// Fail the member and wait for the health to go critical.
	c1.Shutdown()
	retry.Run(t, func(r *retry.R) {
		_, checks, err := state.NodeChecks(nil, c1.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if got, want := checks[0].Status, api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})

	// Make sure the metadata didn't get clobbered.
	_, node, err = state.GetNode(c1.config.NodeName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node == nil {
		t.Fatalf("bad")
	}
	if hello, ok := node.Meta["hello"]; !ok || hello != "world" {
		t.Fatalf("bad")
	}
}

func TestLeader_LeftServer(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Put s1 last so we don't trigger a leader election.
	servers := []*Server{s2, s3, s1}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Kill any server
	servers[0].Shutdown()

	// Force remove the non-leader (transition to left state)
	if err := servers[1].RemoveFailedNode(servers[0].config.NodeName); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait until the remaining servers show only 2 peers.
	for _, s := range servers[1:] {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 2)) })
	}
	s1.Shutdown()
}

func TestLeader_LeftLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
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
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Kill the leader!
	var leader *Server
	for _, s := range servers {
		if s.IsLeader() {
			leader = s
			break
		}
	}
	if leader == nil {
		t.Fatalf("Should have a leader")
	}
	if !leader.isReadyForConsistentReads() {
		t.Fatalf("Expected leader to be ready for consistent reads ")
	}
	leader.Leave()
	if leader.isReadyForConsistentReads() {
		t.Fatalf("Expected consistent read state to be false ")
	}
	leader.Shutdown()
	time.Sleep(100 * time.Millisecond)

	var remain *Server
	for _, s := range servers {
		if s == leader {
			continue
		}
		remain = s
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 2)) })
	}

	// Verify the old leader is deregistered
	state := remain.fsm.State()
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(leader.config.NodeName)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node != nil {
			r.Fatal("leader should be deregistered")
		}
	})
}

func TestLeader_MultiBootstrap(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	servers := []*Server{s1, s2}

	// Try to join
	joinLAN(t, s2, s1)

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) {
			if got, want := len(s.serfLAN.Members()), 2; got != want {
				r.Fatalf("got %d peers want %d", got, want)
			}
		})
	}

	// Ensure we don't have multiple raft peers
	for _, s := range servers {
		peers, _ := s.numPeers()
		if peers != 1 {
			t.Fatalf("should only have 1 raft peer!")
		}
	}
}

func TestLeader_TombstoneGC_Reset(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
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
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	var leader *Server
	for _, s := range servers {
		if s.IsLeader() {
			leader = s
			break
		}
	}
	if leader == nil {
		t.Fatalf("Should have a leader")
	}

	// Check that the leader has a pending GC expiration
	if !leader.tombstoneGC.PendingExpiration() {
		t.Fatalf("should have pending expiration")
	}

	// Kill the leader
	leader.Shutdown()
	time.Sleep(100 * time.Millisecond)

	// Wait for a new leader
	leader = nil
	retry.Run(t, func(r *retry.R) {
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
				return
			}
		}
		r.Fatal("no leader")
	})

	retry.Run(t, func(r *retry.R) {
		if !leader.tombstoneGC.PendingExpiration() {
			r.Fatal("leader has no pending GC expiration")
		}
	})
}

func TestLeader_ReapTombstones(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.TombstoneTTL = 50 * time.Millisecond
		c.TombstoneTTLGranularity = 10 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create a KV entry
	arg := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   "test",
			Value: []byte("test"),
		},
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	var out bool
	if err := msgpackrpc.CallWithCodec(codec, "KVS.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the KV entry (tombstoned).
	arg.Op = api.KVDelete
	if err := msgpackrpc.CallWithCodec(codec, "KVS.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure there's a tombstone.
	state := s1.fsm.State()
	retry.Run(t, func(r *retry.R) {
		snap := state.Snapshot()
		defer snap.Close()
		stones, err := snap.Tombstones()
		if err != nil {
			r.Fatalf("err: %s", err)
		}
		if stones.Next() == nil {
			r.Fatalf("missing tombstones")
		}
		if stones.Next() != nil {
			r.Fatalf("unexpected extra tombstones")
		}
	})

	// Check that the new leader has a pending GC expiration by
	// watching for the tombstone to get removed.
	retry.Run(t, func(r *retry.R) {
		snap := state.Snapshot()
		defer snap.Close()
		stones, err := snap.Tombstones()
		if err != nil {
			r.Fatal(err)
		}
		if stones.Next() != nil {
			r.Fatal("should have no tombstones")
		}
	})
}

func TestLeader_RollRaftServer(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = true
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 2
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 1
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 2
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	servers := []*Server{s1, s2, s3}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Kill the v1 server
	s2.Shutdown()

	for _, s := range []*Server{s1, s3} {
		retry.Run(t, func(r *retry.R) {
			minVer, err := s.autopilot.MinRaftProtocol()
			if err != nil {
				r.Fatal(err)
			}
			if got, want := minVer, 2; got != want {
				r.Fatalf("got min raft version %d want %d", got, want)
			}
		})
	}

	// Replace the dead server with one running raft protocol v3
	dir4, s4 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()
	joinLAN(t, s4, s1)
	servers[1] = s4

	// Make sure the dead server is removed and we're back to 3 total peers
	for _, s := range servers {
		retry.Run(t, func(r *retry.R) {
			addrs := 0
			ids := 0
			future := s.raft.GetConfiguration()
			if err := future.Error(); err != nil {
				r.Fatal(err)
			}
			for _, server := range future.Configuration().Servers {
				if string(server.ID) == string(server.Address) {
					addrs++
				} else {
					ids++
				}
			}
			if got, want := addrs, 2; got != want {
				r.Fatalf("got %d server addresses want %d", got, want)
			}
			if got, want := ids, 1; got != want {
				r.Fatalf("got %d server ids want %d", got, want)
			}
		})
	}
}

func TestLeader_ChangeServerID(t *testing.T) {
	conf := func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 3
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

	servers := []*Server{s1, s2, s3}

	// Try to join and wait for all servers to get promoted
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	for _, s := range servers {
		testrpc.WaitForTestAgent(t, s.RPC, "dc1")
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Shut down a server, freeing up its address/port
	s3.Shutdown()

	retry.Run(t, func(r *retry.R) {
		alive := 0
		for _, m := range s1.LANMembers() {
			if m.Status == serf.StatusAlive {
				alive++
			}
		}
		if got, want := alive, 2; got != want {
			r.Fatalf("got %d alive members want %d", got, want)
		}
	})

	// Bring up a new server with s3's address that will get a different ID
	dir4, s4 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.Datacenter = "dc1"
		c.RaftConfig.ProtocolVersion = 3
		c.SerfLANConfig.MemberlistConfig = s3.config.SerfLANConfig.MemberlistConfig
		c.RPCAddr = s3.config.RPCAddr
		c.RPCAdvertise = s3.config.RPCAdvertise
	})
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	joinLAN(t, s4, s1)
	testrpc.WaitForLeader(t, s4.RPC, "dc1")
	servers[2] = s4

	// While integrating #3327 it uncovered that this test was flaky. The
	// connection pool would use the same TCP connection to the old server
	// which would give EOF errors to the autopilot health check RPC call.
	// To make this more reliable we changed the connection pool to throw
	// away the connection if it sees an EOF error, since there's no way
	// that connection is going to work again. This made this test reliable
	// since it will make a new connection to s4.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			// Make sure the dead server is removed and we're back below 4
			r.Check(wantPeers(s, 3))
		}
	})
}

func TestLeader_ChangeNodeID(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	servers := []*Server{s1, s2, s3}

	// Try to join and wait for all servers to get promoted
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	for _, s := range servers {
		testrpc.WaitForTestAgent(t, s.RPC, "dc1")
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Shut down a server, freeing up its address/port
	s3.Shutdown()

	retry.Run(t, func(r *retry.R) {
		failed := 0
		for _, m := range s1.LANMembers() {
			if m.Status == serf.StatusFailed {
				failed++
			}
		}
		require.Equal(r, 1, failed)
	})

	// Bring up a new server with s3's name that will get a different ID
	dir4, s4 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.Datacenter = "dc1"
		c.NodeName = s3.config.NodeName
	})
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()
	joinLAN(t, s4, s1)
	servers[2] = s4

	// Make sure the dead server is gone from both Raft and Serf and we're back to 3 total peers
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	retry.Run(t, func(r *retry.R) {
		for _, m := range s1.LANMembers() {
			require.Equal(r, serf.StatusAlive, m.Status)
		}
	})
}

func TestLeader_ACL_Initialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		build     string
		master    string
		bootstrap bool
	}{
		{"old version, no master", "0.8.0", "", true},
		{"old version, master", "0.8.0", "root", false},
		{"new version, no master", "0.9.1", "", true},
		{"new version, master", "0.9.1", "root", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := func(c *Config) {
				c.Build = tt.build
				c.Bootstrap = true
				c.Datacenter = "dc1"
				c.ACLDatacenter = "dc1"
				c.ACLsEnabled = true
				c.ACLMasterToken = tt.master
			}
			dir1, s1 := testServerWithConfig(t, conf)
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()
			testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

			if tt.master != "" {
				_, master, err := s1.fsm.State().ACLTokenGetBySecret(nil, tt.master)
				require.NoError(t, err)
				require.NotNil(t, master)
			}

			_, anon, err := s1.fsm.State().ACLTokenGetBySecret(nil, anonymousToken)
			require.NoError(t, err)
			require.NotNil(t, anon)

			canBootstrap, _, err := s1.fsm.State().CanBootstrapACLToken()
			require.NoError(t, err)
			require.Equal(t, tt.bootstrap, canBootstrap)

			_, policy, err := s1.fsm.State().ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID)
			require.NoError(t, err)
			require.NotNil(t, policy)
		})
	}
}

func TestLeader_ACLUpgrade(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// create a legacy management ACL
	mgmt := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "Management token",
			Type: structs.ACLTokenTypeManagement,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var mgmt_id string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.Apply", &mgmt, &mgmt_id))

	// wait for it to be upgraded
	retry.Run(t, func(t *retry.R) {
		_, token, err := s1.fsm.State().ACLTokenGetBySecret(nil, mgmt_id)
		require.NoError(t, err)
		require.NotNil(t, token)
		require.NotEqual(t, "", token.AccessorID)
		require.Equal(t, structs.ACLTokenTypeManagement, token.Type)
		require.Len(t, token.Policies, 1)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, token.Policies[0].ID)
	})

	// create a legacy management ACL
	client := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "Management token",
			Type:  structs.ACLTokenTypeClient,
			Rules: `node "" { policy = "read"}`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var client_id string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.Apply", &client, &client_id))

	// wait for it to be upgraded
	retry.Run(t, func(t *retry.R) {
		_, token, err := s1.fsm.State().ACLTokenGetBySecret(nil, client_id)
		require.NoError(t, err)
		require.NotNil(t, token)
		require.NotEqual(t, "", token.AccessorID)
		require.Len(t, token.Policies, 0)
		require.Equal(t, structs.ACLTokenTypeClient, token.Type)
		require.Equal(t, client.ACL.Rules, token.Rules)
	})
}

func TestLeader_ConfigEntryBootstrap(t *testing.T) {
	t.Parallel()
	global_entry_init := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"foo": "bar",
			"bar": int64(1),
		},
	}

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.5.0"
		c.ConfigEntryBootstrap = []structs.ConfigEntry{
			global_entry_init,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	retry.Run(t, func(t *retry.R) {
		_, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ProxyDefaults, structs.ProxyConfigGlobal)
		require.NoError(t, err)
		require.NotNil(t, entry)
		global, ok := entry.(*structs.ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, global_entry_init.Kind, global.Kind)
		require.Equal(t, global_entry_init.Name, global.Name)
		require.Equal(t, global_entry_init.Config, global.Config)
	})
}

func TestLeader_ConfigEntryBootstrap_Fail(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	defer pw.Close()

	ch := make(chan string, 1)
	go func() {
		defer pr.Close()
		scan := bufio.NewScanner(pr)
		for scan.Scan() {
			line := scan.Text()

			if strings.Contains(line, "consul: failed to establish leadership") {
				ch <- ""
				return
			}
			if strings.Contains(line, "connect: initialized primary datacenter") {
				ch <- "leadership should not have gotten here if config entries properly failed"
				return
			}
		}

		if scan.Err() != nil {
			ch <- fmt.Sprintf("ERROR: %v", scan.Err())
		} else {
			ch <- "should not get here"
		}
	}()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.LogOutput = io.MultiWriter(pw, testutil.TestWriter(t))
		c.Build = "1.6.0"
		c.ConfigEntryBootstrap = []structs.ConfigEntry{
			&structs.ServiceSplitterConfigEntry{
				Kind: structs.ServiceSplitter,
				Name: "web",
				Splits: []structs.ServiceSplit{
					{Weight: 100, Service: "web"},
				},
			},
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	result := <-ch
	require.Empty(t, result)
}
