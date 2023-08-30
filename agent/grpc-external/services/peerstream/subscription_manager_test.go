package peerstream

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbpeerstream"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestSubscriptionManager_RegisterDeregister(t *testing.T) {
	backend := newTestSubscriptionBackend(t)
	// initialCatalogIdx := backend.lastIdx

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	// Only configure a tracker for catalog events.
	tracker := newResourceSubscriptionTracker()
	tracker.Subscribe(pbpeerstream.TypeURLExportedService)

	mgr := newSubscriptionManager(ctx, testutil.Logger(t), Config{
		Datacenter:     "dc1",
		ConnectEnabled: true,
	}, connect.TestTrustDomain, backend, func() StateStore {
		return backend.store
	}, tracker)
	subCh := mgr.subscribe(ctx, id, "my-peering", partition)

	var (
		mysqlCorrID      = subExportedService + structs.NewServiceName("mysql", nil).String()
		mysqlProxyCorrID = subExportedService + structs.NewServiceName("mysql-sidecar-proxy", nil).String()
	)

	// Expect just the empty mesh gateway event to replicate.
	expectEvents(t, subCh,
		func(t *testing.T, got cache.UpdateEvent) {
			checkExportedServices(t, got, []string{})
		})

	// Initially add in L4 failover so that later we can test removing it. We
	// cannot do the other way around because it would fail validation to
	// remove a target.
	backend.ensureConfigEntry(t, &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "mysql",
		Failover: map[string]structs.ServiceResolverFailover{
			"*": {
				Service: "failover",
			},
		},
	})

	testutil.RunStep(t, "initial export syncs empty instance lists", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-other-peering"},
					},
				},
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkExportedServices(t, got, []string{"mysql"})
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 0)
			},
		)
	})

	mysql1 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-check", ServiceID: "mysql-1", Node: "foo"},
		},
	}

	testutil.RunStep(t, "registering exported service instance yields update", func(t *testing.T) {
		backend.ensureNode(t, mysql1.Node)
		backend.ensureService(t, "foo", mysql1.Service)

		// We get one update for the service
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)

			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("foo", "10.0.0.1", partition),
				Service: pbService("", "mysql-1", "mysql", 5000, nil),
			}, res.Nodes[0])
		})

		backend.ensureCheck(t, mysql1.Checks[0])

		// and one for the check
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)

			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("foo", "10.0.0.1", partition),
				Service: pbService("", "mysql-1", "mysql", 5000, nil),
				Checks: []*pbservice.HealthCheck{
					pbCheck("foo", "mysql-1", "mysql", "critical", nil),
				},
			}, res.Nodes[0])
		})
	})

	mysql2 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "bar", Address: "10.0.0.2"},
		Service: &structs.NodeService{ID: "mysql-2", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-2-check", ServiceID: "mysql-2", Node: "bar"},
		},
	}

	testutil.RunStep(t, "additional instances are returned when registered", func(t *testing.T) {
		backend.ensureNode(t, mysql2.Node)
		backend.ensureService(t, "bar", mysql2.Service)

		// We get one update for the service
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 2)

			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("bar", "10.0.0.2", partition),
				Service: pbService("", "mysql-2", "mysql", 5000, nil),
			}, res.Nodes[0])
			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("foo", "10.0.0.1", partition),
				Service: pbService("", "mysql-1", "mysql", 5000, nil),
				Checks: []*pbservice.HealthCheck{
					pbCheck("foo", "mysql-1", "mysql", "critical", nil),
				},
			}, res.Nodes[1])
		})

		backend.ensureCheck(t, mysql2.Checks[0])

		// and one for the check
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 2)
			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("bar", "10.0.0.2", partition),
				Service: pbService("", "mysql-2", "mysql", 5000, nil),
				Checks: []*pbservice.HealthCheck{
					pbCheck("bar", "mysql-2", "mysql", "critical", nil),
				},
			}, res.Nodes[0])
			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("foo", "10.0.0.1", partition),
				Service: pbService("", "mysql-1", "mysql", 5000, nil),
				Checks: []*pbservice.HealthCheck{
					pbCheck("foo", "mysql-1", "mysql", "critical", nil),
				},
			}, res.Nodes[1])
		})
	})

	mongo := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{ID: "mongo", Service: "mongo", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mongo-check", ServiceID: "mongo", Node: "zip"},
		},
	}

	testutil.RunStep(t, "no updates are received for services not exported to my-peering", func(t *testing.T) {
		backend.ensureNode(t, mongo.Node)
		backend.ensureService(t, "zip", mongo.Service)
		backend.ensureCheck(t, mongo.Checks[0])

		// Receive from subCh times out.
		expectEvents(t, subCh)
	})

	testutil.RunStep(t, "deregister an instance and it gets removed from the output", func(t *testing.T) {
		backend.deleteService(t, "foo", mysql1.Service.ID)

		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)
			prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
				Node:    pbNode("bar", "10.0.0.2", partition),
				Service: pbService("", "mysql-2", "mysql", 5000, nil),
				Checks: []*pbservice.HealthCheck{
					pbCheck("bar", "mysql-2", "mysql", "critical", nil),
				},
			}, res.Nodes[0])
		})
	})

	testutil.RunStep(t, "register mesh gateway to send proxy updates", func(t *testing.T) {
		gateway := &structs.CheckServiceNode{
			Node:    &structs.Node{Node: "mgw", Address: "10.1.1.1"},
			Service: &structs.NodeService{ID: "gateway-1", Kind: structs.ServiceKindMeshGateway, Service: "gateway", Port: 8443},
			// TODO: checks
		}
		backend.ensureNode(t, gateway.Node)
		backend.ensureService(t, "mgw", gateway.Service)

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, mysqlProxyCorrID, got.CorrelationID)
				res := got.Result.(*pbservice.IndexedCheckServiceNodes)
				require.Equal(t, uint64(0), res.Index)

				require.Len(t, res.Nodes, 1)
				prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
					Node: pbNode("mgw", "10.1.1.1", partition),
					Service: &pbservice.NodeService{
						Kind:    "connect-proxy",
						ID:      "mysql-sidecar-proxy-instance-0",
						Service: "mysql-sidecar-proxy",
						Port:    8443,
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
						Proxy: &pbservice.ConnectProxyConfig{
							DestinationServiceID:   "mysql-instance-0",
							DestinationServiceName: "mysql",
						},
						Connect: &pbservice.ServiceConnect{
							PeerMeta: &pbservice.PeeringServiceMeta{
								SNI: []string{
									"mysql.default.default.my-peering.external.11111111-2222-3333-4444-555555555555.consul",
								},
								SpiffeID: []string{
									"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mysql",
									"spiffe://11111111-2222-3333-4444-555555555555.consul/gateway/mesh/dc/dc1",
									"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/failover",
								},
								Protocol: "tcp",
							},
						},
					},
				}, res.Nodes[0])
			},
		)
	})

	testutil.RunStep(t, "peer meta changes when L4 disco chain changes", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ServiceResolverConfigEntry{
			Kind: structs.ServiceResolver,
			Name: "mysql",
		})

		// ensure we get updated peer meta

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, mysqlProxyCorrID, got.CorrelationID)
				res := got.Result.(*pbservice.IndexedCheckServiceNodes)
				require.Equal(t, uint64(0), res.Index)

				require.Len(t, res.Nodes, 1)
				prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
					Node: pbNode("mgw", "10.1.1.1", partition),
					Service: &pbservice.NodeService{
						Kind:    "connect-proxy",
						ID:      "mysql-sidecar-proxy-instance-0",
						Service: "mysql-sidecar-proxy",
						Port:    8443,
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
						Proxy: &pbservice.ConnectProxyConfig{
							DestinationServiceID:   "mysql-instance-0",
							DestinationServiceName: "mysql",
						},
						Connect: &pbservice.ServiceConnect{
							PeerMeta: &pbservice.PeeringServiceMeta{
								SNI: []string{
									"mysql.default.default.my-peering.external.11111111-2222-3333-4444-555555555555.consul",
								},
								SpiffeID: []string{
									"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mysql",
									"spiffe://11111111-2222-3333-4444-555555555555.consul/gateway/mesh/dc/dc1",
								},
								Protocol: "tcp",
							},
						},
					},
				}, res.Nodes[0])
			},
		)
	})

	testutil.RunStep(t, "peer meta changes when protocol switches from L4 to L7", func(t *testing.T) {
		// NOTE: for this test we'll just live in a fantasy realm where we assume
		// that mysql understands gRPC
		backend.ensureConfigEntry(t, &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "mysql",
			Protocol: "grpc",
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, mysqlProxyCorrID, got.CorrelationID)
				res := got.Result.(*pbservice.IndexedCheckServiceNodes)
				require.Equal(t, uint64(0), res.Index)

				require.Len(t, res.Nodes, 1)
				prototest.AssertDeepEqual(t, &pbservice.CheckServiceNode{
					Node: pbNode("mgw", "10.1.1.1", partition),
					Service: &pbservice.NodeService{
						Kind:    "connect-proxy",
						ID:      "mysql-sidecar-proxy-instance-0",
						Service: "mysql-sidecar-proxy",
						Port:    8443,
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						EnterpriseMeta: pbcommon.DefaultEnterpriseMeta,
						Proxy: &pbservice.ConnectProxyConfig{
							DestinationServiceID:   "mysql-instance-0",
							DestinationServiceName: "mysql",
						},
						Connect: &pbservice.ServiceConnect{
							PeerMeta: &pbservice.PeeringServiceMeta{
								SNI: []string{
									"mysql.default.default.my-peering.external.11111111-2222-3333-4444-555555555555.consul",
								},
								SpiffeID: []string{
									"spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/mysql",
									"spiffe://11111111-2222-3333-4444-555555555555.consul/gateway/mesh/dc/dc1",
								},
								Protocol: "grpc",
							},
						},
					},
				}, res.Nodes[0])
			},
		)
	})

	testutil.RunStep(t, "deregister the last instance and the output is empty", func(t *testing.T) {
		backend.deleteService(t, "bar", mysql2.Service.ID)

		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 0)
		})
	})

	testutil.RunStep(t, "deregister mesh gateway to send proxy removals", func(t *testing.T) {
		backend.deleteService(t, "mgw", "gateway-1")

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, mysqlProxyCorrID, got.CorrelationID)
				res := got.Result.(*pbservice.IndexedCheckServiceNodes)
				require.Equal(t, uint64(0), res.Index)

				require.Len(t, res.Nodes, 0)
			},
		)
	})

	testutil.RunStep(t, "unexporting a service emits sends an event", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-other-peering"},
					},
				},
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkExportedServices(t, got, []string{})
			},
		)
	})
}

