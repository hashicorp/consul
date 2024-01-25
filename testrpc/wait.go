// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type rpcFn func(context.Context, string, interface{}, interface{}) error

// WaitForLeader ensures we have a leader and a node registration. It
// does not wait for the Consul (node) service to be ready. Use `WaitForTestAgent`
// to make sure the Consul service is ready.
//
// Most uses of this would be better served in the agent/consul package by
// using waitForLeaderEstablishment() instead.
func WaitForLeader(t *testing.T, rpc rpcFn, dc string, options ...waitOption) {
	t.Helper()

	flat := flattenOptions(options)
	if flat.WaitForAntiEntropySync {
		t.Fatalf("WaitForLeader doesn't accept the WaitForAntiEntropySync option")
	}

	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{
			Datacenter:   dc,
			QueryOptions: structs.QueryOptions{Token: flat.Token},
		}
		if err := rpc(context.Background(), "Catalog.ListNodes", args, &out); err != nil {
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
func WaitUntilNoLeader(t *testing.T, rpc rpcFn, dc string, options ...waitOption) {
	t.Helper()

	flat := flattenOptions(options)
	if flat.WaitForAntiEntropySync {
		t.Fatalf("WaitUntilNoLeader doesn't accept the WaitForAntiEntropySync option")
	}

	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{
			Datacenter:   dc,
			QueryOptions: structs.QueryOptions{Token: flat.Token},
		}
		if err := rpc(context.Background(), "Catalog.ListNodes", args, &out); err == nil {
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

func flattenOptions(options []waitOption) waitOption {
	var flat waitOption
	for _, opt := range options {
		if opt.Token != "" {
			flat.Token = opt.Token
		}
		if opt.WaitForAntiEntropySync {
			flat.WaitForAntiEntropySync = true
		}
	}
	return flat
}

// WaitForTestAgent ensures we have a node with serfHealth check registered.
// You'll want to use this if you expect the Consul (node) service to be ready.
func WaitForTestAgent(t *testing.T, rpc rpcFn, dc string, options ...waitOption) {
	t.Helper()

	flat := flattenOptions(options)

	var nodes structs.IndexedNodes
	var checks structs.IndexedHealthChecks

	retry.Run(t, func(r *retry.R) {
		dcReq := &structs.DCSpecificRequest{
			Datacenter:   dc,
			QueryOptions: structs.QueryOptions{Token: flat.Token},
		}
		if err := rpc(context.Background(), "Catalog.ListNodes", dcReq, &nodes); err != nil {
			r.Fatalf("Catalog.ListNodes failed: %v", err)
		}
		if len(nodes.Nodes) == 0 {
			r.Fatalf("No registered nodes")
		}

		if flat.WaitForAntiEntropySync {
			if len(nodes.Nodes[0].TaggedAddresses) == 0 {
				r.Fatalf("Not synced via anti entropy yet")
			}
		}

		// This assumes that there is a single agent per dc, typically a TestAgent
		nodeReq := &structs.NodeSpecificRequest{
			Datacenter:   dc,
			Node:         nodes.Nodes[0].Node,
			QueryOptions: structs.QueryOptions{Token: flat.Token},
		}
		if err := rpc(context.Background(), "Health.NodeChecks", nodeReq, &checks); err != nil {
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
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{
			Datacenter: dc,
		}
		var reply structs.IndexedCARoots
		if err := rpc(context.Background(), "ConnectCA.Roots", args, &reply); err != nil {
			r.Fatalf("err: %v", err)
		}

		root := reply.Active()
		if root == nil {
			r.Fatal("no active root")
		}
		if expect != nil && root.ID != expect.ID {
			r.Fatalf("current active root is %s; waiting for %s", root.ID, expect.ID)
		}
	})
}

// WaitForServiceIntentions waits until the server can accept config entry
// kinds of service-intentions meaning any migration bootstrapping from pre-1.9
// intentions has completed.
func WaitForServiceIntentions(t *testing.T, rpc rpcFn, dc string) {
	const fakeConfigName = "Sa4ohw5raith4si0Ohwuqu3lowiethoh"
	retry.Run(t, func(r *retry.R) {
		args := &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryDelete,
			Datacenter: dc,
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: fakeConfigName,
			},
		}
		var ignored structs.ConfigEntryDeleteResponse
		if err := rpc(context.Background(), "ConfigEntry.Delete", args, &ignored); err != nil {
			r.Fatalf("err: %v", err)
		}
	})
}

func WaitForACLReplication(t *testing.T, rpc rpcFn, dc string, expectedReplicationType structs.ACLReplicationType, minPolicyIndex, minTokenIndex, minRoleIndex uint64) {
	retry.Run(t, func(r *retry.R) {
		args := structs.DCSpecificRequest{
			Datacenter: dc,
		}
		var reply structs.ACLReplicationStatus

		require.NoError(r, rpc(context.Background(), "ACL.ReplicationStatus", &args, &reply))

		require.Equal(r, expectedReplicationType, reply.ReplicationType)
		require.True(r, reply.Running, "Server not running new replicator yet")
		require.True(r, reply.ReplicatedIndex >= minPolicyIndex, "Server hasn't replicated enough policies")
		require.True(r, reply.ReplicatedTokenIndex >= minTokenIndex, "Server hasn't replicated enough tokens")
		require.True(r, reply.ReplicatedRoleIndex >= minRoleIndex, "Server hasn't replicated enough roles")
	})
}
