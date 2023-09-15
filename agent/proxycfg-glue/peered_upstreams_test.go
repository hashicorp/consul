// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func registerService(t *testing.T, index uint64, peerName, serviceName, nodeName string, store *state.Store) {
	require.NoError(t, store.EnsureRegistration(index, &structs.RegisterRequest{
		Node:           nodeName,
		Service:        &structs.NodeService{Service: serviceName, ID: serviceName},
		PeerName:       peerName,
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}))

	require.NoError(t, store.EnsureRegistration(index, &structs.RegisterRequest{
		Node: nodeName,
		Service: &structs.NodeService{
			Service: fmt.Sprintf("%s-proxy", serviceName),
			Kind:    structs.ServiceKindConnectProxy,
			Proxy: structs.ConnectProxyConfig{
				DestinationServiceName: serviceName,
			},
		},
		PeerName:       peerName,
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}))
}

func TestServerPeeredUpstreams(t *testing.T) {
	const (
		index    uint64 = 123
		nodeName        = "node-1"
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)
	enableVirtualIPs(t, store)

	registerService(t, index, "peer-1", "web", nodeName, store)

	eventCh := make(chan proxycfg.UpdateEvent)
	dataSource := ServerPeeredUpstreams(ServerDataSourceDeps{
		GetStore:    func() Store { return store },
		ACLResolver: newStaticResolver(acl.ManageAll()),
	})
	require.NoError(t, dataSource.Notify(ctx, &structs.PartitionSpecificRequest{EnterpriseMeta: *acl.DefaultEnterpriseMeta()}, "", eventCh))

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 1)
		require.Equal(t, "peer-1", result.Services[0].Peer)
		require.Equal(t, "web", result.Services[0].ServiceName.Name)
	})

	testutil.RunStep(t, "register another service", func(t *testing.T) {
		registerService(t, index+1, "peer-2", "db", nodeName, store)

		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 2)
	})

	testutil.RunStep(t, "deregister service", func(t *testing.T) {
		require.NoError(t, store.DeleteService(index+2, nodeName, "web", acl.DefaultEnterpriseMeta(), "peer-1"))

		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 1)
	})
}

func TestServerPeeredUpstreams_ACLEnforcement(t *testing.T) {
	const (
		index    uint64 = 123
		nodeName        = "node-1"
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)
	enableVirtualIPs(t, store)

	registerService(t, index, "peer-1", "web", nodeName, store)

	testutil.RunStep(t, "read web", func(t *testing.T) {
		authz := policyAuthorizer(t, `
		service "web" { policy = "write" }`)

		eventCh := make(chan proxycfg.UpdateEvent)
		dataSource := ServerPeeredUpstreams(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})
		require.NoError(t, dataSource.Notify(ctx, &structs.PartitionSpecificRequest{EnterpriseMeta: *acl.DefaultEnterpriseMeta()}, "", eventCh))

		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 1)
		require.Equal(t, "peer-1", result.Services[0].Peer)
		require.Equal(t, "web", result.Services[0].ServiceName.Name)
	})

	testutil.RunStep(t, "can't read web", func(t *testing.T) {
		authz := policyAuthorizer(t, ``)

		eventCh := make(chan proxycfg.UpdateEvent)
		dataSource := ServerPeeredUpstreams(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})
		require.NoError(t, dataSource.Notify(ctx, &structs.PartitionSpecificRequest{EnterpriseMeta: *acl.DefaultEnterpriseMeta()}, "", eventCh))

		err := getEventError(t, eventCh)
		require.Contains(t, err.Error(), "lacks permission 'service:write' on \"any service\"")
	})
}

func enableVirtualIPs(t *testing.T, store *state.Store) {
	t.Helper()

	require.NoError(t, store.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}))
}