func TestSubscriptionManager_InitialSnapshot(t *testing.T) {
	backend := newTestSubscriptionBackend(t)
	// initialCatalogIdx := backend.lastIdx

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	// Only configure a tracker for catalog events.
	tracker := newResourceSubscriptionTracker()
	tracker.Subscribe(pbpeerstream.TypeURLExportedService)

	mgr := newSubscriptionManager(ctx, testutil.Logger(t), Config{
		Datacenter:     "dc1",
		ConnectEnabled: true,
	}, connect.TestTrustDomain, backend, func() StateStore {
		return backend.store
	}, tracker)
	subCh := mgr.subscribe(ctx, id, "my-peering", partition)

	// Register two services that are not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}
	mysqlSidecar := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "mysql-sidecar-proxy",
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "mysql",
		},
	}
	backend.ensureNode(t, mysql.Node)
	backend.ensureService(t, "foo", mysql.Service)
	backend.ensureService(t, "foo", &mysqlSidecar)

	mongo := &structs.CheckServiceNode{
		Node: &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{
			ID:      "mongo-1",
			Service: "mongo",
			Port:    5000,
		},
	}
	mongoSidecar := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "mongo-sidecar-proxy",
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "mongo",
		},
	}
	backend.ensureNode(t, mongo.Node)
	backend.ensureService(t, "zip", mongo.Service)
	backend.ensureService(t, "zip", &mongoSidecar)

	backend.ensureConfigEntry(t, &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "chain",
	})

	var (
		mysqlCorrID = subExportedService + structs.NewServiceName("mysql", nil).String()
		mongoCorrID = subExportedService + structs.NewServiceName("mongo", nil).String()
		chainCorrID = subExportedService + structs.NewServiceName("chain", nil).String()

		mysqlProxyCorrID = subExportedService + structs.NewServiceName("mysql-sidecar-proxy", nil).String()
		mongoProxyCorrID = subExportedService + structs.NewServiceName("mongo-sidecar-proxy", nil).String()
		chainProxyCorrID = subExportedService + structs.NewServiceName("chain-sidecar-proxy", nil).String()
	)

	// Expect just the empty mesh gateway event to replicate.
	expectEvents(t, subCh,
		func(t *testing.T, got cache.UpdateEvent) {
			checkExportedServices(t, got, []string{})
		})

	// At this point in time we'll have a mesh-gateway notification with no
	// content stored and handled.
	testutil.RunStep(t, "exporting the two services yields an update for both", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
					},
				},
				{
					Name: "chain",
					Consumers: []structs.ServiceConsumer{
						{Peer: "my-peering"},
					},
				},
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkExportedServices(t, got, []string{"mysql", "chain", "mongo"})
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainProxyCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoCorrID, 1, "mongo", string(structs.ServiceKindTypical))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoProxyCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlCorrID, 1, "mysql", string(structs.ServiceKindTypical))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 0)
			},
		)
	})

	testutil.RunStep(t, "registering a mesh gateway triggers connect replies", func(t *testing.T) {
		gateway := &structs.CheckServiceNode{
			Node:    &structs.Node{Node: "mgw", Address: "10.1.1.1"},
			Service: &structs.NodeService{ID: "gateway-1", Kind: structs.ServiceKindMeshGateway, Service: "gateway", Port: 8443},
			// TODO: checks
		}
		backend.ensureNode(t, gateway.Node)
		backend.ensureService(t, "mgw", gateway.Service)

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainProxyCorrID, 1, "chain-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoProxyCorrID, 1, "mongo-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 1, "mysql-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
		)
	})
}

