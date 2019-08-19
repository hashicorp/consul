package proxycfg

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestStateChanged(t *testing.T) {
	tests := []struct {
		name   string
		ns     *structs.NodeService
		token  string
		mutate func(ns structs.NodeService, token string) (*structs.NodeService, string)
		want   bool
	}{
		{
			name: "nil node service",
			ns:   structs.TestNodeServiceProxy(t),
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				return nil, token
			},
			want: true,
		},
		{
			name: "same service",
			ns:   structs.TestNodeServiceProxy(t),
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				return &ns, token
			}, want: false,
		},
		{
			name:  "same service, different token",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				return &ns, "bar"
			},
			want: true,
		},
		{
			name:  "different service ID",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.ID = "badger"
				return &ns, token
			},
			want: true,
		},
		{
			name:  "different address",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.Address = "10.10.10.10"
				return &ns, token
			},
			want: true,
		},
		{
			name:  "different port",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.Port = 12345
				return &ns, token
			},
			want: true,
		},
		{
			name:  "different service kind",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.Kind = ""
				return &ns, token
			},
			want: true,
		},
		{
			name:  "different proxy target",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.Proxy.DestinationServiceName = "badger"
				return &ns, token
			},
			want: true,
		},
		{
			name:  "different proxy upstreams",
			ns:    structs.TestNodeServiceProxy(t),
			token: "foo",
			mutate: func(ns structs.NodeService, token string) (*structs.NodeService, string) {
				ns.Proxy.Upstreams = nil
				return &ns, token
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			state, err := newState(tt.ns, tt.token)
			require.NoError(err)
			otherNS, otherToken := tt.mutate(*tt.ns, tt.token)
			require.Equal(tt.want, state.Changed(otherNS, otherToken))
		})
	}
}

type testCacheNotifierRequest struct {
	cacheType string
	request   cache.Request
	ch        chan<- cache.UpdateEvent
}

type testCacheNotifier struct {
	lock      sync.RWMutex
	notifiers map[string]testCacheNotifierRequest
}

func newTestCacheNotifier() *testCacheNotifier {
	return &testCacheNotifier{
		notifiers: make(map[string]testCacheNotifierRequest),
	}
}

func (cn *testCacheNotifier) Notify(ctx context.Context, t string, r cache.Request, correlationId string, ch chan<- cache.UpdateEvent) error {
	cn.lock.Lock()
	cn.notifiers[correlationId] = testCacheNotifierRequest{t, r, ch}
	cn.lock.Unlock()
	return nil
}

func (cn *testCacheNotifier) getNotifierRequest(t testing.TB, correlationId string) testCacheNotifierRequest {
	cn.lock.RLock()
	req, ok := cn.notifiers[correlationId]
	cn.lock.RUnlock()
	require.True(t, ok)
	return req
}

func (cn *testCacheNotifier) getChanForCorrelationId(t testing.TB, correlationId string) chan<- cache.UpdateEvent {
	req := cn.getNotifierRequest(t, correlationId)
	require.NotNil(t, req.ch)
	return req.ch
}

func (cn *testCacheNotifier) sendNotification(t testing.TB, correlationId string, event cache.UpdateEvent) {
	cn.getChanForCorrelationId(t, correlationId) <- event
}

func (cn *testCacheNotifier) verifyWatch(t testing.TB, correlationId string) (string, cache.Request) {
	// t.Logf("Watches: %+v", cn.notifiers)
	req := cn.getNotifierRequest(t, correlationId)
	require.NotNil(t, req.ch)
	return req.cacheType, req.request
}

type verifyWatchRequest func(t testing.TB, cacheType string, request cache.Request)

func genVerifyDCSpecificWatch(expectedCacheType string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, expectedCacheType, cacheType)

		reqReal, ok := request.(*structs.DCSpecificRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
	}
}

