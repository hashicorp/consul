// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestLeader_RegisterMember(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Should have a check
	_, checks, err := state.NodeChecks(nil, c1.config.NodeName, nil, "")
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
		_, node, err := state.GetNode(s1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatalf("server not registered")
		}
	})

	// Service should be registered
	_, services, err := state.NodeServices(nil, s1.config.NodeName, nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services.Services["consul"]; !ok {
		t.Fatalf("consul service not registered: %v", services)
	}
}

func TestLeader_FailedMember(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})

	// Should have a check
	_, checks, err := state.NodeChecks(nil, c1.config.NodeName, nil, "")
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
		_, checks, err = state.NodeChecks(nil, c1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if len(checks) != 1 {
			r.Fatalf("client missing check")
		}
		if got, want := checks[0].Status, api.HealthCritical; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestLeader_LeftMember(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		require.NoError(r, err)
		require.NotNil(r, node, "client not registered")
	})

	// Node should leave
	c1.Leave()
	c1.Shutdown()

	// Should be deregistered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		require.NoError(r, err)
		require.Nil(r, node, "client still registered")
	})
}

func TestLeader_ReapMember(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		require.NoError(r, err)
		require.NotNil(r, node, "client not registered")
	})

	// Simulate a node reaping
	mems := s1.LANMembersInAgentPartition()
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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		require.NoError(t, err)
		if node == nil {
			reaped = true
			break
		}
	}
	if !reaped {
		t.Fatalf("client should not be registered")
	}
}

func TestLeader_ReapOrLeftMember_IgnoreSelf(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	run := func(t *testing.T, status serf.MemberStatus, nameFn func(string) string) {
		t.Parallel()
		dir1, s1 := testServerWithConfig(t, func(c *Config) {
			c.PrimaryDatacenter = "dc1"
			c.ACLsEnabled = true
			c.ACLInitialManagementToken = "root"
			c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		})
		defer os.RemoveAll(dir1)
		defer s1.Shutdown()

		nodeName := s1.config.NodeName
		if nameFn != nil {
			nodeName = nameFn(nodeName)
		}

		state := s1.fsm.State()

		// Should be registered
		retry.Run(t, func(r *retry.R) {
			_, node, err := state.GetNode(nodeName, nil, "")
			require.NoError(r, err)
			require.NotNil(r, node, "server not registered")
		})

		// Simulate THIS node reaping or leaving
		mems := s1.LANMembersInAgentPartition()
		var s1mem serf.Member
		for _, m := range mems {
			if strings.EqualFold(m.Name, nodeName) {
				s1mem = m
				s1mem.Status = status
				s1mem.Name = nodeName
				break
			}
		}
		s1.reconcileCh <- s1mem

		// Should NOT be deregistered; we have to poll quickly here because
		// anti-entropy will put it back if it did get deleted.
		reaped := false
		for start := time.Now(); time.Since(start) < 5*time.Second; {
			_, node, err := state.GetNode(nodeName, nil, "")
			require.NoError(t, err)
			if node == nil {
				reaped = true
				break
			}
		}
		if reaped {
			t.Fatalf("server should still be registered")
		}
	}

	t.Run("original name", func(t *testing.T) {
		t.Parallel()
		t.Run("left", func(t *testing.T) {
			run(t, serf.StatusLeft, nil)
		})
		t.Run("reap", func(t *testing.T) {
			run(t, StatusReap, nil)
		})
	})

	t.Run("uppercased name", func(t *testing.T) {
		t.Parallel()
		t.Run("left", func(t *testing.T) {
			run(t, serf.StatusLeft, strings.ToUpper)
		})
		t.Run("reap", func(t *testing.T) {
			run(t, StatusReap, strings.ToUpper)
		})
	})
}