func TestSubscriptionManager_CARoots(t *testing.T) {
	backend := newTestSubscriptionBackend(t)

	// Setup CA-related configs in the store
	clusterID, rootA := writeInitialRootsAndCA(t, backend.store)
	trustDomain := connect.SpiffeIDSigningForCluster(clusterID).Host()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	// Only configure a tracker for CA roots events.
	tracker := newResourceSubscriptionTracker()
	tracker.Subscribe(pbpeerstream.TypeURLPeeringTrustBundle)

	mgr := newSubscriptionManager(ctx, testutil.Logger(t), Config{
		Datacenter:     "dc1",
		ConnectEnabled: true,
	}, connect.TestTrustDomain, backend, func() StateStore {
		return backend.store
	}, tracker)
	subCh := mgr.subscribe(ctx, id, "my-peering", partition)

	testutil.RunStep(t, "initial events contain trust bundle", func(t *testing.T) {
		// events are ordered so we can expect a deterministic list
		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subCARoot, got.CorrelationID)
				roots, ok := got.Result.(*pbpeering.PeeringTrustBundle)
				require.True(t, ok)

				require.ElementsMatch(t, []string{rootA.RootCert}, roots.RootPEMs)

				require.Equal(t, trustDomain, roots.TrustDomain)
			},
		)
	})

	testutil.RunStep(t, "updating CA roots triggers event", func(t *testing.T) {
		rootB := connect.TestCA(t, nil)
		rootC := connect.TestCA(t, nil)
		rootC.Active = false // there can only be one active root
		backend.ensureCARoots(t, rootB, rootC)

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subCARoot, got.CorrelationID)
				roots, ok := got.Result.(*pbpeering.PeeringTrustBundle)
				require.True(t, ok)

				require.ElementsMatch(t, []string{rootB.RootCert, rootC.RootCert}, roots.RootPEMs)

				require.Equal(t, trustDomain, roots.TrustDomain)
			},
		)
	})
}