func genVerifyRootsWatch(expectedDatacenter string) verifyWatchRequest {
	return genVerifyDCSpecificWatch(cachetype.ConnectCARootName, expectedDatacenter)
}

func genVerifyListServicesWatch(expectedDatacenter string) verifyWatchRequest {
	return genVerifyDCSpecificWatch(cachetype.CatalogListServicesName, expectedDatacenter)
}

func verifyDatacentersWatch(t testing.TB, cacheType string, request cache.Request) {
	require.Equal(t, cachetype.CatalogDatacentersName, cacheType)

	_, ok := request.(*structs.DatacentersRequest)
	require.True(t, ok)
}

func genVerifyLeafWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.ConnectCALeafName, cacheType)

		reqReal, ok := request.(*cachetype.ConnectCALeafRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Service)
	}
}

func genVerifyIntentionWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.IntentionMatchName, cacheType)

		reqReal, ok := request.(*structs.IntentionQueryRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.NotNil(t, reqReal.Match)
		require.Equal(t, structs.IntentionMatchDestination, reqReal.Match.Type)
		require.Len(t, reqReal.Match.Entries, 1)
		require.Equal(t, structs.IntentionDefaultNamespace, reqReal.Match.Entries[0].Namespace)
		require.Equal(t, expectedService, reqReal.Match.Entries[0].Name)
	}
}

func genVerifyPreparedQueryWatch(expectedName string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.PreparedQueryName, cacheType)

		reqReal, ok := request.(*structs.PreparedQueryExecuteRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedName, reqReal.QueryIDOrName)
		require.Equal(t, true, reqReal.Connect)
	}
}

func genVerifyDiscoveryChainWatch(expected *structs.DiscoveryChainRequest) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.CompiledDiscoveryChainName, cacheType)

		reqReal, ok := request.(*structs.DiscoveryChainRequest)
		require.True(t, ok)
		require.Equal(t, expected, reqReal)
	}
}

func genVerifyGatewayWatch(expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.InternalServiceDumpName, cacheType)

		reqReal, ok := request.(*structs.ServiceDumpRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.True(t, reqReal.UseServiceKind)
		require.Equal(t, structs.ServiceKindMeshGateway, reqReal.ServiceKind)
	}
}

func genVerifyServiceSpecificRequest(expectedCacheType, expectedService, expectedFilter, expectedDatacenter string, connect bool) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, expectedCacheType, cacheType)

		reqReal, ok := request.(*structs.ServiceSpecificRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.ServiceName)
		require.Equal(t, expectedFilter, reqReal.QueryOptions.Filter)
		require.Equal(t, connect, reqReal.Connect)
	}
}

func genVerifyServiceWatch(expectedService, expectedFilter, expectedDatacenter string, connect bool) verifyWatchRequest {
	return genVerifyServiceSpecificRequest(cachetype.HealthServicesName, expectedService, expectedFilter, expectedDatacenter, connect)
}