func TestLeader_CheckServersMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	ports := freeport.GetN(t, 2) // s3 grpc, s3 grpc_tls

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
		c.Bootstrap = false
		c.GRPCPort = ports[0]
		c.GRPCTLSPort = ports[1]
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

	consulService := &structs.NodeService{
		ID:      "consul",
		Service: "consul",
	}
	// s3 should be registered
	retry.Run(t, func(r *retry.R) {
		_, service, err := state.NodeService(nil, s3.config.NodeName, "consul", &consulService.EnterpriseMeta, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if service == nil {
			r.Fatal("client not registered")
		}
		if service.Meta["non_voter"] != "false" {
			r.Fatalf("Expected to be non_voter == false, was: %s", service.Meta["non_voter"])
		}
	})

	member := serf.Member{}
	for _, m := range s1.serfLAN.Members() {
		if m.Name == s3.config.NodeName {
			member = m
			member.Tags = make(map[string]string)
			for key, value := range m.Tags {
				member.Tags[key] = value
			}
		}
	}
	if member.Name != s3.config.NodeName {
		t.Fatal("could not find node in serf members")
	}
	versionToExpect := "19.7.9"

	retry.Run(t, func(r *retry.R) {
		// DEPRECATED - remove nonvoter tag in favor of read_replica in a future version of consul
		member.Tags["nonvoter"] = "1"
		member.Tags["read_replica"] = "1"
		member.Tags["build"] = versionToExpect
		err := s1.registrator.HandleAliveMember(member, nil, s1.joinConsulServer)
		if err != nil {
			r.Fatalf("Unexpected error :%v", err)
		}
		_, service, err := state.NodeService(nil, s3.config.NodeName, "consul", &consulService.EnterpriseMeta, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if service == nil {
			r.Fatal("client not registered")
		}
		// DEPRECATED - remove non_voter in favor of read_replica in a future version of consul
		if service.Meta["non_voter"] != "true" {
			r.Fatalf("Expected to be non_voter == true, was: %s", service.Meta["non_voter"])
		}
		if service.Meta["read_replica"] != "true" {
			r.Fatalf("Expected to be read_replica == true, was: %s", service.Meta["non_voter"])
		}
		newVersion := service.Meta["version"]
		if newVersion != versionToExpect {
			r.Fatalf("Expected version to be updated to %s, was %s", versionToExpect, newVersion)
		}
		grpcPort := service.Meta["grpc_port"]
		if grpcPort != strconv.Itoa(ports[0]) {
			r.Fatalf("Expected grpc port to be %d, was %s", ports[0], grpcPort)
		}
		grpcTLSPort := service.Meta["grpc_tls_port"]
		if grpcTLSPort != strconv.Itoa(ports[1]) {
			r.Fatalf("Expected grpc tls port to be %d, was %s", ports[1], grpcTLSPort)
		}
	})
}

func TestLeader_ReapServer(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
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
		_, node, err := state.GetNode(s3.config.NodeName, nil, "")
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

	err := s1.reconcileReaped(knownMembers, nil)

	if err != nil {
		t.Fatalf("Unexpected error :%v", err)
	}
	// s3 should be deregistered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(s3.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node != nil {
			r.Fatalf("server with id %v should not be registered", s3.config.NodeID)
		}
	})

}

func TestLeader_Reconcile_ReapMember(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
	if err := s1.RPC(context.Background(), "Catalog.Register", &dead, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force a reconciliation
	if err := s1.reconcile(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Node should be gone
	state := s1.fsm.State()
	_, node, err := state.GetNode("no-longer-around", nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("client registered")
	}
}

func TestLeader_Reconcile(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
	_, node, err := state.GetNode(c1.config.NodeName, nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("client registered")
	}

	// Should be registered
	retry.Run(t, func(r *retry.R) {
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if node == nil {
			r.Fatal("client not registered")
		}
	})
}

func TestLeader_Reconcile_Races(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		_, node, err := state.GetNode(c1.config.NodeName, nil, "")
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
	if err := s1.RPC(context.Background(), "Catalog.Register", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force a reconcile and make sure the metadata stuck around.
	if err := s1.reconcile(); err != nil {
		t.Fatalf("err: %v", err)
	}
	_, node, err := state.GetNode(c1.config.NodeName, nil, "")
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
		_, checks, err := state.NodeChecks(nil, c1.config.NodeName, nil, "")
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if len(checks) != 1 {
			r.Fatalf("client missing check")
		}
		if got, want := checks[0].Status, api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})

	// Make sure the metadata didn't get clobbered.
	_, node, err = state.GetNode(c1.config.NodeName, nil, "")
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if err := servers[1].RemoveFailedNode(servers[0].config.NodeName, false, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait until the remaining servers show only 2 peers.
	for _, s := range servers[1:] {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 2)) })
	}
	s1.Shutdown()
}

func TestLeader_LeftLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		_, node, err := state.GetNode(leader.config.NodeName, nil, "")
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
		peers, _ := s.autopilot.NumVoters()
		if peers != 1 {
			t.Fatalf("should only have 1 raft peer!")
		}
	}
}