func TestSubscriptionManager_ServerAddrs(t *testing.T) {
	backend := newTestSubscriptionBackend(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	payload := autopilotevents.EventPayloadReadyServers{
		autopilotevents.ReadyServerInfo{
			ID:          "9aeb73f6-e83e-43c1-bdc9-ca5e43efe3e4",
			Address:     "198.18.0.1",
			Version:     "1.13.1",
			ExtGRPCPort: 8502,
		},
	}
	// mock handler only gets called once during the initial subscription
	backend.handler.expect("", 0, 1, payload)

	// Only configure a tracker for server address events.
	tracker := newResourceSubscriptionTracker()
	tracker.Subscribe(pbpeerstream.TypeURLPeeringServerAddresses)

	mgr := newSubscriptionManager(ctx,
		testutil.Logger(t),
		Config{
			Datacenter:     "dc1",
			ConnectEnabled: true,
		},
		connect.TestTrustDomain,
		backend,
		func() StateStore {
			return backend.store
		},
		tracker)
	subCh := mgr.subscribe(ctx, id, "my-peering", partition)

	testutil.RunStep(t, "initial events", func(t *testing.T) {
		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)
				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				require.Equal(t, []string{"198.18.0.1:8502"}, addrs.GetAddresses())
			},
		)
	})

	testutil.RunStep(t, "added server", func(t *testing.T) {
		payload = append(payload, autopilotevents.ReadyServerInfo{
			ID:          "eec8721f-c42b-48da-a5a5-07565158015e",
			Address:     "198.18.0.2",
			Version:     "1.13.1",
			ExtGRPCPort: 9502,
		})
		backend.Publish([]stream.Event{
			{
				Topic:   autopilotevents.EventTopicReadyServers,
				Index:   2,
				Payload: payload,
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)
				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				require.Equal(t, []string{"198.18.0.1:8502", "198.18.0.2:9502"}, addrs.GetAddresses())
			},
		)
	})

	testutil.RunStep(t, "added server with WAN address", func(t *testing.T) {
		payload = append(payload, autopilotevents.ReadyServerInfo{
			ID:          "eec8721f-c42b-48da-a5a5-07565158015e",
			Address:     "198.18.0.3",
			Version:     "1.13.1",
			ExtGRPCPort: 9502,
			TaggedAddresses: map[string]string{
				structs.TaggedAddressWAN: "198.18.0.103",
			},
		})
		backend.Publish([]stream.Event{
			{
				Topic:   autopilotevents.EventTopicReadyServers,
				Index:   3,
				Payload: payload,
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)
				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				require.Equal(t, []string{"198.18.0.1:8502", "198.18.0.2:9502", "198.18.0.103:9502"}, addrs.GetAddresses())
			},
		)
	})

	testutil.RunStep(t, "flipped to peering through mesh gateways", func(t *testing.T) {
		require.NoError(t, backend.store.EnsureConfigEntry(1, &structs.MeshConfigEntry{
			Peering: &structs.PeeringMeshConfig{
				PeerThroughMeshGateways: true,
			},
		}))

		select {
		case <-time.After(100 * time.Millisecond):
		case <-subCh:
			t.Fatal("expected to time out: no mesh gateways are registered")
		}
	})

	testutil.RunStep(t, "registered and received a mesh gateway", func(t *testing.T) {
		reg := structs.RegisterRequest{
			ID:      types.NodeID("b5489ca9-f5e9-4dba-a779-61fec4e8e364"),
			Node:    "gw-node",
			Address: "1.2.3.4",
			TaggedAddresses: map[string]string{
				structs.TaggedAddressWAN: "172.217.22.14",
			},
			Service: &structs.NodeService{
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Kind:    structs.ServiceKindMeshGateway,
				Port:    443,
				TaggedAddresses: map[string]structs.ServiceAddress{
					structs.TaggedAddressWAN: {Address: "154.238.12.252", Port: 8443},
				},
			},
		}
		require.NoError(t, backend.store.EnsureRegistration(2, &reg))

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)

				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				require.Equal(t, []string{"154.238.12.252:8443"}, addrs.GetAddresses())
			},
		)
	})

	testutil.RunStep(t, "registered and received a second mesh gateway", func(t *testing.T) {
		reg := structs.RegisterRequest{
			ID:      types.NodeID("e4cc0af3-5c09-4ddf-94a9-5840e427bc45"),
			Node:    "gw-node-2",
			Address: "1.2.3.5",
			TaggedAddresses: map[string]string{
				structs.TaggedAddressWAN: "172.217.22.15",
			},
			Service: &structs.NodeService{
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Kind:    structs.ServiceKindMeshGateway,
				Port:    443,
			},
		}
		require.NoError(t, backend.store.EnsureRegistration(3, &reg))

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)

				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				require.Equal(t, []string{"154.238.12.252:8443", "172.217.22.15:443"}, addrs.GetAddresses())
			},
		)
	})

	testutil.RunStep(t, "disabled peering through gateways and received server addresses", func(t *testing.T) {
		require.NoError(t, backend.store.EnsureConfigEntry(4, &structs.MeshConfigEntry{
			Peering: &structs.PeeringMeshConfig{
				PeerThroughMeshGateways: false,
			},
		}))

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				require.Equal(t, subServerAddrs, got.CorrelationID)

				addrs, ok := got.Result.(*pbpeering.PeeringServerAddresses)
				require.True(t, ok)

				// New subscriptions receive a snapshot from the event publisher.
				// At the start of the test the handler registered a mock that only returns a single address.
				require.Equal(t, []string{"198.18.0.1:8502"}, addrs.GetAddresses())
			},
		)
	})
}

