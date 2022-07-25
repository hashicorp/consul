package proxycfgglue

import (
	"context"
	"testing"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerTrustBundle(t *testing.T) {
	const (
		index    uint64 = 123
		peerName        = "peer1"
	)

	store := state.NewStateStore(nil)

	require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
		PeerName:    peerName,
		TrustDomain: "before.com",
	}))

	dataSource := ServerTrustBundle(ServerDataSourceDeps{
		GetStore: func() Store { return store },
	})

	eventCh := make(chan proxycfg.UpdateEvent)
	err := dataSource.Notify(context.Background(), &cachetype.TrustBundleReadRequest{
		Request: &pbpeering.TrustBundleReadRequest{
			Name: peerName,
		},
	}, "", eventCh)
	require.NoError(t, err)

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*pbpeering.TrustBundleReadResponse](t, eventCh)
		require.Equal(t, "before.com", result.Bundle.TrustDomain)
	})

	testutil.RunStep(t, "update trust bundle", func(t *testing.T) {
		require.NoError(t, store.PeeringTrustBundleWrite(index+1, &pbpeering.PeeringTrustBundle{
			PeerName:    peerName,
			TrustDomain: "after.com",
		}))

		result := getEventResult[*pbpeering.TrustBundleReadResponse](t, eventCh)
		require.Equal(t, "after.com", result.Bundle.TrustDomain)
	})
}

func TestServerTrustBundleList(t *testing.T) {
	const index uint64 = 123

	t.Run("list by service", func(t *testing.T) {
		const (
			serviceName = "web"
			us          = "default"
			them        = "peer2"
		)

		store := state.NewStateStore(nil)
		require.NoError(t, store.CASetConfig(index, &structs.CAConfiguration{ClusterID: "cluster-id"}))

		testutil.RunStep(t, "export service to peer", func(t *testing.T) {
			require.NoError(t, store.PeeringWrite(index, &pbpeering.Peering{
				ID:    testUUID(t),
				Name:  them,
				State: pbpeering.PeeringState_ACTIVE,
			}))

			require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
				PeerName: them,
			}))

			require.NoError(t, store.EnsureConfigEntry(index, &structs.ExportedServicesConfigEntry{
				Name: us,
				Services: []structs.ExportedService{
					{
						Name: serviceName,
						Consumers: []structs.ServiceConsumer{
							{PeerName: them},
						},
					},
				},
			}))
		})

		dataSource := ServerTrustBundleList(ServerDataSourceDeps{
			Datacenter: "dc1",
			GetStore:   func() Store { return store },
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.TrustBundleListRequest{
			Request: &pbpeering.TrustBundleListByServiceRequest{
				ServiceName: serviceName,
				Partition:   us,
			},
		}, "", eventCh)
		require.NoError(t, err)

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*pbpeering.TrustBundleListByServiceResponse](t, eventCh)
			require.Len(t, result.Bundles, 1)
		})

		testutil.RunStep(t, "unexport the service", func(t *testing.T) {
			require.NoError(t, store.EnsureConfigEntry(index+1, &structs.ExportedServicesConfigEntry{
				Name:     us,
				Services: []structs.ExportedService{},
			}))

			result := getEventResult[*pbpeering.TrustBundleListByServiceResponse](t, eventCh)
			require.Len(t, result.Bundles, 0)
		})
	})

	t.Run("list for mesh gateway", func(t *testing.T) {
		store := state.NewStateStore(nil)
		require.NoError(t, store.CASetConfig(index, &structs.CAConfiguration{ClusterID: "cluster-id"}))

		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer1",
		}))
		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer2",
		}))

		dataSource := ServerTrustBundleList(ServerDataSourceDeps{
			GetStore: func() Store { return store },
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.TrustBundleListRequest{
			Request: &pbpeering.TrustBundleListByServiceRequest{
				Kind:      string(structs.ServiceKindMeshGateway),
				Partition: "default",
			},
		}, "", eventCh)
		require.NoError(t, err)

		result := getEventResult[*pbpeering.TrustBundleListByServiceResponse](t, eventCh)
		require.Len(t, result.Bundles, 2)
	})
}

func testUUID(t *testing.T) string {
	v, err := lib.GenerateUUID(nil)
	require.NoError(t, err)
	return v
}
