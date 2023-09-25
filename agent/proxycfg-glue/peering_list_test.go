// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerPeeringList(t *testing.T) {
	const (
		index uint64 = 123
	)

	store := state.NewStateStore(nil)

	req := pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name: "peer-01",
			ID:   "00000000-0000-0000-0000-000000000000",
		},
	}

	require.NoError(t, store.PeeringWrite(index, &req))

	dataSource := ServerPeeringList(ServerDataSourceDeps{
		GetStore:    func() Store { return store },
		ACLResolver: newStaticResolver(acl.ManageAll()),
	})

	eventCh := make(chan proxycfg.UpdateEvent)
	err := dataSource.Notify(context.Background(), &cachetype.PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	}, "", eventCh)
	require.NoError(t, err)

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*pbpeering.PeeringListResponse](t, eventCh)
		require.Len(t, result.Peerings, 1)
		require.Equal(t, "peer-01", result.Peerings[0].Name)
		require.Equal(t, index, result.OBSOLETE_Index)
	})

	testutil.RunStep(t, "add peering", func(t *testing.T) {
		req = pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "peer-02",
				ID:   "00000000-0000-0000-0000-000000000001",
			},
		}
		require.NoError(t, store.PeeringWrite(index+1, &req))

		result := getEventResult[*pbpeering.PeeringListResponse](t, eventCh)
		require.Len(t, result.Peerings, 2)
		require.Equal(t, "peer-02", result.Peerings[1].Name)
		require.Equal(t, index+1, result.OBSOLETE_Index)
	})
}

func TestServerPeeringList_ACLEnforcement(t *testing.T) {
	const (
		index uint64 = 123
	)

	store := state.NewStateStore(nil)

	req := pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name: "peer-01",
			ID:   "00000000-0000-0000-0000-000000000000",
		},
	}

	require.NoError(t, store.PeeringWrite(index, &req))

	testutil.RunStep(t, "can read", func(t *testing.T) {
		authz := policyAuthorizer(t, `
		peering = "read"`)
		dataSource := ServerPeeringList(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.PeeringListRequest{
			Request: &pbpeering.PeeringListRequest{},
		}, "", eventCh)
		require.NoError(t, err)

		result := getEventResult[*pbpeering.PeeringListResponse](t, eventCh)
		require.Len(t, result.Peerings, 1)
		require.Equal(t, "peer-01", result.Peerings[0].Name)
		require.Equal(t, index, result.OBSOLETE_Index)
	})

	testutil.RunStep(t, "can't read", func(t *testing.T) {
		authz := policyAuthorizer(t, ``)
		dataSource := ServerPeeringList(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.PeeringListRequest{
			Request: &pbpeering.PeeringListRequest{},
		}, "", eventCh)
		require.NoError(t, err)

		err = getEventError(t, eventCh)
		require.Contains(t, err.Error(), "token with AccessorID '' lacks permission 'peering:read'")
	})
}