func TestFlattenChecks(t *testing.T) {
	type testcase struct {
		checks         []*pbservice.HealthCheck
		expect         string
		expectNoResult bool
	}

	run := func(t *testing.T, tc testcase) {
		t.Helper()
		got := flattenChecks(
			"node-name", "service-id", "service-name", nil, tc.checks,
		)
		if tc.expectNoResult {
			require.Empty(t, got)
		} else {
			require.Len(t, got, 1)
			require.Equal(t, tc.expect, got[0].Status)
		}
	}

	cases := map[string]testcase{
		"empty": {
			checks:         nil,
			expectNoResult: true,
		},
		"passing": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthPassing,
				},
			},
			expect: api.HealthPassing,
		},
		"warning": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthWarning,
				},
			},
			expect: api.HealthWarning,
		},
		"critical": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthCritical,
				},
			},
			expect: api.HealthCritical,
		},
		"node_maintenance": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: api.NodeMaint,
					Status:  api.HealthPassing,
				},
			},
			expect: api.HealthMaint,
		},
		"service_maintenance": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: api.ServiceMaintPrefix + "service",
					Status:  api.HealthPassing,
				},
			},
			expect: api.HealthMaint,
		},
		"unknown": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  "nope-nope-noper",
				},
			},
			expect: "nope-nope-noper",
		},
		"maintenance_over_critical": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: api.NodeMaint,
					Status:  api.HealthPassing,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthCritical,
				},
			},
			expect: api.HealthMaint,
		},
		"critical_over_warning": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthCritical,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthWarning,
				},
			},
			expect: api.HealthCritical,
		},
		"warning_over_passing": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthWarning,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthPassing,
				},
			},
			expect: api.HealthWarning,
		},
		"lots": {
			checks: []*pbservice.HealthCheck{
				{
					CheckID: "check-id",
					Status:  api.HealthPassing,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthPassing,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthPassing,
				},
				{
					CheckID: "check-id",
					Status:  api.HealthWarning,
				},
			},
			expect: api.HealthWarning,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

type testSubscriptionBackend struct {
	state.EventPublisher
	store   *state.Store
	handler *mockSnapshotHandler

	lastIdx uint64
}

func newTestSubscriptionBackend(t *testing.T) *testSubscriptionBackend {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store, handler := newStateStore(t, publisher)

	backend := &testSubscriptionBackend{
		EventPublisher: publisher,
		store:          store,
		handler:        handler,
	}

	backend.ensureCAConfig(t, &structs.CAConfiguration{
		Provider:  "consul",
		ClusterID: connect.TestClusterID,
	})

	// Create some placeholder data to ensure raft index > 0
	//
	// TODO(peering): is there some extremely subtle max-index table reading bug in play?
	placeholder := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "placeholder", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "placeholder-1", Service: "placeholder", Port: 5000},
	}
	backend.ensureNode(t, placeholder.Node)
	backend.ensureService(t, "placeholder", placeholder.Service)

	return backend
}

