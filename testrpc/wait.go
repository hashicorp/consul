package testrpc

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

type rpcFn func(string, interface{}, interface{}) error

// WaitForLeader ensures we have a leader and a node registration.
func WaitForLeader(t *testing.T, rpc rpcFn, dc string) {
	t.Helper()

	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{Datacenter: dc}
		if err := rpc("Catalog.ListNodes", args, &out); err != nil {
			r.Fatalf("Catalog.ListNodes failed: %v", err)
		}
		if !out.QueryMeta.KnownLeader {
			r.Fatalf("No leader")
		}
		if out.Index < 2 {
			r.Fatalf("Consul index should be at least 2 in %s", dc)
		}
	})
}

// WaitUntilNoLeader ensures no leader is present, useful for testing lost leadership.
func WaitUntilNoLeader(t *testing.T, rpc rpcFn, dc string) {
	t.Helper()

	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{Datacenter: dc}
		if err := rpc("Catalog.ListNodes", args, &out); err == nil {
			r.Fatalf("It still has a leader: %#v", out)
		}
		if out.QueryMeta.KnownLeader {
			r.Fatalf("Has still a leader")
		}
	})
}

type waitOption struct {
	Token                  string
	WaitForAntiEntropySync bool
}

func WithToken(token string) waitOption {
	return waitOption{Token: token}
}

func WaitForAntiEntropySync() waitOption {
	return waitOption{WaitForAntiEntropySync: true}
}

// WaitForTestAgent ensures we have a node with serfHealth check registered
func WaitForTestAgent(t *testing.T, rpc rpcFn, dc string, options ...waitOption) {
	t.Helper()

	var nodes structs.IndexedNodes
	var checks structs.IndexedHealthChecks

	var (
		token                  string
		waitForAntiEntropySync bool
	)
	for _, opt := range options {
		if opt.Token != "" {
			token = opt.Token
		}
		if opt.WaitForAntiEntropySync {
			waitForAntiEntropySync = true
		}
	}

	retry.Run(t, func(r *retry.R) {
		dcReq := &structs.DCSpecificRequest{
			Datacenter:   dc,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		if err := rpc("Catalog.ListNodes", dcReq, &nodes); err != nil {
			r.Fatalf("Catalog.ListNodes failed: %v", err)
		}
		if len(nodes.Nodes) == 0 {
			r.Fatalf("No registered nodes")
		}

		if waitForAntiEntropySync {
			if len(nodes.Nodes[0].TaggedAddresses) == 0 {
				r.Fatalf("Not synced via anti entropy yet")
			}
		}

		// This assumes that there is a single agent per dc, typically a TestAgent
		nodeReq := &structs.NodeSpecificRequest{
			Datacenter:   dc,
			Node:         nodes.Nodes[0].Node,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		if err := rpc("Health.NodeChecks", nodeReq, &checks); err != nil {
			r.Fatalf("Health.NodeChecks failed: %v", err)
		}

		var found bool
		for _, check := range checks.HealthChecks {
			if check.CheckID == "serfHealth" {
				found = true
				break
			}
		}
		if !found {
			r.Fatalf("serfHealth check not found")
		}
	})
}

// WaitForActiveCARoot polls until the server returns an active Connect root CA
// with the same ID field as expect. If expect is nil, it just waits until _any_
// active root is returned. This is useful because initializing CA happens after
// raft leadership is gained so WaitForLeader isn't sufficient to be sure that
// the CA is fully initialized.
func WaitForActiveCARoot(t *testing.T, rpc rpcFn, dc string, expect *structs.CARoot) {
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{
			Datacenter: dc,
		}
		var reply structs.IndexedCARoots
		if err := rpc("ConnectCA.Roots", args, &reply); err != nil {
			r.Fatalf("err: %v", err)
		}

		var root *structs.CARoot
		for _, r := range reply.Roots {
			if r.ID == reply.ActiveRootID {
				root = r
				break
			}
		}
		if root == nil {
			r.Fatal("no active root")
		}
		if expect != nil && root.ID != expect.ID {
			r.Fatalf("current active root is %s; waiting for %s", root.ID, expect.ID)
		}
	})
}

func WaitForACLReplication(t *testing.T, rpc rpcFn, dc string, expectedReplicationType structs.ACLReplicationType, minPolicyIndex, minTokenIndex, minRoleIndex uint64) {
	retry.Run(t, func(r *retry.R) {
		args := structs.DCSpecificRequest{
			Datacenter: dc,
		}
		var reply structs.ACLReplicationStatus

		require.NoError(r, rpc("ACL.ReplicationStatus", &args, &reply))

		require.Equal(r, expectedReplicationType, reply.ReplicationType)
		require.True(r, reply.Running, "Server not running new replicator yet")
		require.True(r, reply.ReplicatedIndex >= minPolicyIndex, "Server hasn't replicated enough policies")
		require.True(r, reply.ReplicatedTokenIndex >= minTokenIndex, "Server hasn't replicated enough tokens")
		require.True(r, reply.ReplicatedRoleIndex >= minRoleIndex, "Server hasn't replicated enough roles")
	})
}
