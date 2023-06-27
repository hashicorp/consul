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
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerTrustBundle(t *testing.T) {
	const (
		index    uint64 = 123
		peerName        = "peer1"
	)

	store := state.NewStateStore(nil)

	// Peering must exist for ptb write to succeed
	require.NoError(t, store.PeeringWrite(index-1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name: peerName,
			ID:   "2ae8c79e-242e-4f4a-afd6-9aede8831c5f",
		},
	}))

	require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
		PeerName:    peerName,
		TrustDomain: "before.com",
	}))

	dataSource := ServerTrustBundle(ServerDataSourceDeps{
		GetStore:    func() Store { return store },
		ACLResolver: newStaticResolver(acl.ManageAll()),
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

func TestServerTrustBundle_ACLEnforcement(t *testing.T) {
	const (
		index    uint64 = 123
		peerName        = "peer1"
	)

	store := state.NewStateStore(nil)

	// Peering must exist for ptb write to succeed
	require.NoError(t, store.PeeringWrite(index-1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name: peerName,
			ID:   "2ae8c79e-242e-4f4a-afd6-9aede8831c5f",
		},
	}))

	require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
		PeerName:    peerName,
		TrustDomain: "before.com",
	}))

	testutil.RunStep(t, "can read", func(t *testing.T) {
		authz := policyAuthorizer(t, `
		service "web" { policy = "write" }`)
		dataSource := ServerTrustBundle(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.TrustBundleReadRequest{
			Request: &pbpeering.TrustBundleReadRequest{
				Name: peerName,
			},
		}, "", eventCh)
		require.NoError(t, err)

		result := getEventResult[*pbpeering.TrustBundleReadResponse](t, eventCh)
		require.Equal(t, "before.com", result.Bundle.TrustDomain)
	})

	testutil.RunStep(t, "can't read", func(t *testing.T) {
		authz := policyAuthorizer(t, ``)
		dataSource := ServerTrustBundle(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(authz),
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), &cachetype.TrustBundleReadRequest{
			Request: &pbpeering.TrustBundleReadRequest{
				Name: peerName,
			},
		}, "", eventCh)
		require.NoError(t, err)

		err = getEventError(t, eventCh)
		require.Contains(t, err.Error(), "token with AccessorID '' lacks permission 'service:write' on \"any service\"")
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
			require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:    testUUID(t),
					Name:  them,
					State: pbpeering.PeeringState_ACTIVE,
				},
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
							{Peer: them},
						},
					},
				},
			}))
		})

		dataSource := ServerTrustBundleList(ServerDataSourceDeps{
			Datacenter:  "dc1",
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(acl.ManageAll()),
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

		// Peering must exist for ptb write to succeed
		require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "peer1",
				ID:   "2ae8c79e-242e-4f4a-afd6-9aede8831c5f",
			},
		}))

		require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "peer2",
				ID:   "e69f14e3-f253-43bc-bdbe-888994ca4f81",
			},
		}))

		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer1",
		}))
		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer2",
		}))

		dataSource := ServerTrustBundleList(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			ACLResolver: newStaticResolver(acl.ManageAll()),
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

func TestServerTrustBundleList_ACLEnforcement(t *testing.T) {
	const index uint64 = 123
	var (
		authzWriteWeb = policyAuthorizer(t, `service "web" { policy = "write" }`)
		authzWriteAll = policyAuthorizer(t, `service "" { policy = "write" }`)
		authzNothing  = policyAuthorizer(t, ``)
	)

	t.Run("ACL enforcement: list by service", func(t *testing.T) {
		const (
			serviceName = "web"
			us          = "default"
			them        = "peer2"
		)

		store := state.NewStateStore(nil)
		require.NoError(t, store.CASetConfig(index, &structs.CAConfiguration{ClusterID: "cluster-id"}))

		testutil.RunStep(t, "export service to peer", func(t *testing.T) {
			require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:    testUUID(t),
					Name:  them,
					State: pbpeering.PeeringState_ACTIVE,
				},
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
							{Peer: them},
						},
					},
				},
			}))
		})

		testutil.RunStep(t, "can read", func(t *testing.T) {
			dataSource := ServerTrustBundleList(ServerDataSourceDeps{
				Datacenter:  "dc1",
				GetStore:    func() Store { return store },
				ACLResolver: newStaticResolver(authzWriteWeb),
			})

			eventCh := make(chan proxycfg.UpdateEvent)
			err := dataSource.Notify(context.Background(), &cachetype.TrustBundleListRequest{
				Request: &pbpeering.TrustBundleListByServiceRequest{
					ServiceName: serviceName,
					Partition:   us,
				},
			}, "", eventCh)
			require.NoError(t, err)

			result := getEventResult[*pbpeering.TrustBundleListByServiceResponse](t, eventCh)
			require.Len(t, result.Bundles, 1)
		})

		testutil.RunStep(t, "can't read", func(t *testing.T) {
			dataSource := ServerTrustBundleList(ServerDataSourceDeps{
				Datacenter:  "dc1",
				GetStore:    func() Store { return store },
				ACLResolver: newStaticResolver(authzNothing),
			})

			eventCh := make(chan proxycfg.UpdateEvent)
			err := dataSource.Notify(context.Background(), &cachetype.TrustBundleListRequest{
				Request: &pbpeering.TrustBundleListByServiceRequest{
					ServiceName: serviceName,
					Partition:   us,
				},
			}, "", eventCh)
			require.NoError(t, err)

			err = getEventError(t, eventCh)
			require.Contains(t, err.Error(), "token with AccessorID '' lacks permission 'service:write' on \"web\"")
		})
	})

	t.Run("ACL Enforcement: list for mesh gateway", func(t *testing.T) {
		store := state.NewStateStore(nil)
		require.NoError(t, store.CASetConfig(index, &structs.CAConfiguration{ClusterID: "cluster-id"}))

		// Peering must exist for ptb write to succeed
		require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "peer1",
				ID:   "2ae8c79e-242e-4f4a-afd6-9aede8831c5f",
			},
		}))

		require.NoError(t, store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "peer2",
				ID:   "e69f14e3-f253-43bc-bdbe-888994ca4f81",
			},
		}))

		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer1",
		}))
		require.NoError(t, store.PeeringTrustBundleWrite(index, &pbpeering.PeeringTrustBundle{
			PeerName: "peer2",
		}))

		testutil.RunStep(t, "can read", func(t *testing.T) {
			dataSource := ServerTrustBundleList(ServerDataSourceDeps{
				Datacenter:  "dc1",
				GetStore:    func() Store { return store },
				ACLResolver: newStaticResolver(authzWriteAll),
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

		testutil.RunStep(t, "can't read", func(t *testing.T) {
			dataSource := ServerTrustBundleList(ServerDataSourceDeps{
				Datacenter:  "dc1",
				GetStore:    func() Store { return store },
				ACLResolver: newStaticResolver(authzNothing),
			})

			eventCh := make(chan proxycfg.UpdateEvent)
			err := dataSource.Notify(context.Background(), &cachetype.TrustBundleListRequest{
				Request: &pbpeering.TrustBundleListByServiceRequest{
					Kind:      string(structs.ServiceKindMeshGateway),
					Partition: "default",
				},
			}, "", eventCh)
			require.NoError(t, err)

			err = getEventError(t, eventCh)
			require.Contains(t, err.Error(), "token with AccessorID '' lacks permission 'service:write'")
		})
	})
}

func testUUID(t *testing.T) string {
	v, err := lib.GenerateUUID(nil)
	require.NoError(t, err)
	return v
}