//nolint:unparam
func (b *testSubscriptionBackend) ensurePeering(t *testing.T, name string) (uint64, string) {
	b.lastIdx++
	return b.lastIdx, setupTestPeering(t, b.store, name, b.lastIdx)
}

//nolint:unparam
func (b *testSubscriptionBackend) ensureConfigEntry(t *testing.T, entry structs.ConfigEntry) uint64 {
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	b.lastIdx++
	require.NoError(t, b.store.EnsureConfigEntry(b.lastIdx, entry))
	return b.lastIdx
}

func (b *testSubscriptionBackend) deleteConfigEntry(t *testing.T, kind, name string) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.DeleteConfigEntry(b.lastIdx, kind, name, nil))
	return b.lastIdx
}

//nolint:unparam
func (b *testSubscriptionBackend) ensureNode(t *testing.T, node *structs.Node) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureNode(b.lastIdx, node))
	return b.lastIdx
}

//nolint:unparam
func (b *testSubscriptionBackend) ensureService(t *testing.T, node string, svc *structs.NodeService) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureService(b.lastIdx, node, svc))
	return b.lastIdx
}

//nolint:unparam
func (b *testSubscriptionBackend) ensureCheck(t *testing.T, hc *structs.HealthCheck) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureCheck(b.lastIdx, hc))
	return b.lastIdx
}