// This test is meant to exercise the various parts of the cache watching done by the state as
// well as its management of the ConfigSnapshot
//
// This test is expressly not calling Watch which in turn would execute the run function in a go
// routine. This allows the test to be fully synchronous and deterministic while still being able
// to validate the logic of most of the watching and state updating.
//
// The general strategy here is to
//
// 1. Initialize a state with a call to newState + setting some of the extra stuff like the CacheNotifier
//    We will not be using the CacheNotifier to send notifications but calling handleUpdate ourselves
// 2. Iterate through a list of verification stages performing validation and updates for each.
//    a. Ensure that the required watches are in place and validate they are correct
//    b. Process a bunch of UpdateEvents by calling handleUpdate
//    c. Validate that the ConfigSnapshot has been updated appropriately
func TestState_WatchesAndUpdates(t *testing.T) {
	t.Parallel()

	type verificationStage struct {
		requiredWatches map[string]verifyWatchRequest
		events          []cache.UpdateEvent
		verifySnapshot  func(t testing.TB, snap *ConfigSnapshot)
	}

	type testCase struct {
		// the state to operate on. the logger, source, cache,
		// ctx and cancel fields will be filled in by the test
		ns       structs.NodeService
		sourceDC string
		stages   []verificationStage
	}

	newConnectProxyCase := func(meshGatewayProxyConfigValue structs.MeshGatewayMode) testCase {
		ns := structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "web-sidecar-proxy",
			Service: "web-sidecar-proxy",
			Address: "10.0.1.1",
			Port:    443,
			Proxy: structs.ConnectProxyConfig{
				DestinationServiceName: "web",
				Upstreams: structs.Upstreams{
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypePreparedQuery,
						DestinationName: "query",
						LocalBindPort:   10001,
					},
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypeService,
						DestinationName: "api",
						LocalBindPort:   10002,
					},
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypeService,
						DestinationName: "api-failover-remote",
						Datacenter:      "dc2",
						LocalBindPort:   10003,
						MeshGateway: structs.MeshGatewayConfig{
							Mode: structs.MeshGatewayModeRemote,
						},
					},
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypeService,
						DestinationName: "api-failover-local",
						Datacenter:      "dc2",
						LocalBindPort:   10004,
						MeshGateway: structs.MeshGatewayConfig{
							Mode: structs.MeshGatewayModeLocal,
						},
					},
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypeService,
						DestinationName: "api-failover-direct",
						Datacenter:      "dc2",
						LocalBindPort:   10005,
						MeshGateway: structs.MeshGatewayConfig{
							Mode: structs.MeshGatewayModeNone,
						},
					},
					structs.Upstream{
						DestinationType: structs.UpstreamDestTypeService,
						DestinationName: "api-dc2",
						LocalBindPort:   10006,
					},
				},
			},
		}

		if meshGatewayProxyConfigValue != structs.MeshGatewayModeDefault {
			ns.Proxy.MeshGateway.Mode = meshGatewayProxyConfigValue
		}

		stage0 := verificationStage{
			requiredWatches: map[string]verifyWatchRequest{
				rootsWatchID:                    genVerifyRootsWatch("dc1"),
				leafWatchID:                     genVerifyLeafWatch("web", "dc1"),
				intentionsWatchID:               genVerifyIntentionWatch("web", "dc1"),
				"upstream:prepared_query:query": genVerifyPreparedQueryWatch("query", "dc1"),
				"discovery-chain:api": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: meshGatewayProxyConfigValue,
					},
				}),
				"discovery-chain:api-failover-remote?dc=dc2": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-remote",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
				}),
				"discovery-chain:api-failover-local?dc=dc2": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-local",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeLocal,
					},
				}),
				"discovery-chain:api-failover-direct?dc=dc2": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-direct",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeNone,
					},
				}),
				"discovery-chain:api-dc2": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-dc2",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: meshGatewayProxyConfigValue,
					},
				}),
			},
			events: []cache.UpdateEvent{
				cache.UpdateEvent{
					CorrelationID: "discovery-chain:api",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "dc1", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							}),
					},
					Err: nil,
				},
				cache.UpdateEvent{
					CorrelationID: "discovery-chain:api-failover-remote?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-remote", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeRemote
							}),
					},
					Err: nil,
				},
				cache.UpdateEvent{
					CorrelationID: "discovery-chain:api-failover-local?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-local", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeLocal
							}),
					},
					Err: nil,
				},
				cache.UpdateEvent{
					CorrelationID: "discovery-chain:api-failover-direct?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-direct", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeNone
							}),
					},
					Err: nil,
				},
				cache.UpdateEvent{
					CorrelationID: "discovery-chain:api-dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-dc2", "default", "dc1", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							},
							&structs.ServiceResolverConfigEntry{
								Kind: structs.ServiceResolver,
								Name: "api-dc2",
								Redirect: &structs.ServiceResolverRedirect{
									Service:    "api",
									Datacenter: "dc2",
								},
							},
						),
					},
					Err: nil,
				},
			},
		}

		stage1 := verificationStage{
			requiredWatches: map[string]verifyWatchRequest{
				"upstream-target:api.default.dc1:api":                                        genVerifyServiceWatch("api", "", "dc1", true),
				"upstream-target:api-failover-remote.default.dc2:api-failover-remote?dc=dc2": genVerifyServiceWatch("api-failover-remote", "", "dc2", true),
				"upstream-target:api-failover-local.default.dc2:api-failover-local?dc=dc2":   genVerifyServiceWatch("api-failover-local", "", "dc2", true),
				"upstream-target:api-failover-direct.default.dc2:api-failover-direct?dc=dc2": genVerifyServiceWatch("api-failover-direct", "", "dc2", true),
				"mesh-gateway:dc2:api-failover-remote?dc=dc2":                                genVerifyGatewayWatch("dc2"),
				"mesh-gateway:dc1:api-failover-local?dc=dc2":                                 genVerifyGatewayWatch("dc1"),
			},
		}

		if meshGatewayProxyConfigValue == structs.MeshGatewayModeLocal {
			stage1.requiredWatches["mesh-gateway:dc1:api-dc2"] = genVerifyGatewayWatch("dc1")
		}

		return testCase{
			ns:       ns,
			sourceDC: "dc1",
			stages:   []verificationStage{stage0, stage1},
		}
	}

	cases := map[string]testCase{
		"initial-gateway": testCase{
			ns: structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Address: "10.0.1.1",
				Port:    443,
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				verificationStage{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID:       genVerifyRootsWatch("dc1"),
						serviceListWatchID: genVerifyListServicesWatch("dc1"),
						datacentersWatchID: verifyDatacentersWatch,
					},
				},
			},
		},
		"connect-proxy":                    newConnectProxyCase(structs.MeshGatewayModeDefault),
		"connect-proxy-mesh-gateway-local": newConnectProxyCase(structs.MeshGatewayModeLocal),
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			state, err := newState(&tc.ns, "")

			// verify building the initial state worked
			require.NoError(t, err)
			require.NotNil(t, state)

			// setup the test logger to use the t.Log
			state.logger = testutil.TestLogger(t)

			// setup a new testing cache notifier
			cn := newTestCacheNotifier()
			state.cache = cn

			// setup the local datacenter information
			state.source = &structs.QuerySource{
				Datacenter: tc.sourceDC,
			}

			// setup the ctx as initWatches expects this to be there
			state.ctx, state.cancel = context.WithCancel(context.Background())

			// ensure the initial watch setup did not error
			require.NoError(t, state.initWatches())

			// get the initial configuration snapshot
			snap := state.initialConfigSnapshot()

			//--------------------------------------------------------------------
			//
			// All the nested subtests here are to make failures easier to
			// correlate back with the test table
			//
			//--------------------------------------------------------------------

			for idx, stage := range tc.stages {
				require.True(t, t.Run(fmt.Sprintf("stage-%d", idx), func(t *testing.T) {
					for correlationId, verifier := range stage.requiredWatches {
						require.True(t, t.Run(correlationId, func(t *testing.T) {
							// verify that the watch was initiated
							cacheType, request := cn.verifyWatch(t, correlationId)

							// run the verifier if any
							if verifier != nil {
								verifier(t, cacheType, request)
							}
						}))
					}

					// the state is not currently executing the run method in a goroutine
					// therefore we just tell it about the updates
					for eveIdx, event := range stage.events {
						require.True(t, t.Run(fmt.Sprintf("update-%d", eveIdx), func(t *testing.T) {
							require.NoError(t, state.handleUpdate(event, &snap))
						}))
					}

					// verify the snapshot
					if stage.verifySnapshot != nil {
						stage.verifySnapshot(t, &snap)
					}
				}))
			}
		})
	}
}
