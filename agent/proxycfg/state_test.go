package proxycfg

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
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
			state, err := newState(tt.ns, tt.token, cachetype.HealthServicesName)
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
	require.True(t, ok, "Correlation ID: %s is missing", correlationId)
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
	return genVerifyDCSpecificWatch(cachetype.CatalogServiceListName, expectedDatacenter)
}

func verifyDatacentersWatch(t testing.TB, cacheType string, request cache.Request) {
	require.Equal(t, cachetype.CatalogDatacentersName, cacheType)

	_, ok := request.(*structs.DatacentersRequest)
	require.True(t, ok)
}

func genVerifyLeafWatchWithDNSSANs(expectedService string, expectedDatacenter string, expectedDNSSANs []string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.ConnectCALeafName, cacheType)

		reqReal, ok := request.(*cachetype.ConnectCALeafRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Service)
		require.ElementsMatch(t, expectedDNSSANs, reqReal.DNSSAN)
	}
}

func genVerifyLeafWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return genVerifyLeafWatchWithDNSSANs(expectedService, expectedDatacenter, nil)
}

func genVerifyResolverWatch(expectedService, expectedDatacenter, expectedKind string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.ConfigEntriesName, cacheType)

		reqReal, ok := request.(*structs.ConfigEntryQuery)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Name)
		require.Equal(t, expectedKind, reqReal.Kind)
	}
}

func genVerifyResolvedConfigWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.ResolvedServiceConfigName, cacheType)

		reqReal, ok := request.(*structs.ServiceConfigRequest)
		require.True(t, ok)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Name)
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
		require.Equal(t, structs.DefaultEnterpriseMeta(), &reqReal.EnterpriseMeta)
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

func genVerifyGatewayServiceWatch(expectedService, expectedDatacenter string) verifyWatchRequest {
	return genVerifyServiceSpecificRequest(cachetype.GatewayServicesName, expectedService, "", expectedDatacenter, false)
}

func genVerifyConfigEntryWatch(expectedKind, expectedName, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, cacheType string, request cache.Request) {
		require.Equal(t, cachetype.ConfigEntryName, cacheType)

		reqReal, ok := request.(*structs.ConfigEntryQuery)
		require.True(t, ok)
		require.Equal(t, expectedKind, reqReal.Kind)
		require.Equal(t, expectedName, reqReal.Name)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
	}
}