//nolint:unparam
func (b *testSubscriptionBackend) deleteService(t *testing.T, nodeName, serviceID string) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.DeleteService(b.lastIdx, nodeName, serviceID, nil, ""))
	return b.lastIdx
}

func (b *testSubscriptionBackend) ensureCAConfig(t *testing.T, config *structs.CAConfiguration) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.CASetConfig(b.lastIdx, config))
	return b.lastIdx
}

func (b *testSubscriptionBackend) ensureCARoots(t *testing.T, roots ...*structs.CARoot) uint64 {
	// Get the max index for Check-and-Set operation
	cidx, _, err := b.store.CARoots(nil)
	require.NoError(t, err)

	b.lastIdx++
	set, err := b.store.CARootSetCAS(b.lastIdx, cidx, roots)
	require.True(t, set)
	require.NoError(t, err)
	return b.lastIdx
}

func setupTestPeering(t *testing.T, store *state.Store, name string, index uint64) string {
	err := store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(t),
			Name: name,
		},
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: name})
	require.NoError(t, err)
	require.NotNil(t, p)

	return p.ID
}

func newStateStore(t *testing.T, publisher *stream.EventPublisher) (*state.Store, *mockSnapshotHandler) {
	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	handler := newMockSnapshotHandler(t)

	store := state.NewStateStoreWithEventPublisher(gc, publisher)
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealth, store.ServiceHealthSnapshot, false))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealthConnect, store.ServiceHealthSnapshot, false))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicCARoots, store.CARootsSnapshot, false))
	require.NoError(t, publisher.RegisterHandler(autopilotevents.EventTopicReadyServers, handler.handle, false))

	// WaitGroup used to make sure that the publisher returns
	// before handler's t.Cleanup is called (otherwise an event
	// might fire during an assertion and cause a data race).
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
		publisher.Run(ctx)
		wg.Done()
	}()

	return store, handler
}

func expectEvents(
	t *testing.T,
	ch <-chan cache.UpdateEvent,
	checkFns ...func(t *testing.T, got cache.UpdateEvent),
) {
	t.Helper()

	num := len(checkFns)

	var out []cache.UpdateEvent

	if num == 0 {
		// No updates should be received.
		select {
		case <-ch:
			t.Fatalf("received unexpected update")
		case <-time.After(100 * time.Millisecond):
			// Expect this to fire
		}
		return
	}

	const timeout = 10 * time.Second
	timeoutCh := time.After(timeout)

	for len(out) < num {
		select {
		case <-timeoutCh:
			t.Fatalf("timed out with %d of %d events after %v", len(out), num, timeout)
		case evt := <-ch:
			out = append(out, evt)
		}
	}

	select {
	case <-time.After(100 * time.Millisecond):
	case evt := <-ch:
		t.Fatalf("expected only %d events but got more; prev %+v; next %+v;", num, out, evt)
	}

	require.Len(t, out, num)

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CorrelationID < out[j].CorrelationID
	})

	for i := 0; i < num; i++ {
		checkFns[i](t, out[i])
	}
}

