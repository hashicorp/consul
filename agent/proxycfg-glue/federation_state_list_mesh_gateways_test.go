// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerFederationStateListMeshGateways(t *testing.T) {
	const index uint64 = 123

	store := state.NewStateStore(nil)

	authz := policyAuthorizer(t, `
		service_prefix "dc2-" { policy = "read" }
		node_prefix "dc2-" { policy = "read" }

		service_prefix "dc3-" { policy = "read" }
		node_prefix "dc3-" { policy = "read" }
	`)

	require.NoError(t, store.FederationStateSet(index, &structs.FederationState{
		Datacenter: "dc2",
		MeshGateways: structs.CheckServiceNodes{
			{
				Service: &structs.NodeService{Service: "dc2-gw1"},
				Node:    &structs.Node{Node: "dc2-gw1"},
			},
		},
	}))

	// No access to this DC, we shouldn't see it in results.
	require.NoError(t, store.FederationStateSet(index, &structs.FederationState{
		Datacenter: "dc4",
		MeshGateways: structs.CheckServiceNodes{
			{
				Service: &structs.NodeService{Service: "dc4-gw1"},
				Node:    &structs.Node{Node: "dc4-gw1"},
			},
		},
	}))

	dataSource := ServerFederationStateListMeshGateways(ServerDataSourceDeps{
		ACLResolver: newStaticResolver(authz),
		GetStore:    func() Store { return store },
	})

	eventCh := make(chan proxycfg.UpdateEvent)
	require.NoError(t, dataSource.Notify(context.Background(), &structs.DCSpecificRequest{Datacenter: "dc1"}, "", eventCh))

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*structs.DatacenterIndexedCheckServiceNodes](t, eventCh)
		require.Equal(t, map[string]structs.CheckServiceNodes{
			"dc2": {
				{
					Service: &structs.NodeService{Service: "dc2-gw1"},
					Node:    &structs.Node{Node: "dc2-gw1"},
				},
			},
		}, result.DatacenterNodes)
	})

	testutil.RunStep(t, "add new datacenter", func(t *testing.T) {
		require.NoError(t, store.FederationStateSet(index+1, &structs.FederationState{
			Datacenter: "dc3",
			MeshGateways: structs.CheckServiceNodes{
				{
					Service: &structs.NodeService{Service: "dc3-gw1"},
					Node:    &structs.Node{Node: "dc3-gw1"},
				},
			},
		}))

		result := getEventResult[*structs.DatacenterIndexedCheckServiceNodes](t, eventCh)
		require.Equal(t, map[string]structs.CheckServiceNodes{
			"dc2": {
				{
					Service: &structs.NodeService{Service: "dc2-gw1"},
					Node:    &structs.Node{Node: "dc2-gw1"},
				},
			},
			"dc3": {
				{
					Service: &structs.NodeService{Service: "dc3-gw1"},
					Node:    &structs.Node{Node: "dc3-gw1"},
				},
			},
		}, result.DatacenterNodes)
	})

	testutil.RunStep(t, "delete datacenter", func(t *testing.T) {
		require.NoError(t, store.FederationStateDelete(index+2, "dc3"))

		result := getEventResult[*structs.DatacenterIndexedCheckServiceNodes](t, eventCh)
		require.NotContains(t, result.DatacenterNodes, "dc3")
	})
}