func ingressConfigWatchEvent(tlsEnabled bool) cache.UpdateEvent {
	return cache.UpdateEvent{
		CorrelationID: gatewayConfigWatchID,
		Result: &structs.ConfigEntryResponse{
			Entry: &structs.IngressGatewayConfigEntry{
				TLS: structs.GatewayTLSConfig{
					Enabled: tlsEnabled,
				},
			},
		},
		Err: nil,
	}
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

	indexedRoots, issuedCert := TestCerts(t)

	rootWatchEvent := func() cache.UpdateEvent {
		return cache.UpdateEvent{
			CorrelationID: rootsWatchID,
			Result:        indexedRoots,
			Err:           nil,
		}
	}

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

		ixnMatch := TestIntentions()

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
				rootWatchEvent(),
				{
					CorrelationID: leafWatchID,
					Result:        issuedCert,
					Err:           nil,
				},
				{
					CorrelationID: intentionsWatchID,
					Result:        ixnMatch,
					Err:           nil,
				},
				{
					CorrelationID: "discovery-chain:api",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "dc1", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							}),
					},
					Err: nil,
				},
				{
					CorrelationID: "discovery-chain:api-failover-remote?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-remote", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeRemote
							}),
					},
					Err: nil,
				},
				{
					CorrelationID: "discovery-chain:api-failover-local?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-local", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeLocal
							}),
					},
					Err: nil,
				},
				{
					CorrelationID: "discovery-chain:api-failover-direct?dc=dc2",
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-direct", "default", "dc2", "trustdomain.consul", "dc1",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeNone
							}),
					},
					Err: nil,
				},
				{
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
			verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
				require.True(t, snap.Valid())
				require.True(t, snap.MeshGateway.IsEmpty())
				require.Equal(t, indexedRoots, snap.Roots)

				require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)
				require.Len(t, snap.ConnectProxy.DiscoveryChain, 5, "%+v", snap.ConnectProxy.DiscoveryChain)
				require.Len(t, snap.ConnectProxy.WatchedUpstreams, 5, "%+v", snap.ConnectProxy.WatchedUpstreams)
				require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 5, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
				require.Len(t, snap.ConnectProxy.WatchedGateways, 5, "%+v", snap.ConnectProxy.WatchedGateways)
				require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 5, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

				require.Len(t, snap.ConnectProxy.WatchedServiceChecks, 0, "%+v", snap.ConnectProxy.WatchedServiceChecks)
				require.Len(t, snap.ConnectProxy.PreparedQueryEndpoints, 0, "%+v", snap.ConnectProxy.PreparedQueryEndpoints)

				require.True(t, snap.ConnectProxy.IntentionsSet)
				require.Equal(t, ixnMatch.Matches[0], snap.ConnectProxy.Intentions)
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
			verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
				require.True(t, snap.Valid())
				require.True(t, snap.MeshGateway.IsEmpty())
				require.Equal(t, indexedRoots, snap.Roots)

				require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)
				require.Len(t, snap.ConnectProxy.DiscoveryChain, 5, "%+v", snap.ConnectProxy.DiscoveryChain)
				require.Len(t, snap.ConnectProxy.WatchedUpstreams, 5, "%+v", snap.ConnectProxy.WatchedUpstreams)
				require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 5, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
				require.Len(t, snap.ConnectProxy.WatchedGateways, 5, "%+v", snap.ConnectProxy.WatchedGateways)
				require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 5, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

				require.Len(t, snap.ConnectProxy.WatchedServiceChecks, 0, "%+v", snap.ConnectProxy.WatchedServiceChecks)
				require.Len(t, snap.ConnectProxy.PreparedQueryEndpoints, 0, "%+v", snap.ConnectProxy.PreparedQueryEndpoints)

				require.True(t, snap.ConnectProxy.IntentionsSet)
				require.Equal(t, ixnMatch.Matches[0], snap.ConnectProxy.Intentions)
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

	// Used in terminating-gateway cases to account for differences in OSS/ent implementations of ServiceID.String()
	db := structs.NewServiceName("db", nil)
	dbStr := db.String()

	billing := structs.NewServiceName("billing", nil)

	api := structs.NewServiceName("api", nil)
	apiStr := api.String()

	dbIxnMatch := &structs.IndexedIntentionMatches{
		Matches: []structs.Intentions{
			[]*structs.Intention{
				{
					ID:              "abc-123",
					SourceNS:        "default",
					SourceName:      "api",
					DestinationNS:   "default",
					DestinationName: "db",
					Action:          structs.IntentionActionAllow,
				},
			},
		},
	}

	dbConfig := &structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"protocol": "grpc",
		},
	}

	dbResolver := &structs.IndexedConfigEntries{
		Kind: structs.ServiceResolver,
		Entries: []structs.ConfigEntry{
			&structs.ServiceResolverConfigEntry{
				Name: "db",
				Kind: structs.ServiceResolver,
				Redirect: &structs.ServiceResolverRedirect{
					Service:    "db",
					Datacenter: "dc2",
				},
			},
		},
	}

	cases := map[string]testCase{
		"initial-gateway": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Address: "10.0.1.1",
				Port:    443,
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID:       genVerifyRootsWatch("dc1"),
						serviceListWatchID: genVerifyListServicesWatch("dc1"),
						datacentersWatchID: verifyDatacentersWatch,
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.ConnectProxy.IsEmpty())
					},
				},
				{
					events: []cache.UpdateEvent{
						rootWatchEvent(),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without services is valid")
						require.True(t, snap.ConnectProxy.IsEmpty())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Empty(t, snap.MeshGateway.WatchedServices)
						require.False(t, snap.MeshGateway.WatchedServicesSet)
						require.Empty(t, snap.MeshGateway.WatchedDatacenters)
						require.Empty(t, snap.MeshGateway.ServiceGroups)
						require.Empty(t, snap.MeshGateway.ServiceResolvers)
						require.Empty(t, snap.MeshGateway.GatewayGroups)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: serviceListWatchID,
							Result: &structs.IndexedServiceList{
								Services: make(structs.ServiceList, 0),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with empty service list is valid")
						require.True(t, snap.ConnectProxy.IsEmpty())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Empty(t, snap.MeshGateway.WatchedServices)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
						require.Empty(t, snap.MeshGateway.WatchedDatacenters)
						require.Empty(t, snap.MeshGateway.ServiceGroups)
						require.Empty(t, snap.MeshGateway.ServiceResolvers)
						require.Empty(t, snap.MeshGateway.GatewayGroups)
					},
				},
			},
		},
		"mesh-gateway-do-not-cancel-service-watches": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Address: "10.0.1.1",
				Port:    443,
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID:       genVerifyRootsWatch("dc1"),
						serviceListWatchID: genVerifyListServicesWatch("dc1"),
						datacentersWatchID: verifyDatacentersWatch,
					},
					events: []cache.UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: serviceListWatchID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "web"},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 1)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: serviceListWatchID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "web"},
									{Name: "api"},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 2)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: "mesh-gateway:dc4",
							Result: &structs.IndexedNodesWithGateways{
								Nodes: TestGatewayNodesDC4Hostname(t),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 2)
						require.True(t, snap.MeshGateway.WatchedServicesSet)

						expect := structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{
									ID:         "mesh-gateway-1",
									Node:       "mesh-gateway",
									Address:    "10.30.1.1",
									Datacenter: "dc4",
								},
								Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
									"10.30.1.1", 8443,
									structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
									structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
							},
							structs.CheckServiceNode{
								Node: &structs.Node{
									ID:         "mesh-gateway-2",
									Node:       "mesh-gateway",
									Address:    "10.30.1.2",
									Datacenter: "dc4",
								},
								Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
									"10.30.1.2", 8443,
									structs.ServiceAddress{Address: "10.30.1.2", Port: 8443},
									structs.ServiceAddress{Address: "456.us-west-2.elb.notaws.com", Port: 443}),
							},
						}
						require.Equal(t, snap.MeshGateway.HostnameDatacenters["dc4"], expect)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: federationStateListGatewaysWatchID,
							Result: &structs.DatacenterIndexedCheckServiceNodes{
								DatacenterNodes: map[string]structs.CheckServiceNodes{
									"dc5": TestGatewayNodesDC5Hostname(t),
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 2)
						require.True(t, snap.MeshGateway.WatchedServicesSet)

						expect := structs.CheckServiceNodes{
							structs.CheckServiceNode{
								Node: &structs.Node{
									ID:         "mesh-gateway-1",
									Node:       "mesh-gateway",
									Address:    "10.30.1.1",
									Datacenter: "dc5",
								},
								Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
									"10.30.1.1", 8443,
									structs.ServiceAddress{Address: "10.0.1.1", Port: 8443},
									structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
							},
							structs.CheckServiceNode{
								Node: &structs.Node{
									ID:         "mesh-gateway-2",
									Node:       "mesh-gateway",
									Address:    "10.30.1.2",
									Datacenter: "dc5",
								},
								Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
									"10.30.1.2", 8443,
									structs.ServiceAddress{Address: "10.30.1.2", Port: 8443},
									structs.ServiceAddress{Address: "456.us-west-2.elb.notaws.com", Port: 443}),
							},
						}
						require.Equal(t, snap.MeshGateway.HostnameDatacenters["dc5"], expect)
					},
				},
			},
		},
		"ingress-gateway": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindIngressGateway,
				ID:      "ingress-gateway",
				Service: "ingress-gateway",
				Address: "10.0.1.1",
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID:           genVerifyRootsWatch("dc1"),
						gatewayConfigWatchID:   genVerifyConfigEntryWatch(structs.IngressGateway, "ingress-gateway", "dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("ingress-gateway", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.IngressGateway.IsEmpty())
					},
				},
				{
					events: []cache.UpdateEvent{
						rootWatchEvent(),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without config entry is not valid")
						require.Equal(t, indexedRoots, snap.Roots)
					},
				},
				{
					events: []cache.UpdateEvent{
						ingressConfigWatchEvent(false),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without hosts set is not valid")
						require.True(t, snap.IngressGateway.TLSSet)
						require.False(t, snap.IngressGateway.TLSEnabled)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Gateway:  structs.NewServiceName("ingress-gateway", nil),
										Service:  structs.NewServiceName("api", nil),
										Port:     9999,
										Protocol: "http",
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without leaf is not valid")
						require.True(t, snap.IngressGateway.HostsSet)
						require.Len(t, snap.IngressGateway.Hosts, 0)
						require.Len(t, snap.IngressGateway.Upstreams, 1)
						key := IngressListenerKey{Protocol: "http", Port: 9999}
						require.Equal(t, snap.IngressGateway.Upstreams[key], structs.Upstreams{
							{
								DestinationNamespace: "default",
								DestinationName:      "api",
								LocalBindPort:        9999,
								Config: map[string]interface{}{
									"protocol": "http",
								},
							},
						})
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 1)
						require.Contains(t, snap.IngressGateway.WatchedDiscoveryChains, "api")
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						leafWatchID: genVerifyLeafWatch("ingress-gateway", "dc1"),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with root and leaf certs is valid")
						require.Equal(t, issuedCert, snap.IngressGateway.Leaf)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"discovery-chain:api": genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "api",
							EvaluateInDatacenter: "dc1",
							EvaluateInNamespace:  "default",
							Datacenter:           "dc1",
						}),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "discovery-chain:api",
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "dc1", "trustdomain.consul", "dc1", nil),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.IngressGateway.WatchedUpstreams, 1)
						require.Len(t, snap.IngressGateway.WatchedUpstreams["api"], 1)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"upstream-target:api.default.dc1:api": genVerifyServiceWatch("api", "", "dc1", true),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "upstream-target:api.default.dc1:api",
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "127.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      "api1",
											Service: "api",
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.IngressGateway.WatchedUpstreamEndpoints, 1)
						require.Contains(t, snap.IngressGateway.WatchedUpstreamEndpoints, "api")
						require.Len(t, snap.IngressGateway.WatchedUpstreamEndpoints["api"], 1)
						require.Contains(t, snap.IngressGateway.WatchedUpstreamEndpoints["api"], "api.default.dc1")
						require.Equal(t, snap.IngressGateway.WatchedUpstreamEndpoints["api"]["api.default.dc1"],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Node:    "node1",
										Address: "127.0.0.1",
									},
									Service: &structs.NodeService{
										ID:      "api1",
										Service: "api",
									},
								},
							},
						)
					},
				},
			},
		},
		"ingress-gateway-with-tls-update-upstreams": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindIngressGateway,
				ID:      "ingress-gateway",
				Service: "ingress-gateway",
				Address: "10.0.1.1",
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID:           genVerifyRootsWatch("dc1"),
						gatewayConfigWatchID:   genVerifyConfigEntryWatch(structs.IngressGateway, "ingress-gateway", "dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("ingress-gateway", "dc1"),
					},
					events: []cache.UpdateEvent{
						rootWatchEvent(),
						ingressConfigWatchEvent(true),
						{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Gateway: structs.NewServiceName("ingress-gateway", nil),
										Service: structs.NewServiceName("api", nil),
										Hosts:   []string{"test.example.com"},
										Port:    9999,
									},
								},
							},
							Err: nil,
						},
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.True(t, snap.IngressGateway.TLSSet)
						require.True(t, snap.IngressGateway.TLSEnabled)
						require.True(t, snap.IngressGateway.HostsSet)
						require.Len(t, snap.IngressGateway.Hosts, 1)
						require.Len(t, snap.IngressGateway.Upstreams, 1)
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 1)
						require.Contains(t, snap.IngressGateway.WatchedDiscoveryChains, "api")
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						leafWatchID: genVerifyLeafWatchWithDNSSANs("ingress-gateway", "dc1", []string{
							"test.example.com",
							"*.ingress.consul.",
							"*.ingress.dc1.consul.",
							"*.ingress.alt.consul.",
							"*.ingress.dc1.alt.consul.",
						}),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: gatewayServicesWatchID,
							Result:        &structs.IndexedGatewayServices{},
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.Len(t, snap.IngressGateway.Upstreams, 0)
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 0)
						require.NotContains(t, snap.IngressGateway.WatchedDiscoveryChains, "api")
					},
				},
			},
		},
		"terminating-gateway-initial": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Address: "10.0.1.1",
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID: genVerifyRootsWatch("dc1"),
						gatewayServicesWatchID: genVerifyServiceSpecificRequest(gatewayServicesWatchID,
							"terminating-gateway", "", "dc1", false),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.ConnectProxy.IsEmpty())
						require.True(t, snap.MeshGateway.IsEmpty())
						require.True(t, snap.IngressGateway.IsEmpty())
					},
				},
				{
					events: []cache.UpdateEvent{
						rootWatchEvent(),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway without services is valid")
						require.True(t, snap.ConnectProxy.IsEmpty())
						require.True(t, snap.MeshGateway.IsEmpty())
						require.True(t, snap.IngressGateway.IsEmpty())
						require.True(t, snap.TerminatingGateway.IsEmpty())
						require.Equal(t, indexedRoots, snap.Roots)
					},
				},
			},
		},
		"terminating-gateway-handle-update": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Address: "10.0.1.1",
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID: genVerifyRootsWatch("dc1"),
						gatewayServicesWatchID: genVerifyServiceSpecificRequest(gatewayServicesWatchID,
							"terminating-gateway", "", "dc1", false),
					},
					events: []cache.UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: db,
										Gateway: structs.NewServiceName("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.WatchedServices, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, db)
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: db,
										Gateway: structs.NewServiceName("terminating-gateway", nil),
									},
									{
										Service: billing,
										Gateway: structs.NewServiceName("terminating-gateway", nil),
									},
									{
										Service: api,
										Gateway: structs.NewServiceName("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.WatchedServices, 3)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, db)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, billing)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, api)

						require.Len(t, snap.TerminatingGateway.WatchedIntentions, 3)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, db)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, billing)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, api)

						require.Len(t, snap.TerminatingGateway.WatchedLeaves, 3)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, db)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, billing)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, api)

						require.Len(t, snap.TerminatingGateway.WatchedConfigs, 3)
						require.Contains(t, snap.TerminatingGateway.WatchedConfigs, db)
						require.Contains(t, snap.TerminatingGateway.WatchedConfigs, billing)
						require.Contains(t, snap.TerminatingGateway.WatchedConfigs, api)

						require.Len(t, snap.TerminatingGateway.WatchedResolvers, 3)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, db)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, billing)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, api)

						require.Len(t, snap.TerminatingGateway.GatewayServices, 3)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, db)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, billing)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, api)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"external-service:" + dbStr: genVerifyServiceWatch("db", "", "dc1", false),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "external-service:" + dbStr,
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "127.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      "db",
											Service: "db",
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.ServiceGroups, 1)
						require.Equal(t, snap.TerminatingGateway.ServiceGroups[db],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Node:    "node1",
										Address: "127.0.0.1",
									},
									Service: &structs.NodeService{
										ID:      "db",
										Service: "db",
									},
								},
							},
						)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"external-service:" + apiStr: genVerifyServiceWatch("api", "", "dc1", false),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "external-service:" + apiStr,
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "10.0.1.1",
										},
										Service: &structs.NodeService{
											ID:      "api",
											Service: "api",
											Address: "api.mydomain",
										},
									},
									{
										Node: &structs.Node{
											Node:    "node2",
											Address: "10.0.1.2",
										},
										Service: &structs.NodeService{
											ID:      "api",
											Service: "api",
											Address: "api.altdomain",
										},
									},
									{
										Node: &structs.Node{
											Node:    "node3",
											Address: "10.0.1.3",
										},
										Service: &structs.NodeService{
											ID:      "api",
											Service: "api",
											Address: "10.0.1.3",
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.ServiceGroups, 2)
						expect := structs.CheckServiceNodes{
							{
								Node: &structs.Node{
									Node:    "node1",
									Address: "10.0.1.1",
								},
								Service: &structs.NodeService{
									ID:      "api",
									Service: "api",
									Address: "api.mydomain",
								},
							},
							{
								Node: &structs.Node{
									Node:    "node2",
									Address: "10.0.1.2",
								},
								Service: &structs.NodeService{
									ID:      "api",
									Service: "api",
									Address: "api.altdomain",
								},
							},
							{
								Node: &structs.Node{
									Node:    "node3",
									Address: "10.0.1.3",
								},
								Service: &structs.NodeService{
									ID:      "api",
									Service: "api",
									Address: "10.0.1.3",
								},
							},
						}
						require.Equal(t, snap.TerminatingGateway.ServiceGroups[api], expect)

						// The instance in node3 should not be present in HostnameDatacenters because it has a valid IP
						require.ElementsMatch(t, snap.TerminatingGateway.HostnameServices[api], expect[:2])
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"service-leaf:" + dbStr: genVerifyLeafWatch("db", "dc1"),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "service-leaf:" + dbStr,
							Result:        issuedCert,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Equal(t, snap.TerminatingGateway.ServiceLeaves[db], issuedCert)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						serviceIntentionsIDPrefix + dbStr: genVerifyIntentionWatch("db", "dc1"),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: serviceIntentionsIDPrefix + dbStr,
							Result:        dbIxnMatch,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.Intentions, 1)
						dbIxn, ok := snap.TerminatingGateway.Intentions[db]
						require.True(t, ok)
						require.Equal(t, dbIxnMatch.Matches[0], dbIxn)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						serviceConfigIDPrefix + dbStr: genVerifyResolvedConfigWatch("db", "dc1"),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: serviceConfigIDPrefix + dbStr,
							Result:        dbConfig,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						require.Len(t, snap.TerminatingGateway.ServiceConfigs, 1)
						require.Equal(t, snap.TerminatingGateway.ServiceConfigs[db], dbConfig)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"service-resolver:" + dbStr: genVerifyResolverWatch("db", "dc1", structs.ServiceResolver),
					},
					events: []cache.UpdateEvent{
						{
							CorrelationID: "service-resolver:" + dbStr,
							Result:        dbResolver,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						// Finally we have everything we need
						require.Equal(t, []structs.ServiceName{db}, snap.TerminatingGateway.ValidServices())

						require.Len(t, snap.TerminatingGateway.ServiceResolversSet, 1)
						require.True(t, snap.TerminatingGateway.ServiceResolversSet[db])

						require.Len(t, snap.TerminatingGateway.ServiceResolvers, 1)
						require.Equal(t, dbResolver.Entries[0], snap.TerminatingGateway.ServiceResolvers[db])
					},
				},
				{
					events: []cache.UpdateEvent{
						{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: billing,
										Gateway: structs.NewServiceName("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.ValidServices(), 0)

						// All the watches should have been cancelled for db
						require.Len(t, snap.TerminatingGateway.WatchedServices, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, billing)

						require.Len(t, snap.TerminatingGateway.WatchedIntentions, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, billing)

						require.Len(t, snap.TerminatingGateway.WatchedLeaves, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, billing)

						require.Len(t, snap.TerminatingGateway.WatchedResolvers, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, billing)

						require.Len(t, snap.TerminatingGateway.GatewayServices, 1)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, billing)

						// There was no update event for billing's leaf/endpoints/resolvers, so length is 0
						require.Len(t, snap.TerminatingGateway.ServiceGroups, 0)
						require.Len(t, snap.TerminatingGateway.ServiceLeaves, 0)
						require.Len(t, snap.TerminatingGateway.ServiceResolvers, 0)
						require.Len(t, snap.TerminatingGateway.HostnameServices, 0)
					},
				},
			},
		},
		"connect-proxy":                    newConnectProxyCase(structs.MeshGatewayModeDefault),
		"connect-proxy-mesh-gateway-local": newConnectProxyCase(structs.MeshGatewayModeLocal),
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			state, err := newState(&tc.ns, "", cachetype.HealthServicesName)

			// verify building the initial state worked
			require.NoError(t, err)
			require.NotNil(t, state)

			// setup the test logger to use the t.Log
			state.logger = testutil.Logger(t)

			// setup a new testing cache notifier
			cn := newTestCacheNotifier()
			state.cache = cn

			// setup the local datacenter information
			state.source = &structs.QuerySource{
				Datacenter: tc.sourceDC,
			}

			state.dnsConfig = DNSConfig{
				Domain:    "consul.",
				AltDomain: "alt.consul.",
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