func checkEvent(
	t *testing.T,
	got cache.UpdateEvent,
	correlationID string,
	expectNodes int,
	serviceKindPairs ...string) {
	t.Helper()

	require.True(t, len(serviceKindPairs) == 2*expectNodes, "sanity check")

	require.Equal(t, correlationID, got.CorrelationID)

	evt := got.Result.(*pbservice.IndexedCheckServiceNodes)
	require.Equal(t, uint64(0), evt.Index)

	if expectNodes == 0 {
		require.Len(t, evt.Nodes, 0)
	} else {
		require.Len(t, evt.Nodes, expectNodes)

		for i := 0; i < expectNodes; i++ {
			expectName := serviceKindPairs[i*2]
			expectKind := serviceKindPairs[i*2+1]
			require.Equal(t, expectName, evt.Nodes[i].Service.Service)
			require.Equal(t, expectKind, evt.Nodes[i].Service.Kind)
		}
	}
}

func checkExportedServices(
	t *testing.T,
	got cache.UpdateEvent,
	expectedServices []string,
) {
	t.Helper()

	var qualifiedServices []string
	for _, s := range expectedServices {
		qualifiedServices = append(qualifiedServices, structs.ServiceName{Name: s}.String())
	}

	require.Equal(t, subExportedServiceList, got.CorrelationID)
	evt := got.Result.(*pbpeerstream.ExportedServiceList)
	require.ElementsMatch(t, qualifiedServices, evt.Services)
}

func pbNode(node, addr, partition string) *pbservice.Node {
	return &pbservice.Node{Node: node, Partition: partition, Address: addr}
}

func pbService(kind, id, name string, port int32, entMeta *pbcommon.EnterpriseMeta) *pbservice.NodeService {
	if entMeta == nil {
		entMeta = pbcommon.DefaultEnterpriseMeta
	}
	return &pbservice.NodeService{
		ID:      id,
		Kind:    kind,
		Service: name,
		Port:    port,
		Weights: &pbservice.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: entMeta,
	}
}

func pbCheck(node, svcID, svcName, status string, entMeta *pbcommon.EnterpriseMeta) *pbservice.HealthCheck {
	if entMeta == nil {
		entMeta = pbcommon.DefaultEnterpriseMeta
	}
	return &pbservice.HealthCheck{
		Node:           node,
		CheckID:        svcID + ":overall-check",
		Name:           "overall-check",
		Status:         status,
		ServiceID:      svcID,
		ServiceName:    svcName,
		EnterpriseMeta: entMeta,
	}
}

// mockSnapshotHandler is copied from server_discovery/server_test.go
type mockSnapshotHandler struct {
	mock.Mock
}

func newMockSnapshotHandler(t *testing.T) *mockSnapshotHandler {
	handler := &mockSnapshotHandler{}
	t.Cleanup(func() {
		handler.AssertExpectations(t)
	})
	return handler
}

func (m *mockSnapshotHandler) handle(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	ret := m.Called(req, buf)
	return ret.Get(0).(uint64), ret.Error(1)
}

func (m *mockSnapshotHandler) expect(token string, requestIndex uint64, eventIndex uint64, payload autopilotevents.EventPayloadReadyServers) {
	m.On("handle", stream.SubscribeRequest{
		Topic:   autopilotevents.EventTopicReadyServers,
		Subject: stream.SubjectNone,
		Token:   token,
		Index:   requestIndex,
	}, mock.Anything).Run(func(args mock.Arguments) {
		buf := args.Get(1).(stream.SnapshotAppender)
		buf.Append([]stream.Event{
			{
				Topic:   autopilotevents.EventTopicReadyServers,
				Index:   eventIndex,
				Payload: payload,
			},
		})
	}).Return(eventIndex, nil)
}
