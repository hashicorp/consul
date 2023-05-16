// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	apimod "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/prototest"
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
			proxyID := ProxyID{ServiceID: tt.ns.CompoundServiceID()}
			state, err := newState(proxyID, tt.ns, testSource, tt.token, stateConfig{logger: hclog.New(nil)}, rate.NewLimiter(rate.Inf, 1))
			require.NoError(t, err)
			otherNS, otherToken := tt.mutate(*tt.ns, tt.token)
			require.Equal(t, tt.want, state.Changed(otherNS, otherToken))
		})
	}
}

func recordWatches(sc *stateConfig) *watchRecorder {
	wr := newWatchRecorder()

	sc.dataSources = DataSources{
		CARoots:                         typedWatchRecorder[*structs.DCSpecificRequest]{wr},
		CompiledDiscoveryChain:          typedWatchRecorder[*structs.DiscoveryChainRequest]{wr},
		ConfigEntry:                     typedWatchRecorder[*structs.ConfigEntryQuery]{wr},
		ConfigEntryList:                 typedWatchRecorder[*structs.ConfigEntryQuery]{wr},
		Datacenters:                     typedWatchRecorder[*structs.DatacentersRequest]{wr},
		FederationStateListMeshGateways: typedWatchRecorder[*structs.DCSpecificRequest]{wr},
		GatewayServices:                 typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		ServiceGateways:                 typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		Health:                          typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		HTTPChecks:                      typedWatchRecorder[*cachetype.ServiceHTTPChecksRequest]{wr},
		Intentions:                      typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		IntentionUpstreams:              typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		IntentionUpstreamsDestination:   typedWatchRecorder[*structs.ServiceSpecificRequest]{wr},
		InternalServiceDump:             typedWatchRecorder[*structs.ServiceDumpRequest]{wr},
		LeafCertificate:                 typedWatchRecorder[*cachetype.ConnectCALeafRequest]{wr},
		PeeringList:                     typedWatchRecorder[*cachetype.PeeringListRequest]{wr},
		PeeredUpstreams:                 typedWatchRecorder[*structs.PartitionSpecificRequest]{wr},
		PreparedQuery:                   typedWatchRecorder[*structs.PreparedQueryExecuteRequest]{wr},
		ResolvedServiceConfig:           typedWatchRecorder[*structs.ServiceConfigRequest]{wr},
		ServiceList:                     typedWatchRecorder[*structs.DCSpecificRequest]{wr},
		TrustBundle:                     typedWatchRecorder[*cachetype.TrustBundleReadRequest]{wr},
		TrustBundleList:                 typedWatchRecorder[*cachetype.TrustBundleListRequest]{wr},
		ExportedPeeredServices:          typedWatchRecorder[*structs.DCSpecificRequest]{wr},
	}
	recordWatchesEnterprise(sc, wr)

	return wr
}

func newWatchRecorder() *watchRecorder {
	return &watchRecorder{
		watches: make(map[string]any),
	}
}

type watchRecorder struct {
	mu      sync.Mutex
	watches map[string]any
}

func (r *watchRecorder) record(correlationID string, req any) {
	r.mu.Lock()
	r.watches[correlationID] = req
	r.mu.Unlock()
}

func (r *watchRecorder) verify(t *testing.T, correlationID string, verifyFn verifyWatchRequest) {
	t.Helper()

	r.mu.Lock()
	req, ok := r.watches[correlationID]
	r.mu.Unlock()

	require.True(t, ok, "No such watch for Correlation ID: %q", correlationID)

	if verifyFn != nil {
		verifyFn(t, req)
	}
}

type typedWatchRecorder[ReqType any] struct {
	recorder *watchRecorder
}

func (r typedWatchRecorder[ReqType]) Notify(_ context.Context, req ReqType, correlationID string, _ chan<- UpdateEvent) error {
	r.recorder.record(correlationID, req)
	return nil
}

type verifyWatchRequest func(t testing.TB, request any)

func genVerifyDCSpecificWatch(expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.DCSpecificRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
	}
}

func verifyDatacentersWatch(t testing.TB, request any) {
	_, ok := request.(*structs.DatacentersRequest)
	require.True(t, ok)
}

func genVerifyTrustBundleReadWatch(peer string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*cachetype.TrustBundleReadRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, peer, reqReal.Request.Name)
	}
}

func genVerifyLeafWatchWithDNSSANs(expectedService string, expectedDatacenter string, expectedDNSSANs []string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*cachetype.ConnectCALeafRequest)
		reqReal.Token = aclToken
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Service)
		require.ElementsMatch(t, expectedDNSSANs, reqReal.DNSSAN)
	}
}

func genVerifyLeafWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return genVerifyLeafWatchWithDNSSANs(expectedService, expectedDatacenter, nil)
}

func genVerifyTrustBundleListWatch(service string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*cachetype.TrustBundleListRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, service, reqReal.Request.ServiceName)
	}
}

func genVerifyTrustBundleListWatchForMeshGateway(partition string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*cachetype.TrustBundleListRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, string(structs.ServiceKindMeshGateway), reqReal.Request.Kind)
		require.True(t, acl.EqualPartitions(partition, reqReal.Request.Partition), "%q != %q", partition, reqReal.Request.Partition)
		require.Equal(t, aclToken, reqReal.Token)
		require.NotEmpty(t, reqReal.Request.ServiceName)
	}
}

func genVerifyPeeringListWatchForMeshGateway() verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*cachetype.PeeringListRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, acl.WildcardPartitionName, reqReal.Request.Partition)
	}
}

func genVerifyResolverWatch(expectedService, expectedDatacenter, expectedKind string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ConfigEntryQuery)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Name)
		require.Equal(t, expectedKind, reqReal.Kind)
	}
}

func genVerifyResolvedConfigWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ServiceConfigRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.Name)
	}
}

func genVerifyIntentionWatch(expectedService string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ServiceSpecificRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedService, reqReal.ServiceName)
	}
}

func genVerifyPreparedQueryWatch(expectedName string, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.PreparedQueryExecuteRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedName, reqReal.QueryIDOrName)
		require.Equal(t, true, reqReal.Connect)
	}
}

func genVerifyDiscoveryChainWatch(expected *structs.DiscoveryChainRequest) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.DiscoveryChainRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expected, reqReal)
	}
}

func genVerifyMeshConfigWatch(expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ConfigEntryQuery)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, structs.MeshConfigMesh, reqReal.Name)
		require.Equal(t, structs.MeshConfig, reqReal.Kind)
	}
}

func genVerifyGatewayWatch(expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ServiceDumpRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.True(t, reqReal.UseServiceKind)
		require.Equal(t, structs.ServiceKindMeshGateway, reqReal.ServiceKind)
		require.Equal(t, structs.DefaultEnterpriseMetaInDefaultPartition(), &reqReal.EnterpriseMeta)
	}
}

func genVerifyServiceSpecificRequest(expectedService, expectedFilter, expectedDatacenter string, connect bool) verifyWatchRequest {
	return genVerifyServiceSpecificPeeredRequest(expectedService, expectedFilter, expectedDatacenter, "", connect)
}

func genVerifyServiceSpecificPeeredRequest(expectedService, expectedFilter, expectedDatacenter, expectedPeer string, connect bool) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ServiceSpecificRequest)
		reqReal.QueryOptions = structs.QueryOptions{
			Token: aclToken,
		}
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedPeer, reqReal.PeerName)
		require.Equal(t, expectedService, reqReal.ServiceName)
		require.Equal(t, expectedFilter, reqReal.QueryOptions.Filter)
		require.Equal(t, connect, reqReal.Connect)
	}
}

func genVerifyPartitionSpecificRequest(expectedPartition, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.PartitionSpecificRequest)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
		require.Equal(t, expectedPartition, reqReal.PartitionOrDefault())
	}
}

func genVerifyGatewayServiceWatch(expectedService, expectedDatacenter string) verifyWatchRequest {
	return genVerifyServiceSpecificRequest(expectedService, "", expectedDatacenter, false)
}

func genVerifyConfigEntryWatch(expectedKind, expectedName, expectedDatacenter string) verifyWatchRequest {
	return func(t testing.TB, request any) {
		reqReal, ok := request.(*structs.ConfigEntryQuery)
		require.True(t, ok)
		require.Equal(t, aclToken, reqReal.Token)
		require.Equal(t, expectedKind, reqReal.Kind)
		require.Equal(t, expectedName, reqReal.Name)
		require.Equal(t, expectedDatacenter, reqReal.Datacenter)
	}
}

func ingressConfigWatchEvent(gwTLS bool, mixedTLS bool) UpdateEvent {
	e := &structs.IngressGatewayConfigEntry{
		TLS: structs.GatewayTLSConfig{
			Enabled: gwTLS,
		},
	}

	if mixedTLS {
		// Add two listeners one with and one without connect TLS enabled
		e.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "tcp",
				TLS:      &structs.GatewayTLSConfig{Enabled: true},
			},
			{
				Port:     9090,
				Protocol: "tcp",
				TLS:      nil,
			},
		}
	}

	return UpdateEvent{
		CorrelationID: gatewayConfigWatchID,
		Result: &structs.ConfigEntryResponse{
			Entry: e,
		},
		Err: nil,
	}
}

func upstreamIDForDC2(uid UpstreamID) UpstreamID {
	uid.Datacenter = "dc2"
	return uid
}

func discoChainSetWithEntries(entries ...structs.ConfigEntry) *configentry.DiscoveryChainSet {
	set := configentry.NewDiscoveryChainSet()
	set.AddEntries(entries...)
	return set
}

// This test is meant to exercise the various parts of the cache watching done by the state as
// well as its management of the ConfigSnapshot
//
// This test is expressly not calling Watch which in turn would execute the run function in a go
// routine. This allows the test to be fully synchronous and deterministic while still being able
// to validate the logic of most of the watching and state updating.
//
// The general strategy here is to:
//
//  1. Initialize a state with a call to newState + setting some of the extra stuff like the CacheNotifier
//     We will not be using the CacheNotifier to send notifications but calling handleUpdate ourselves
//  2. Iterate through a list of verification stages performing validation and updates for each.
//     a. Ensure that the required watches are in place and validate they are correct
//     b. Process a bunch of UpdateEvents by calling handleUpdate
//     c. Validate that the ConfigSnapshot has been updated appropriately
func TestState_WatchesAndUpdates(t *testing.T) {
	t.Parallel()

	indexedRoots, issuedCert := TestCerts(t)
	peerTrustBundles := TestPeerTrustBundles(t)

	// Used to account for differences in OSS/ent implementations of ServiceID.String()
	var (
		db           = structs.NewServiceName("db", nil)
		billing      = structs.NewServiceName("billing", nil)
		api          = structs.NewServiceName("api", nil)
		apiA         = structs.NewServiceName("api-a", nil)
		hcpCollector = structs.NewServiceName(apimod.HCPMetricsCollectorName, nil)

		apiUID          = NewUpstreamIDFromServiceName(api)
		dbUID           = NewUpstreamIDFromServiceName(db)
		pqUID           = UpstreamIDFromString("prepared_query:query")
		extApiUID       = NewUpstreamIDFromServiceName(apiA)
		extDBUID        = NewUpstreamIDFromServiceName(db)
		hcpCollectorUID = NewUpstreamIDFromServiceName(hcpCollector)
	)
	// TODO(peering): NewUpstreamIDFromServiceName should take a PeerName
	extApiUID.Peer = "peer-a"
	extDBUID.Peer = "peer-a"

	const peerTrustDomain = "1c053652-8512-4373-90cf-5a7f6263a994.consul"

	rootWatchEvent := func() UpdateEvent {
		return UpdateEvent{
			CorrelationID: rootsWatchID,
			Result:        indexedRoots,
			Err:           nil,
		}
	}

	type verificationStage struct {
		requiredWatches map[string]verifyWatchRequest
		events          []UpdateEvent
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
						DestinationName: "api-failover-to-peer",
						LocalBindPort:   10007,
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
			for i := range ns.Proxy.Upstreams {
				if u := &ns.Proxy.Upstreams[i]; u.MeshGateway.Mode == structs.MeshGatewayModeDefault {
					u.MeshGateway.Mode = meshGatewayProxyConfigValue
				}
			}
		}

		ixnMatch := TestIntentions()

		stage0 := verificationStage{
			requiredWatches: map[string]verifyWatchRequest{
				intentionsWatchID: genVerifyIntentionWatch("web", "dc1"),
				meshConfigEntryID: genVerifyMeshConfigWatch("dc1"),
				fmt.Sprintf("discovery-chain:%s", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: meshGatewayProxyConfigValue,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				fmt.Sprintf("discovery-chain:%s-failover-remote?dc=dc2", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-remote",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				fmt.Sprintf("discovery-chain:%s-failover-local?dc=dc2", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-local",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeLocal,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				fmt.Sprintf("discovery-chain:%s-failover-direct?dc=dc2", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-direct",
					EvaluateInDatacenter: "dc2",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeNone,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				fmt.Sprintf("discovery-chain:%s-failover-to-peer", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-failover-to-peer",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: meshGatewayProxyConfigValue,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				fmt.Sprintf("discovery-chain:%s-dc2", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
					Name:                 "api-dc2",
					EvaluateInDatacenter: "dc1",
					EvaluateInNamespace:  "default",
					EvaluateInPartition:  "default",
					Datacenter:           "dc1",
					OverrideMeshGateway: structs.MeshGatewayConfig{
						Mode: meshGatewayProxyConfigValue,
					},
					QueryOptions: structs.QueryOptions{
						Token: aclToken,
					},
				}),
				"upstream:" + pqUID.String(): genVerifyPreparedQueryWatch("query", "dc1"),
				rootsWatchID:                 genVerifyDCSpecificWatch("dc1"),
				leafWatchID:                  genVerifyLeafWatch("web", "dc1"),
			},
			events: []UpdateEvent{
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
					CorrelationID: meshConfigEntryID,
					Result:        &structs.ConfigEntryResponse{},
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "default", "dc1", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							}, nil),
					},
					Err: nil,
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s-failover-remote?dc=dc2", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-remote", "default", "default", "dc2", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeRemote
							}, nil),
					},
					Err: nil,
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s-failover-local?dc=dc2", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-local", "default", "default", "dc2", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeLocal
							}, nil),
					},
					Err: nil,
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s-failover-direct?dc=dc2", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-direct", "default", "default", "dc2", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = structs.MeshGatewayModeNone
							}, nil),
					},
					Err: nil,
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s-dc2", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-dc2", "default", "default", "dc1", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							}, discoChainSetWithEntries(&structs.ServiceResolverConfigEntry{
								Kind: structs.ServiceResolver,
								Name: "api-dc2",
								Redirect: &structs.ServiceResolverRedirect{
									Service:    "api",
									Datacenter: "dc2",
								},
							})),
					},
					Err: nil,
				},
				{
					CorrelationID: fmt.Sprintf("discovery-chain:%s-failover-to-peer", apiUID.String()),
					Result: &structs.DiscoveryChainResponse{
						Chain: discoverychain.TestCompileConfigEntries(t, "api-failover-to-peer", "default", "default", "dc1", "trustdomain.consul",
							func(req *discoverychain.CompileRequest) {
								req.OverrideMeshGateway.Mode = meshGatewayProxyConfigValue
							}, discoChainSetWithEntries(&structs.ServiceResolverConfigEntry{
								Kind: structs.ServiceResolver,
								Name: "api-failover-to-peer",
								Failover: map[string]structs.ServiceResolverFailover{
									"*": {
										Targets: []structs.ServiceResolverFailoverTarget{
											{Peer: "cluster-01"},
										},
									},
								},
							})),
					},
					Err: nil,
				},
			},
			verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
				require.True(t, snap.Valid())
				require.True(t, snap.MeshGateway.isEmpty())
				require.Equal(t, indexedRoots, snap.Roots)

				require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)
				require.Len(t, snap.ConnectProxy.DiscoveryChain, 6, "%+v", snap.ConnectProxy.DiscoveryChain)
				require.Len(t, snap.ConnectProxy.WatchedUpstreams, 6, "%+v", snap.ConnectProxy.WatchedUpstreams)
				require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 6, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
				require.Len(t, snap.ConnectProxy.WatchedGateways, 6, "%+v", snap.ConnectProxy.WatchedGateways)
				require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 6, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

				require.Len(t, snap.ConnectProxy.WatchedServiceChecks, 0, "%+v", snap.ConnectProxy.WatchedServiceChecks)
				require.Len(t, snap.ConnectProxy.PreparedQueryEndpoints, 0, "%+v", snap.ConnectProxy.PreparedQueryEndpoints)

				require.Equal(t, 1, snap.ConnectProxy.ConfigSnapshotUpstreams.PeerUpstreamEndpoints.Len())
				require.Equal(t, 1, snap.ConnectProxy.ConfigSnapshotUpstreams.UpstreamPeerTrustBundles.Len())

				require.True(t, snap.ConnectProxy.IntentionsSet)
				require.Equal(t, ixnMatch, snap.ConnectProxy.Intentions)
				require.True(t, snap.ConnectProxy.MeshConfigSet)

				if meshGatewayProxyConfigValue == structs.MeshGatewayModeLocal {
					require.True(t, snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched("dc1"))
					_, ok := snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
					require.False(t, ok)
				}
			},
		}

		stage1 := verificationStage{
			requiredWatches: map[string]verifyWatchRequest{
				fmt.Sprintf("upstream-target:api.default.default.dc1:%s", apiUID.String()):                                        genVerifyServiceSpecificRequest("api", "", "dc1", true),
				fmt.Sprintf("upstream-target:api-failover-remote.default.default.dc2:%s-failover-remote?dc=dc2", apiUID.String()): genVerifyServiceSpecificRequest("api-failover-remote", "", "dc2", true),
				fmt.Sprintf("upstream-target:api-failover-local.default.default.dc2:%s-failover-local?dc=dc2", apiUID.String()):   genVerifyServiceSpecificRequest("api-failover-local", "", "dc2", true),
				fmt.Sprintf("upstream-target:api-failover-direct.default.default.dc2:%s-failover-direct?dc=dc2", apiUID.String()): genVerifyServiceSpecificRequest("api-failover-direct", "", "dc2", true),
				upstreamPeerWatchIDPrefix + fmt.Sprintf("%s-failover-to-peer?peer=cluster-01", apiUID.String()):                   genVerifyServiceSpecificPeeredRequest("api-failover-to-peer", "", "dc1", "cluster-01", true),
				fmt.Sprintf("mesh-gateway:dc2:%s-failover-remote?dc=dc2", apiUID.String()):                                        genVerifyGatewayWatch("dc2"),
				fmt.Sprintf("mesh-gateway:dc1:%s-failover-local?dc=dc2", apiUID.String()):                                         genVerifyGatewayWatch("dc1"),
			},
			verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
				require.True(t, snap.Valid())
				require.True(t, snap.MeshGateway.isEmpty())
				require.Equal(t, indexedRoots, snap.Roots)

				require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)
				require.Len(t, snap.ConnectProxy.DiscoveryChain, 6, "%+v", snap.ConnectProxy.DiscoveryChain)
				require.Len(t, snap.ConnectProxy.WatchedUpstreams, 6, "%+v", snap.ConnectProxy.WatchedUpstreams)
				require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 6, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
				require.Len(t, snap.ConnectProxy.WatchedGateways, 6, "%+v", snap.ConnectProxy.WatchedGateways)
				require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 6, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

				require.Len(t, snap.ConnectProxy.WatchedServiceChecks, 0, "%+v", snap.ConnectProxy.WatchedServiceChecks)
				require.Len(t, snap.ConnectProxy.PreparedQueryEndpoints, 0, "%+v", snap.ConnectProxy.PreparedQueryEndpoints)

				require.Equal(t, 1, snap.ConnectProxy.ConfigSnapshotUpstreams.PeerUpstreamEndpoints.Len())
				require.Equal(t, 1, snap.ConnectProxy.ConfigSnapshotUpstreams.UpstreamPeerTrustBundles.Len())

				require.True(t, snap.ConnectProxy.IntentionsSet)
				require.Equal(t, ixnMatch, snap.ConnectProxy.Intentions)

				if meshGatewayProxyConfigValue == structs.MeshGatewayModeLocal {
					require.True(t, snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched("dc1"))
					_, ok := snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
					require.False(t, ok)
				}
			},
		}

		if meshGatewayProxyConfigValue == structs.MeshGatewayModeLocal {
			stage1.requiredWatches[fmt.Sprintf("mesh-gateway:dc1:%s-dc2", apiUID.String())] = genVerifyGatewayWatch("dc1")
		}

		return testCase{
			ns:       ns,
			sourceDC: "dc1",
			stages:   []verificationStage{stage0, stage1},
		}
	}

	dbIxnMatch := structs.SimplifiedIntentions{
		{
			ID:              "abc-123",
			SourceNS:        "default",
			SourceName:      "api",
			DestinationNS:   "default",
			DestinationName: "db",
			Action:          structs.IntentionActionAllow,
		},
	}

	dbConfig := &structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"protocol": "grpc",
		},
	}

	dbResolver := &structs.ConfigEntryResponse{
		Entry: &structs.ServiceResolverConfigEntry{
			Name: "db",
			Kind: structs.ServiceResolver,
			Redirect: &structs.ServiceResolverRedirect{
				Service:    "db",
				Datacenter: "dc2",
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
				Meta: map[string]string{
					structs.MetaWANFederationKey: "1",
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						datacentersWatchID:         verifyDatacentersWatch,
						serviceListWatchID:         genVerifyDCSpecificWatch("dc1"),
						rootsWatchID:               genVerifyDCSpecificWatch("dc1"),
						exportedServiceListWatchID: genVerifyDCSpecificWatch("dc1"),
						meshConfigEntryID:          genVerifyMeshConfigWatch("dc1"),
						peeringTrustBundlesWatchID: genVerifyTrustBundleListWatchForMeshGateway(""),
						consulServerListWatchID:    genVerifyServiceSpecificPeeredRequest(structs.ConsulServiceName, "", "dc1", "", false),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.ConnectProxy.isEmpty())
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: exportedServiceListWatchID,
							Result: &structs.IndexedExportedServiceList{
								Services: nil,
							},
						},
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result: &pbpeering.TrustBundleListByServiceResponse{
								Bundles: nil,
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without services is valid")
						require.True(t, snap.ConnectProxy.isEmpty())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Empty(t, snap.MeshGateway.WatchedServices)
						require.False(t, snap.MeshGateway.WatchedServicesSet)
						require.Empty(t, snap.MeshGateway.WatchedGateways)
						require.Empty(t, snap.MeshGateway.ServiceGroups)
						require.Empty(t, snap.MeshGateway.ServiceResolvers)
						require.Empty(t, snap.MeshGateway.GatewayGroups)
						require.Empty(t, snap.MeshGateway.WatchedPeeringServices)
						require.Empty(t, snap.MeshGateway.WatchedPeers)
						require.Empty(t, snap.MeshGateway.PeeringServices)
					},
				},
				{
					events: []UpdateEvent{
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
						require.True(t, snap.ConnectProxy.isEmpty())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Empty(t, snap.MeshGateway.WatchedServices)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
						require.Empty(t, snap.MeshGateway.WatchedGateways)
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
						datacentersWatchID:         verifyDatacentersWatch,
						serviceListWatchID:         genVerifyDCSpecificWatch("dc1"),
						rootsWatchID:               genVerifyDCSpecificWatch("dc1"),
						exportedServiceListWatchID: genVerifyDCSpecificWatch("dc1"),
						meshConfigEntryID:          genVerifyMeshConfigWatch("dc1"),
						peeringTrustBundlesWatchID: genVerifyTrustBundleListWatchForMeshGateway(""),
					},
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: exportedServiceListWatchID,
							Result: &structs.IndexedExportedServiceList{
								Services: nil,
							},
						},
						{
							CorrelationID: serviceListWatchID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "web"},
								},
							},
						},
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result:        peerTrustBundles,
						},
						{
							CorrelationID: peeringServiceListWatchID + "peer-a",
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "service-1"},
								},
							},
						},
						{
							CorrelationID: peeringServiceListWatchID + "peer-b",
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "service-10"},
								},
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 1)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
						require.Len(t, snap.MeshGateway.WatchedPeers, 2)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices, 2)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices["peer-a"], 1)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices["peer-b"], 1)
					},
				},
				{
					events: []UpdateEvent{
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
						{
							CorrelationID: peeringServiceListWatchID + "peer-a",
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									{Name: "service-1"},
									{Name: "service-2"},
									{Name: "service-3"},
								},
							},
							Err: nil,
						},
						{
							CorrelationID: peeringServiceListWatchID + "peer-b",
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.MeshGateway.WatchedServices, 2)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
						require.Len(t, snap.MeshGateway.WatchedPeers, 2)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices, 2)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices["peer-a"], 3)
						require.Len(t, snap.MeshGateway.WatchedPeeringServices["peer-b"], 0)
					},
				},
				{
					events: []UpdateEvent{
						{
							CorrelationID: "mesh-gateway:dc4",
							Result: &structs.IndexedCheckServiceNodes{
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
					events: []UpdateEvent{
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
		"mesh-gateway-peering-control-plane": {
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
						datacentersWatchID:         verifyDatacentersWatch,
						serviceListWatchID:         genVerifyDCSpecificWatch("dc1"),
						rootsWatchID:               genVerifyDCSpecificWatch("dc1"),
						exportedServiceListWatchID: genVerifyDCSpecificWatch("dc1"),
						meshConfigEntryID:          genVerifyMeshConfigWatch("dc1"),
						peeringTrustBundlesWatchID: genVerifyTrustBundleListWatchForMeshGateway(""),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									Peering: &structs.PeeringMeshConfig{
										PeerThroughMeshGateways: true,
									},
								},
							},
						},
						{
							CorrelationID: exportedServiceListWatchID,
							Result: &structs.IndexedExportedServiceList{
								Services: nil,
							},
						},
						{
							CorrelationID: serviceListWatchID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{},
							},
						},
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result: &pbpeering.TrustBundleListByServiceResponse{
								Bundles: nil,
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Equal(t, indexedRoots, snap.Roots)
						require.True(t, snap.MeshGateway.WatchedServicesSet)
						require.True(t, snap.MeshGateway.PeeringTrustBundlesSet)
						require.True(t, snap.MeshGateway.MeshConfigSet)

						require.True(t, snap.Valid(), "gateway without services is valid")
						require.True(t, snap.ConnectProxy.isEmpty())
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						consulServerListWatchID: genVerifyServiceSpecificPeeredRequest(structs.ConsulServiceName, "", "dc1", "", false),
						peerServersWatchID:      genVerifyPeeringListWatchForMeshGateway(),
					},
					events: []UpdateEvent{
						{
							CorrelationID: consulServerListWatchID,
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "node1",
											Address:    "127.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      structs.ConsulServiceID,
											Service: structs.ConsulServiceName,
										},
									},
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "replica1",
											Address:    "127.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      structs.ConsulServiceID,
											Service: structs.ConsulServiceName,
											Meta:    map[string]string{"read_replica": "true"},
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.NotNil(t, snap.MeshGateway.PeerServersWatchCancel)

						servers, ok := snap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
						require.True(t, ok)

						expect := structs.CheckServiceNodes{
							{
								Node: &structs.Node{
									Datacenter: "dc1",
									Node:       "node1",
									Address:    "127.0.0.1",
								},
								Service: &structs.NodeService{
									ID:      structs.ConsulServiceID,
									Service: structs.ConsulServiceName,
								},
							},
							{
								Node: &structs.Node{
									Datacenter: "dc1",
									Node:       "replica1",
									Address:    "127.0.0.1",
								},
								Service: &structs.NodeService{
									ID:      structs.ConsulServiceID,
									Service: structs.ConsulServiceName,
									Meta:    map[string]string{"read_replica": "true"},
								},
							},
						}
						require.Equal(t, expect, servers)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						consulServerListWatchID: genVerifyServiceSpecificPeeredRequest(structs.ConsulServiceName, "", "dc1", "", false),
						peerServersWatchID:      genVerifyPeeringListWatchForMeshGateway(),
					},
					events: []UpdateEvent{
						{
							CorrelationID: peerServersWatchID,
							Result: &pbpeering.PeeringListResponse{
								Peerings: []*pbpeering.Peering{
									{
										Name:                "peer-bar",
										PeerServerName:      "server.bar.peering.bar-domain",
										PeerServerAddresses: []string{"1.2.3.4:8443", "2.3.4.5:8443"},
										ModifyIndex:         30,
									},
									{
										Name:                "peer-broken",
										PeerServerName:      "server.broken.peering.broken-domain",
										PeerServerAddresses: []string{},
										ModifyIndex:         45,
									},
									{
										Name:                "peer-foo-zap",
										PeerServerName:      "server.foo.peering.foo-domain",
										PeerServerAddresses: []string{"elb.now-aws.com:8443", "1.2.3.4:8443"},
										ModifyIndex:         20,
									},
									{
										Name:                "peer-foo-zip",
										PeerServerName:      "server.foo.peering.foo-domain",
										PeerServerAddresses: []string{"1.2.3.4:8443", "2.3.4.5:8443"},
										ModifyIndex:         12,
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.NotNil(t, snap.MeshGateway.PeerServersWatchCancel)

						expect := map[string]PeerServersValue{
							"server.foo.peering.foo-domain": {
								Addresses: []structs.ServiceAddress{{Address: "elb.now-aws.com", Port: 8443}},
								UseCDS:    true,
								Index:     20,
							},
							"server.bar.peering.bar-domain": {
								Addresses: []structs.ServiceAddress{{Address: "1.2.3.4", Port: 8443}, {Address: "2.3.4.5", Port: 8443}},
								UseCDS:    false,
								Index:     30,
							},
						}
						require.Equal(t, expect, snap.MeshGateway.PeerServers)
					},
				},
				{
					events: []UpdateEvent{
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									Peering: &structs.PeeringMeshConfig{
										PeerThroughMeshGateways: false,
									},
								},
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.NotNil(t, snap.MeshConfig())

						require.Nil(t, snap.MeshGateway.PeerServersWatchCancel)
						require.Empty(t, snap.MeshGateway.PeerServers)

						require.False(t, snap.MeshGateway.WatchedLocalServers.IsWatched(structs.ConsulServiceName))
						servers, ok := snap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
						require.False(t, ok)
						require.Empty(t, servers)
					},
				},
				{
					events: []UpdateEvent{
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: nil,
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.Nil(t, snap.MeshConfig())

						require.False(t, snap.MeshGateway.WatchedLocalServers.IsWatched(structs.ConsulServiceName))
						servers, ok := snap.MeshGateway.WatchedLocalServers.Get(structs.ConsulServiceName)
						require.False(t, ok)
						require.Empty(t, servers)
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
						meshConfigEntryID:      genVerifyMeshConfigWatch("dc1"),
						gatewayConfigWatchID:   genVerifyConfigEntryWatch(structs.IngressGateway, "ingress-gateway", "dc1"),
						rootsWatchID:           genVerifyDCSpecificWatch("dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("ingress-gateway", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.IngressGateway.isEmpty())
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without config entry is not valid")
						require.Equal(t, indexedRoots, snap.Roots)
					},
				},
				{
					events: []UpdateEvent{
						ingressConfigWatchEvent(false, false),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without hosts set is not valid")
						require.True(t, snap.IngressGateway.GatewayConfigLoaded)
						require.False(t, snap.IngressGateway.TLSConfig.Enabled)
					},
				},
				{
					events: []UpdateEvent{
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
								DestinationPartition: "default",
								DestinationName:      "api",
								LocalBindPort:        9999,
								Config: map[string]interface{}{
									"protocol": "http",
								},
							},
						})
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 1)
						require.Contains(t, snap.IngressGateway.WatchedDiscoveryChains, apiUID)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						leafWatchID: genVerifyLeafWatch("ingress-gateway", "dc1"),
					},
					events: []UpdateEvent{
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
						"discovery-chain:" + apiUID.String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "api",
							EvaluateInDatacenter: "dc1",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "discovery-chain:" + apiUID.String(),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "default", "dc1", "trustdomain.consul", nil, nil),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.IngressGateway.WatchedUpstreams, 1)
						require.Len(t, snap.IngressGateway.WatchedUpstreams[apiUID], 1)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"upstream-target:api.default.default.dc1:" + apiUID.String(): genVerifyServiceSpecificRequest("api", "", "dc1", true),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "upstream-target:api.default.default.dc1:" + apiUID.String(),
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
						require.Contains(t, snap.IngressGateway.WatchedUpstreamEndpoints, apiUID)
						require.Len(t, snap.IngressGateway.WatchedUpstreamEndpoints[apiUID], 1)
						require.Contains(t, snap.IngressGateway.WatchedUpstreamEndpoints[apiUID], "api.default.default.dc1")
						require.Equal(t, snap.IngressGateway.WatchedUpstreamEndpoints[apiUID]["api.default.default.dc1"],
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
						meshConfigEntryID:      genVerifyMeshConfigWatch("dc1"),
						gatewayConfigWatchID:   genVerifyConfigEntryWatch(structs.IngressGateway, "ingress-gateway", "dc1"),
						rootsWatchID:           genVerifyDCSpecificWatch("dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("ingress-gateway", "dc1"),
					},
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						ingressConfigWatchEvent(true, false),
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
						require.True(t, snap.IngressGateway.GatewayConfigLoaded)
						require.True(t, snap.IngressGateway.TLSConfig.Enabled)
						require.True(t, snap.IngressGateway.HostsSet)
						require.Len(t, snap.IngressGateway.Hosts, 1)
						require.Len(t, snap.IngressGateway.Upstreams, 1)
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 1)
						require.Contains(t, snap.IngressGateway.WatchedDiscoveryChains, apiUID)
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
					events: []UpdateEvent{
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
		"ingress-gateway-with-mixed-tls": {
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
						meshConfigEntryID:      genVerifyMeshConfigWatch("dc1"),
						gatewayConfigWatchID:   genVerifyConfigEntryWatch(structs.IngressGateway, "ingress-gateway", "dc1"),
						rootsWatchID:           genVerifyDCSpecificWatch("dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("ingress-gateway", "dc1"),
					},
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						ingressConfigWatchEvent(false, true),
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
						require.True(t, snap.IngressGateway.GatewayConfigLoaded)
						// GW level TLS should be disabled
						require.False(t, snap.IngressGateway.TLSConfig.Enabled)
						// Mixed listener TLS
						l, ok := snap.IngressGateway.Listeners[IngressListenerKey{"tcp", 8080}]
						require.True(t, ok)
						require.NotNil(t, l.TLS)
						require.True(t, l.TLS.Enabled)
						l, ok = snap.IngressGateway.Listeners[IngressListenerKey{"tcp", 9090}]
						require.True(t, ok)
						require.Nil(t, l.TLS)

						require.True(t, snap.IngressGateway.HostsSet)
						require.Len(t, snap.IngressGateway.Hosts, 1)
						require.Len(t, snap.IngressGateway.Upstreams, 1)
						require.Len(t, snap.IngressGateway.WatchedDiscoveryChains, 1)
						require.Contains(t, snap.IngressGateway.WatchedDiscoveryChains, apiUID)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						// This is the real point of this test - ensure we still generate
						// the right DNS SANs for the whole gateway even when only a subset
						// of listeners have TLS enabled.
						leafWatchID: genVerifyLeafWatchWithDNSSANs("ingress-gateway", "dc1", []string{
							"test.example.com",
							"*.ingress.consul.",
							"*.ingress.dc1.consul.",
							"*.ingress.alt.consul.",
							"*.ingress.dc1.alt.consul.",
						}),
					},
					events: []UpdateEvent{
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
						meshConfigEntryID:      genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:           genVerifyDCSpecificWatch("dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("terminating-gateway", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "gateway without root is not valid")
						require.True(t, snap.ConnectProxy.isEmpty())
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway without services is valid")
						require.True(t, snap.ConnectProxy.isEmpty())
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.False(t, snap.TerminatingGateway.isEmpty())
						require.Nil(t, snap.TerminatingGateway.MeshConfig)
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
						meshConfigEntryID:      genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:           genVerifyDCSpecificWatch("dc1"),
						gatewayServicesWatchID: genVerifyGatewayServiceWatch("terminating-gateway", "dc1"),
					},
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
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
					events: []UpdateEvent{
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
						"external-service:" + db.String(): genVerifyServiceSpecificRequest("db", "", "dc1", false),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "external-service:" + db.String(),
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
						"external-service:" + api.String(): genVerifyServiceSpecificRequest("api", "", "dc1", false),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "external-service:" + api.String(),
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
						"service-leaf:" + db.String(): genVerifyLeafWatch("db", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "service-leaf:" + db.String(),
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
						serviceIntentionsIDPrefix + db.String(): genVerifyIntentionWatch("db", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: serviceIntentionsIDPrefix + db.String(),
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
						require.Equal(t, dbIxnMatch, dbIxn)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						serviceConfigIDPrefix + db.String(): genVerifyResolvedConfigWatch("db", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: serviceConfigIDPrefix + db.String(),
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
						"service-resolver:" + db.String(): genVerifyResolverWatch("db", "dc1", structs.ServiceResolver),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "service-resolver:" + db.String(),
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
						require.Equal(t, dbResolver.Entry, snap.TerminatingGateway.ServiceResolvers[db])
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"service-resolver:" + db.String(): genVerifyResolverWatch("db", "dc1", structs.ServiceResolver),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "service-resolver:" + db.String(),
							Result: &structs.ConfigEntryResponse{
								Entry: nil,
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						// Finally ensure we cleaned up the resolver
						require.Equal(t, []structs.ServiceName{db}, snap.TerminatingGateway.ValidServices())

						require.False(t, snap.TerminatingGateway.ServiceResolversSet[db])
						require.Nil(t, snap.TerminatingGateway.ServiceResolvers[db])
					},
				},
				{
					events: []UpdateEvent{
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
		"transparent-proxy-initial": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Address: "10.0.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					Mode:                   structs.ProxyModeTransparent,
					Upstreams: structs.Upstreams{
						{
							DestinationName: "db",
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						meshConfigEntryID:               genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "proxy without roots/leaf/intentions is not valid")
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())

						require.False(t, snap.ConnectProxy.isEmpty())
						expectUpstreams := map[UpstreamID]*structs.Upstream{
							dbUID: {
								DestinationName:      "db",
								DestinationNamespace: structs.IntentionDefaultNamespace,
								DestinationPartition: structs.IntentionDefaultNamespace,
							},
						}
						require.Equal(t, expectUpstreams, snap.ConnectProxy.UpstreamConfig)
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: nil, // no explicit config
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.Equal(t, TestIntentions(), snap.ConnectProxy.Intentions)
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.Nil(t, snap.ConnectProxy.MeshConfig)
					},
				},
			},
		},
		"transparent-proxy-handle-update": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Address: "10.0.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					Mode:                   structs.ProxyModeTransparent,
					Upstreams: structs.Upstreams{
						{
							CentrallyConfigured:  true,
							DestinationName:      structs.WildcardSpecifier,
							DestinationNamespace: structs.WildcardSpecifier,
							Config: map[string]interface{}{
								"connect_timeout_ms": 6000,
							},
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				// Empty on initialization
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						meshConfigEntryID:               genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "proxy without roots/leaf/intentions is not valid")
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())

						// Centrally configured upstream defaults should be stored so that upstreams from intentions can inherit them
						require.Len(t, snap.ConnectProxy.UpstreamConfig, 1)

						wc := structs.NewServiceName(structs.WildcardSpecifier, structs.WildcardEnterpriseMetaInDefaultPartition())
						wcUID := NewUpstreamIDFromServiceName(wc)
						require.Contains(t, snap.ConnectProxy.UpstreamConfig, wcUID)
					},
				},
				// Valid snapshot after roots, leaf, and intentions
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									TransparentProxy: structs.TransparentProxyMeshConfig{},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.Equal(t, TestIntentions(), snap.ConnectProxy.Intentions)
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.NotNil(t, snap.ConnectProxy.MeshConfig)
					},
				},
				// Receiving an intention should lead to spinning up a discovery chain watch
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: intentionUpstreamsID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									db,
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "should still be valid")

						require.Equal(t, map[UpstreamID]struct{}{dbUID: {}}, snap.ConnectProxy.IntentionUpstreams)

						// Should start watch for db's chain
						require.Contains(t, snap.ConnectProxy.WatchedDiscoveryChains, dbUID)

						// Should not have results yet
						require.Empty(t, snap.ConnectProxy.DiscoveryChain)

						require.Len(t, snap.ConnectProxy.UpstreamConfig, 2)
						cfg, ok := snap.ConnectProxy.UpstreamConfig[dbUID]
						require.True(t, ok)

						// Upstream config should have been inherited from defaults under wildcard key
						require.Equal(t, cfg.Config["connect_timeout_ms"], 6000)
					},
				},
				// Discovery chain updates should be stored
				{
					requiredWatches: map[string]verifyWatchRequest{
						"discovery-chain:" + dbUID.String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                   "db",
							EvaluateInDatacenter:   "dc1",
							EvaluateInNamespace:    "default",
							EvaluateInPartition:    "default",
							Datacenter:             "dc1",
							OverrideConnectTimeout: 6 * time.Second,
							OverrideMeshGateway:    structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "discovery-chain:" + dbUID.String(),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(
									t, "db", "default", "default", "dc1", "trustdomain.consul", nil,
									discoChainSetWithEntries(&structs.ServiceConfigEntry{
										Kind: structs.ServiceDefaults,
										Name: "db",
										TransparentProxy: structs.TransparentProxyConfig{
											DialedDirectly: true,
										},
									}),
								),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams[dbUID], 1)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						"upstream-target:db.default.default.dc1:" + dbUID.String(): genVerifyServiceSpecificRequest("db", "", "dc1", true),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "node1",
											Address:    "10.0.0.1",
										},
										Service: &structs.NodeService{
											Kind:    structs.ServiceKindConnectProxy,
											ID:      "db-sidecar-proxy",
											Service: "db-sidecar-proxy",
											Address: "10.10.10.10",
											TaggedAddresses: map[string]structs.ServiceAddress{
												structs.TaggedAddressWAN:     {Address: "17.5.7.8"},
												structs.TaggedAddressWANIPv6: {Address: "2607:f0d0:1002:51::4"},
											},
											Proxy: structs.ConnectProxyConfig{
												DestinationServiceName: "db",
												TransparentProxy: structs.TransparentProxyConfig{
													DialedDirectly: false, // This is set true by the service-defaults entry above.
												},
											},
											RaftIndex: structs.RaftIndex{
												ModifyIndex: 12,
											},
										},
									},
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "node2",
											Address:    "10.0.0.2",
											RaftIndex: structs.RaftIndex{
												ModifyIndex: 21,
											},
										},
										Service: &structs.NodeService{
											Kind:    structs.ServiceKindConnectProxy,
											ID:      "db-sidecar-proxy2",
											Service: "db-sidecar-proxy",
											Proxy: structs.ConnectProxyConfig{
												DestinationServiceName: "db",
												TransparentProxy: structs.TransparentProxyConfig{
													DialedDirectly: true,
												},
											},
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, dbUID)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID], 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID], "db.default.default.dc1")
						require.Equal(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID]["db.default.default.dc1"],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Datacenter: "dc1",
										Node:       "node1",
										Address:    "10.0.0.1",
									},
									Service: &structs.NodeService{
										Kind:    structs.ServiceKindConnectProxy,
										ID:      "db-sidecar-proxy",
										Service: "db-sidecar-proxy",
										Address: "10.10.10.10",
										TaggedAddresses: map[string]structs.ServiceAddress{
											structs.TaggedAddressWAN:     {Address: "17.5.7.8"},
											structs.TaggedAddressWANIPv6: {Address: "2607:f0d0:1002:51::4"},
										},
										Proxy: structs.ConnectProxyConfig{
											DestinationServiceName: "db",
											TransparentProxy: structs.TransparentProxyConfig{
												DialedDirectly: false,
											},
										},
										RaftIndex: structs.RaftIndex{
											ModifyIndex: 12,
										},
									},
								},
								{
									Node: &structs.Node{
										Datacenter: "dc1",
										Node:       "node2",
										Address:    "10.0.0.2",
										RaftIndex: structs.RaftIndex{
											ModifyIndex: 21,
										},
									},
									Service: &structs.NodeService{
										Kind:    structs.ServiceKindConnectProxy,
										ID:      "db-sidecar-proxy2",
										Service: "db-sidecar-proxy",
										Proxy: structs.ConnectProxyConfig{
											DestinationServiceName: "db",
											TransparentProxy: structs.TransparentProxyConfig{
												DialedDirectly: true,
											},
										},
									},
								},
							},
						)
						// The LAN service address is used below because transparent proxying
						// does not support querying service nodes in other DCs, and the WAN address
						// should not be used in DC-local calls.
						require.Equal(t, snap.ConnectProxy.PassthroughUpstreams, map[UpstreamID]map[string]map[string]struct{}{
							dbUID: {
								"db.default.default.dc1": map[string]struct{}{
									"10.10.10.10": {},
									"10.0.0.2":    {},
								},
							},
						})
						require.Equal(t, snap.ConnectProxy.PassthroughIndices, map[string]indexedTarget{
							"10.0.0.2": {
								upstreamID: dbUID,
								targetID:   "db.default.default.dc1",
								idx:        21,
							},
							"10.10.10.10": {
								upstreamID: dbUID,
								targetID:   "db.default.default.dc1",
								idx:        12,
							},
						})
					},
				},
				// Discovery chain updates should be stored
				{
					requiredWatches: map[string]verifyWatchRequest{
						"discovery-chain:" + dbUID.String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                   "db",
							EvaluateInDatacenter:   "dc1",
							EvaluateInNamespace:    "default",
							EvaluateInPartition:    "default",
							Datacenter:             "dc1",
							OverrideConnectTimeout: 6 * time.Second,
							OverrideMeshGateway:    structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "discovery-chain:" + dbUID.String(),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", "trustdomain.consul", nil,
									discoChainSetWithEntries(&structs.ServiceResolverConfigEntry{
										Kind: structs.ServiceResolver,
										Name: "db",
										Redirect: &structs.ServiceResolverRedirect{
											Service: "mysql",
										},
									})),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams[dbUID], 2)

						// In transparent mode we watch the upstream's endpoints even if the upstream is not a target of its chain.
						// This will happen in cases like redirects.
						require.Contains(t, snap.ConnectProxy.WatchedUpstreams[dbUID], "db.default.default.dc1")
						require.Contains(t, snap.ConnectProxy.WatchedUpstreams[dbUID], "mysql.default.default.dc1")
					},
				},
				{
					// Receive a new upstream target event without proxy1.
					events: []UpdateEvent{
						{
							CorrelationID: "upstream-target:db.default.default.dc1:" + dbUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "node2",
											Address:    "10.0.0.2",
											RaftIndex: structs.RaftIndex{
												ModifyIndex: 21,
											},
										},
										Service: &structs.NodeService{
											Kind:    structs.ServiceKindConnectProxy,
											ID:      "db-sidecar-proxy2",
											Service: "db-sidecar-proxy",
											Proxy: structs.ConnectProxyConfig{
												DestinationServiceName: "db",
												TransparentProxy: structs.TransparentProxyConfig{
													DialedDirectly: true,
												},
											},
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, dbUID)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID], 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID], "db.default.default.dc1")

						// THe endpoint and passthrough address for proxy1 should be gone.
						require.Equal(t, snap.ConnectProxy.WatchedUpstreamEndpoints[dbUID]["db.default.default.dc1"],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Datacenter: "dc1",
										Node:       "node2",
										Address:    "10.0.0.2",
										RaftIndex: structs.RaftIndex{
											ModifyIndex: 21,
										},
									},
									Service: &structs.NodeService{
										Kind:    structs.ServiceKindConnectProxy,
										ID:      "db-sidecar-proxy2",
										Service: "db-sidecar-proxy",
										Proxy: structs.ConnectProxyConfig{
											DestinationServiceName: "db",
											TransparentProxy: structs.TransparentProxyConfig{
												DialedDirectly: true,
											},
										},
									},
								},
							},
						)
						require.Equal(t, snap.ConnectProxy.PassthroughUpstreams, map[UpstreamID]map[string]map[string]struct{}{
							dbUID: {
								"db.default.default.dc1": map[string]struct{}{
									"10.0.0.2": {},
								},
							},
						})
						require.Equal(t, snap.ConnectProxy.PassthroughIndices, map[string]indexedTarget{
							"10.0.0.2": {
								upstreamID: dbUID,
								targetID:   "db.default.default.dc1",
								idx:        21,
							},
						})
					},
				},
				{
					// Receive a new upstream target event with a conflicting passthrough address
					events: []UpdateEvent{
						{
							CorrelationID: "upstream-target:api.default.default.dc1:" + apiUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Datacenter: "dc1",
											Node:       "node2",
										},
										Service: &structs.NodeService{
											Kind:    structs.ServiceKindConnectProxy,
											ID:      "api-sidecar-proxy",
											Service: "api-sidecar-proxy",
											Address: "10.0.0.2",
											Proxy: structs.ConnectProxyConfig{
												DestinationServiceName: "api",
												TransparentProxy: structs.TransparentProxyConfig{
													DialedDirectly: true,
												},
											},
											RaftIndex: structs.RaftIndex{
												ModifyIndex: 32,
											},
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 2)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, apiUID)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID], 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID], "api.default.default.dc1")

						// THe endpoint and passthrough address for proxy1 should be gone.
						require.Equal(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID]["api.default.default.dc1"],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Datacenter: "dc1",
										Node:       "node2",
									},
									Service: &structs.NodeService{
										Kind:    structs.ServiceKindConnectProxy,
										ID:      "api-sidecar-proxy",
										Service: "api-sidecar-proxy",
										Address: "10.0.0.2",
										Proxy: structs.ConnectProxyConfig{
											DestinationServiceName: "api",
											TransparentProxy: structs.TransparentProxyConfig{
												DialedDirectly: true,
											},
										},
										RaftIndex: structs.RaftIndex{
											ModifyIndex: 32,
										},
									},
								},
							},
						)
						require.Equal(t, snap.ConnectProxy.PassthroughUpstreams, map[UpstreamID]map[string]map[string]struct{}{
							apiUID: {
								// This target has a higher index so the old passthrough address should be discarded.
								"api.default.default.dc1": map[string]struct{}{
									"10.0.0.2": {},
								},
							},
						})
						require.Equal(t, snap.ConnectProxy.PassthroughIndices, map[string]indexedTarget{
							"10.0.0.2": {
								upstreamID: apiUID,
								targetID:   "api.default.default.dc1",
								idx:        32,
							},
						})
					},
				},
				{
					// Event with no nodes should clean up addrs
					events: []UpdateEvent{
						{
							CorrelationID: "upstream-target:api.default.default.dc1:" + apiUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 2)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, apiUID)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID], 1)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID], "api.default.default.dc1")

						// The endpoint and passthrough address for proxy1 should be gone.
						require.Empty(t, snap.ConnectProxy.WatchedUpstreamEndpoints[apiUID]["api.default.default.dc1"])
						require.Empty(t, snap.ConnectProxy.PassthroughUpstreams[apiUID]["api.default.default.dc1"])
						require.Empty(t, snap.ConnectProxy.PassthroughIndices)
					},
				},
				{
					// Empty list of upstreams should clean up map keys
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: intentionUpstreamsID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "should still be valid")

						// Empty intention upstreams leads to cancelling all associated watches
						require.Empty(t, snap.ConnectProxy.WatchedDiscoveryChains)
						require.Empty(t, snap.ConnectProxy.WatchedUpstreams)
						require.Empty(t, snap.ConnectProxy.WatchedUpstreamEndpoints)
						require.Empty(t, snap.ConnectProxy.WatchedGateways)
						require.Empty(t, snap.ConnectProxy.WatchedGatewayEndpoints)
						require.Empty(t, snap.ConnectProxy.DiscoveryChain)
						require.Empty(t, snap.ConnectProxy.IntentionUpstreams)
						require.Empty(t, snap.ConnectProxy.PassthroughUpstreams)
						require.Empty(t, snap.ConnectProxy.PassthroughIndices)
					},
				},
			},
		},
		"transparent-proxy-handle-update-destination": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Address: "10.0.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					Mode:                   structs.ProxyModeTransparent,
					Upstreams: structs.Upstreams{
						{
							CentrallyConfigured:  true,
							DestinationName:      structs.WildcardSpecifier,
							DestinationNamespace: structs.WildcardSpecifier,
							Config: map[string]interface{}{
								"connect_timeout_ms": 6000,
							},
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				// Empty on initialization
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						meshConfigEntryID:               genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "proxy without roots/leaf/intentions is not valid")
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())

						// Centrally configured upstream defaults should be stored so that upstreams from intentions can inherit them
						require.Len(t, snap.ConnectProxy.UpstreamConfig, 1)

						wc := structs.NewServiceName(structs.WildcardSpecifier, structs.WildcardEnterpriseMetaInDefaultPartition())
						wcUID := NewUpstreamIDFromServiceName(wc)
						require.Contains(t, snap.ConnectProxy.UpstreamConfig, wcUID)
					},
				},
				// Valid snapshot after roots, leaf, and intentions
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									TransparentProxy: structs.TransparentProxyMeshConfig{},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.Equal(t, TestIntentions(), snap.ConnectProxy.Intentions)
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.NotNil(t, snap.ConnectProxy.MeshConfig)
					},
				},
				// Receiving an intention should lead to spinning up a DestinationConfigEntryID
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						rootsWatchID:                    genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                     genVerifyLeafWatch("api", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: intentionUpstreamsDestinationID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{
									db,
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "should still be valid")

						// Watches have a key allocated even if the value is not set
						require.Equal(t, 1, snap.ConnectProxy.DestinationsUpstream.Len())
					},
				},
				// DestinationConfigEntryID updates should be stored
				{
					requiredWatches: map[string]verifyWatchRequest{
						DestinationConfigEntryID + dbUID.String(): genVerifyConfigEntryWatch(structs.ServiceDefaults, db.Name, "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: DestinationConfigEntryID + dbUID.String(),
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.ServiceConfigEntry{Name: "db", Destination: &structs.DestinationConfig{}},
							},
							Err: nil,
						},
						{
							CorrelationID: DestinationGatewayID + dbUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:       "foo",
											Partition:  api.PartitionOrDefault(),
											Datacenter: "dc1",
										},
										Service: &structs.NodeService{
											Service: "gtwy1",
											TaggedAddresses: map[string]structs.ServiceAddress{
												structs.ServiceGatewayVirtualIPTag(structs.ServiceName{Name: "db", EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition()}): {Address: "172.0.0.1", Port: 443},
											},
										},
										Checks: structs.HealthChecks{},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "should still be valid")
						require.Equal(t, 1, snap.ConnectProxy.DestinationsUpstream.Len())
						require.Equal(t, 1, snap.ConnectProxy.DestinationGateways.Len())
						snap.ConnectProxy.DestinationsUpstream.ForEachKey(func(uid UpstreamID) bool {
							_, ok := snap.ConnectProxy.DestinationsUpstream.Get(uid)
							require.True(t, ok)
							return true
						})
						dbDest, ok := snap.ConnectProxy.DestinationsUpstream.Get(dbUID)
						require.True(t, ok)
						require.Equal(t, structs.ServiceConfigEntry{Name: "db", Destination: &structs.DestinationConfig{}}, *dbDest)
					},
				},
			},
		},
		// Receiving an empty upstreams from Intentions list shouldn't delete explicit upstream watches
		"transparent-proxy-handle-update-explicit-cross-dc": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Address: "10.0.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					Mode:                   structs.ProxyModeTransparent,
					Upstreams: structs.Upstreams{
						{
							CentrallyConfigured:  true,
							DestinationName:      structs.WildcardSpecifier,
							DestinationNamespace: structs.WildcardSpecifier,
							Config: map[string]interface{}{
								"connect_timeout_ms": 6000,
							},
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
						{
							DestinationName:      db.Name,
							DestinationNamespace: db.NamespaceOrDefault(),
							Datacenter:           "dc2",
							LocalBindPort:        8080,
							MeshGateway:          structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				// Empty on initialization
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						meshConfigEntryID:               genVerifyMeshConfigWatch("dc1"),
						"discovery-chain:" + upstreamIDForDC2(dbUID).String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "db",
							EvaluateInDatacenter: "dc2",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							OverrideMeshGateway:  structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
						rootsWatchID: genVerifyDCSpecificWatch("dc1"),
						leafWatchID:  genVerifyLeafWatch("api", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "proxy without roots/leaf/intentions is not valid")
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())

						// Centrally configured upstream defaults should be stored so that upstreams from intentions can inherit them
						require.Len(t, snap.ConnectProxy.UpstreamConfig, 2)

						wc := structs.NewServiceName(structs.WildcardSpecifier, structs.WildcardEnterpriseMetaInDefaultPartition())
						wcUID := NewUpstreamIDFromServiceName(wc)
						require.Contains(t, snap.ConnectProxy.UpstreamConfig, wcUID)
						require.Contains(t, snap.ConnectProxy.UpstreamConfig, upstreamIDForDC2(dbUID))
					},
				},
				// Valid snapshot after roots, leaf, and intentions
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									TransparentProxy: structs.TransparentProxyMeshConfig{},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.Equal(t, TestIntentions(), snap.ConnectProxy.Intentions)
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.NotNil(t, snap.ConnectProxy.MeshConfig)
					},
				},
				// Discovery chain updates should be stored
				{
					requiredWatches: map[string]verifyWatchRequest{
						"discovery-chain:" + upstreamIDForDC2(dbUID).String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "db",
							EvaluateInDatacenter: "dc2",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							OverrideMeshGateway:  structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
					},
					events: []UpdateEvent{
						{
							CorrelationID: "discovery-chain:" + upstreamIDForDC2(dbUID).String(),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc2", "trustdomain.consul",
									func(req *discoverychain.CompileRequest) {
										req.OverrideMeshGateway.Mode = structs.MeshGatewayModeLocal
									}, nil),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.ConnectProxy.WatchedGateways, 1)
						require.Len(t, snap.ConnectProxy.WatchedGateways[upstreamIDForDC2(dbUID)], 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams[upstreamIDForDC2(dbUID)], 1)
					},
				},
				// Empty list of upstreams should only clean up implicit upstreams. The explicit upstream db should not
				// be deleted from the snapshot.
				{
					requiredWatches: map[string]verifyWatchRequest{
						intentionsWatchID:               genVerifyIntentionWatch("api", "dc1"),
						intentionUpstreamsID:            genVerifyServiceSpecificRequest("api", "", "dc1", false),
						intentionUpstreamsDestinationID: genVerifyServiceSpecificRequest("api", "", "dc1", false),
						"discovery-chain:" + upstreamIDForDC2(dbUID).String(): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "db",
							EvaluateInDatacenter: "dc2",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							OverrideMeshGateway:  structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
						rootsWatchID: genVerifyDCSpecificWatch("dc1"),
						leafWatchID:  genVerifyLeafWatch("api", "dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: intentionUpstreamsID,
							Result: &structs.IndexedServiceList{
								Services: structs.ServiceList{},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "should still be valid")
						require.Empty(t, snap.ConnectProxy.IntentionUpstreams)

						// Explicit upstream discovery chain watches don't get stored in these maps because they don't
						// get canceled unless the proxy registration is modified.
						require.Empty(t, snap.ConnectProxy.WatchedDiscoveryChains)

						// Explicit upstreams should not be deleted when the empty update event happens since that is
						// for intention upstreams.
						require.Len(t, snap.ConnectProxy.DiscoveryChain, 1)
						require.Contains(t, snap.ConnectProxy.DiscoveryChain, upstreamIDForDC2(dbUID))
						require.Len(t, snap.ConnectProxy.WatchedGateways, 1)
						require.Len(t, snap.ConnectProxy.WatchedGateways[upstreamIDForDC2(dbUID)], 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams[upstreamIDForDC2(dbUID)], 1)
					},
				},
			},
		},
		"transparent-proxy-initial-with-peers": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Address: "10.0.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeLocal,
					},
					Mode: structs.ProxyModeTransparent,
					Upstreams: structs.Upstreams{
						{
							DestinationName: "api-a",
							DestinationPeer: "peer-a",
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						peeringTrustBundlesWatchID: genVerifyTrustBundleListWatch("api"),
						peeredUpstreamsID:          genVerifyPartitionSpecificRequest(acl.DefaultEnterpriseMeta().PartitionOrDefault(), "dc1"),
						meshConfigEntryID:          genVerifyMeshConfigWatch("dc1"),
						rootsWatchID:               genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                genVerifyLeafWatch("api", "dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "proxy without roots/leaf/intentions is not valid")
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())

						require.False(t, snap.ConnectProxy.isEmpty())

						// This is explicitly defined from proxy config
						expectUpstreams := map[UpstreamID]*structs.Upstream{
							extApiUID: {
								DestinationName:      "api-a",
								DestinationNamespace: structs.IntentionDefaultNamespace,
								DestinationPartition: structs.IntentionDefaultNamespace,
								DestinationPeer:      "peer-a",
							},
						}
						require.Equal(t, expectUpstreams, snap.ConnectProxy.UpstreamConfig)
					},
				},
				{
					// Initial events
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result:        peerTrustBundles,
						},
						{
							CorrelationID: peeredUpstreamsID,
							Result: &structs.IndexedPeeredServiceList{
								Services: []structs.PeeredServiceName{
									{
										ServiceName: apiA,
										Peer:        "peer-a",
									},
									{
										// This service is dynamic (not from static config)
										ServiceName: db,
										Peer:        "peer-a",
									},
								},
							},
						},
						{
							CorrelationID: meshConfigEntryID,
							Result: &structs.ConfigEntryResponse{
								Entry: nil, // no explicit config
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.Nil(t, snap.ConnectProxy.MeshConfig)

						// Check PeeredUpstream is populated
						expect := map[UpstreamID]struct{}{
							extDBUID:  {},
							extApiUID: {},
						}
						require.Equal(t, expect, snap.ConnectProxy.PeeredUpstreams)

						require.True(t, snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(extApiUID))
						_, ok := snap.ConnectProxy.PeerUpstreamEndpoints.Get(extApiUID)
						require.False(t, ok, "expected initialized but empty PeerUpstreamEndpoint")

						require.True(t, snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(extDBUID))
						_, ok = snap.ConnectProxy.PeerUpstreamEndpoints.Get(extDBUID)
						require.False(t, ok, "expected initialized but empty PeerUpstreamEndpoint")

						require.True(t, snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched("peer-a"))
						_, ok = snap.ConnectProxy.UpstreamPeerTrustBundles.Get("peer-a")
						require.False(t, ok, "expected initialized but empty PeerTrustBundle")
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						upstreamPeerWatchIDPrefix + extApiUID.String(): genVerifyServiceSpecificPeeredRequest("api-a", "", "dc1", "peer-a", true),
						upstreamPeerWatchIDPrefix + extDBUID.String():  genVerifyServiceSpecificPeeredRequest("db", "", "dc1", "peer-a", true),
						peerTrustBundleIDPrefix + "peer-a":             genVerifyTrustBundleReadWatch("peer-a"),
						"mesh-gateway:dc1":                             genVerifyGatewayWatch("dc1"),
					},
					events: []UpdateEvent{
						{
							CorrelationID: peerTrustBundleIDPrefix + "peer-a",
							Result: &pbpeering.TrustBundleReadResponse{
								Bundle: peerTrustBundles.Bundles[0],
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.Leaf())
						require.True(t, snap.MeshGateway.isEmpty())
						require.True(t, snap.IngressGateway.isEmpty())
						require.True(t, snap.TerminatingGateway.isEmpty())
						require.True(t, snap.ConnectProxy.MeshConfigSet)
						require.Nil(t, snap.ConnectProxy.MeshConfig)

						// Check PeeredUpstream is populated
						expect := map[UpstreamID]struct{}{
							extDBUID:  {},
							extApiUID: {},
						}
						require.Equal(t, expect, snap.ConnectProxy.PeeredUpstreams)

						// Expect two entries (DB and api-a)
						require.Equal(t, 2, snap.ConnectProxy.PeerUpstreamEndpoints.Len())

						// db does not have endpoints yet
						ep, _ := snap.ConnectProxy.PeerUpstreamEndpoints.Get(extDBUID)
						require.Nil(t, ep)

						// Expect a trust bundle
						ptb, ok := snap.ConnectProxy.UpstreamPeerTrustBundles.Get("peer-a")
						require.True(t, ok)
						prototest.AssertDeepEqual(t, peerTrustBundles.Bundles[0], ptb)

						// Ensure that maps for non-peering endpoints are not populated.
						require.Empty(t, snap.ConnectProxy.WatchedUpstreamEndpoints[extDBUID])
						require.Empty(t, snap.ConnectProxy.PassthroughUpstreams[extDBUID])
						require.Empty(t, snap.ConnectProxy.PassthroughIndices)

						// Local gateway is  watched but there are no endpoints
						require.True(t, snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched("dc1"))
						_, ok = snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
						require.False(t, ok)
					},
				},
				{
					events: []UpdateEvent{
						{
							CorrelationID: upstreamPeerWatchIDPrefix + extDBUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:     "node1",
											Address:  "127.0.0.1",
											PeerName: "peer-a",
										},
										Service: &structs.NodeService{
											ID:       "db",
											Service:  "db",
											PeerName: "peer-a",
											Connect:  structs.ServiceConnect{},
										},
									},
								},
							},
						},
						{
							CorrelationID: "mesh-gateway:dc1",
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "10.1.2.3",
										},
										Service: structs.TestNodeServiceMeshGateway(t),
									},
								},
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")

						// Check PeeredUpstream is populated
						expect := map[UpstreamID]struct{}{
							extApiUID: {},
							extDBUID:  {},
						}
						require.Equal(t, expect, snap.ConnectProxy.PeeredUpstreams)

						// Expect two entries (api-a, db)
						require.Equal(t, 2, snap.ConnectProxy.PeerUpstreamEndpoints.Len())

						// db has an endpoint now
						ep, _ := snap.ConnectProxy.PeerUpstreamEndpoints.Get(extDBUID)
						require.NotNil(t, ep)
						require.Len(t, ep, 1)

						require.Equal(t, 1, snap.ConnectProxy.WatchedLocalGWEndpoints.Len())

						gwEp, _ := snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
						require.NotNil(t, gwEp)
						require.Len(t, gwEp, 1)

						// Expect a trust bundle
						ptb, ok := snap.ConnectProxy.UpstreamPeerTrustBundles.Get("peer-a")
						require.True(t, ok)
						prototest.AssertDeepEqual(t, peerTrustBundles.Bundles[0], ptb)

						// Ensure that maps for non-peering endpoints are not populated.
						require.Empty(t, snap.ConnectProxy.WatchedUpstreamEndpoints[extDBUID])
						require.Empty(t, snap.ConnectProxy.PassthroughUpstreams[extDBUID])
						require.Empty(t, snap.ConnectProxy.PassthroughIndices)
					},
				},
				{
					// Empty list of peered upstreams should clean up map keys
					events: []UpdateEvent{
						{
							CorrelationID: peeredUpstreamsID,
							Result: &structs.IndexedPeeredServiceList{
								Services: []structs.PeeredServiceName{},
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "proxy with roots/leaf/intentions is valid")

						require.Empty(t, snap.ConnectProxy.PeeredUpstreams)

						// db endpoint should have been cleaned up
						require.False(t, snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(extDBUID))

						// Expect only api-a endpoint
						require.Equal(t, 1, snap.ConnectProxy.PeerUpstreamEndpoints.Len())
						require.Equal(t, 1, snap.ConnectProxy.UpstreamPeerTrustBundles.Len())
					},
				},
			},
		},
		"connect-proxy":                    newConnectProxyCase(structs.MeshGatewayModeDefault),
		"connect-proxy-mesh-gateway-local": newConnectProxyCase(structs.MeshGatewayModeLocal),
		"connect-proxy-with-peers": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-sidecar-proxy",
				Service: "web-sidecar-proxy",
				Address: "10.0.1.1",
				Port:    443,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationType: structs.UpstreamDestTypeService,
							DestinationName: "api",
							LocalBindPort:   10000,
						},
						structs.Upstream{
							DestinationType: structs.UpstreamDestTypeService,
							DestinationName: "api-a",
							DestinationPeer: "peer-a",
							LocalBindPort:   10001,
							MeshGateway: structs.MeshGatewayConfig{
								Mode: structs.MeshGatewayModeLocal,
							},
						},
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				// First evaluate peered upstream
				{
					requiredWatches: map[string]verifyWatchRequest{
						fmt.Sprintf("discovery-chain:%s", apiUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 "api",
							EvaluateInDatacenter: "dc1",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
						rootsWatchID:                                   genVerifyDCSpecificWatch("dc1"),
						leafWatchID:                                    genVerifyLeafWatch("web", "dc1"),
						peeringTrustBundlesWatchID:                     genVerifyTrustBundleListWatch("web"),
						peerTrustBundleIDPrefix + "peer-a":             genVerifyTrustBundleReadWatch("peer-a"),
						upstreamPeerWatchIDPrefix + extApiUID.String(): genVerifyServiceSpecificPeeredRequest("api-a", "", "dc1", "peer-a", true),
						"mesh-gateway:dc1":                             genVerifyGatewayWatch("dc1"),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "should not be valid")
						require.True(t, snap.MeshGateway.isEmpty())

						require.Len(t, snap.ConnectProxy.DiscoveryChain, 0, "%+v", snap.ConnectProxy.DiscoveryChain)
						require.Len(t, snap.ConnectProxy.WatchedDiscoveryChains, 0, "%+v", snap.ConnectProxy.WatchedDiscoveryChains)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 0, "%+v", snap.ConnectProxy.WatchedUpstreams)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 0, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
						require.Len(t, snap.ConnectProxy.WatchedGateways, 0, "%+v", snap.ConnectProxy.WatchedGateways)
						require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 0, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

						// watch initialized
						require.True(t, snap.ConnectProxy.UpstreamPeerTrustBundles.IsWatched("peer-a"))
						_, ok := snap.ConnectProxy.UpstreamPeerTrustBundles.Get("peer-a")
						require.False(t, ok) // but no data

						// watch initialized
						require.True(t, snap.ConnectProxy.PeerUpstreamEndpoints.IsWatched(extApiUID))
						_, ok = snap.ConnectProxy.PeerUpstreamEndpoints.Get(extApiUID)
						require.False(t, ok) // but no data

						// Local gateway is  watched but there are no endpoints
						require.True(t, snap.ConnectProxy.WatchedLocalGWEndpoints.IsWatched("dc1"))
						_, ok = snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
						require.False(t, ok)

						require.Len(t, snap.ConnectProxy.WatchedServiceChecks, 0, "%+v", snap.ConnectProxy.WatchedServiceChecks)
						require.Len(t, snap.ConnectProxy.PreparedQueryEndpoints, 0, "%+v", snap.ConnectProxy.PreparedQueryEndpoints)
						require.Len(t, snap.ConnectProxy.InboundPeerTrustBundles, 0, "%+v", snap.ConnectProxy.InboundPeerTrustBundles)
						require.False(t, snap.ConnectProxy.InboundPeerTrustBundlesSet)
					},
				},
				{
					// This time add the events
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result:        peerTrustBundles,
						},
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						{
							CorrelationID: fmt.Sprintf("discovery-chain:%s", apiUID.String()),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, "api", "default", "default", "dc1", "trustdomain.consul", nil, nil),
							},
							Err: nil,
						},
						{
							CorrelationID: peerTrustBundleIDPrefix + "peer-a",
							Result: &pbpeering.TrustBundleReadResponse{
								Bundle: peerTrustBundles.Bundles[0],
							},
						},
						{
							CorrelationID: upstreamPeerWatchIDPrefix + extApiUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:     "node1",
											Address:  "127.0.0.1",
											PeerName: "peer-a",
										},
										Service: &structs.NodeService{
											ID:       "api-a-1",
											Service:  "api-a",
											PeerName: "peer-a",
											Connect: structs.ServiceConnect{
												PeerMeta: &structs.PeeringServiceMeta{
													SNI: []string{
														"payments.default.default.cloud.external." + peerTrustDomain,
													},
													SpiffeID: []string{
														"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/payments",
													},
													Protocol: "tcp",
												},
											},
										},
									},
								},
							},
						},
						{
							CorrelationID: "mesh-gateway:dc1",
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "10.1.2.3",
										},
										Service: structs.TestNodeServiceMeshGateway(t),
									},
								},
							},
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.True(t, snap.MeshGateway.isEmpty())

						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)
						prototest.AssertDeepEqual(t, peerTrustBundles.Bundles, snap.ConnectProxy.InboundPeerTrustBundles)

						require.Len(t, snap.ConnectProxy.DiscoveryChain, 1, "%+v", snap.ConnectProxy.DiscoveryChain)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1, "%+v", snap.ConnectProxy.WatchedUpstreams)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 1, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
						require.Len(t, snap.ConnectProxy.WatchedGateways, 1, "%+v", snap.ConnectProxy.WatchedGateways)
						require.Len(t, snap.ConnectProxy.WatchedGatewayEndpoints, 1, "%+v", snap.ConnectProxy.WatchedGatewayEndpoints)

						tb, ok := snap.ConnectProxy.UpstreamPeerTrustBundles.Get("peer-a")
						require.True(t, ok)
						prototest.AssertDeepEqual(t, peerTrustBundles.Bundles[0], tb)

						require.Equal(t, 1, snap.ConnectProxy.PeerUpstreamEndpoints.Len())
						ep, _ := snap.ConnectProxy.PeerUpstreamEndpoints.Get(extApiUID)
						require.NotNil(t, ep)

						require.Equal(t, 1, snap.ConnectProxy.WatchedLocalGWEndpoints.Len())

						gwEp, _ := snap.ConnectProxy.WatchedLocalGWEndpoints.Get("dc1")
						require.NotNil(t, gwEp)
						require.Len(t, gwEp, 1)
					},
				},
			},
		},
		"hcp-metrics": {
			ns: structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-sidecar-proxy",
				Service: "web-sidecar-proxy",
				Address: "10.0.1.1",
				Port:    443,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					Config: map[string]interface{}{
						"envoy_hcp_metrics_bind_socket_dir": "/tmp/consul/hcp-metrics/",
					},
				},
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				{
					requiredWatches: map[string]verifyWatchRequest{
						fmt.Sprintf("discovery-chain:%s", hcpCollectorUID.String()): genVerifyDiscoveryChainWatch(&structs.DiscoveryChainRequest{
							Name:                 hcpCollector.Name,
							EvaluateInDatacenter: "dc1",
							EvaluateInNamespace:  "default",
							EvaluateInPartition:  "default",
							Datacenter:           "dc1",
							QueryOptions: structs.QueryOptions{
								Token: aclToken,
							},
						}),
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.False(t, snap.Valid(), "should not be valid")

						require.Len(t, snap.ConnectProxy.DiscoveryChain, 0, "%+v", snap.ConnectProxy.DiscoveryChain)
						require.Len(t, snap.ConnectProxy.WatchedDiscoveryChains, 0, "%+v", snap.ConnectProxy.WatchedDiscoveryChains)
						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 0, "%+v", snap.ConnectProxy.WatchedUpstreams)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 0, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
					},
				},
				{
					events: []UpdateEvent{
						rootWatchEvent(),
						{
							CorrelationID: peeringTrustBundlesWatchID,
							Result:        peerTrustBundles,
						},
						{
							CorrelationID: leafWatchID,
							Result:        issuedCert,
							Err:           nil,
						},
						{
							CorrelationID: intentionsWatchID,
							Result:        TestIntentions(),
							Err:           nil,
						},
						{
							CorrelationID: meshConfigEntryID,
							Result:        &structs.ConfigEntryResponse{},
						},
						{
							CorrelationID: fmt.Sprintf("discovery-chain:%s", hcpCollectorUID.String()),
							Result: &structs.DiscoveryChainResponse{
								Chain: discoverychain.TestCompileConfigEntries(t, hcpCollector.Name, "default", "default", "dc1", "trustdomain.consul", nil, nil),
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)

						// An event was received with the HCP collector's discovery chain, which sets up some bookkeeping in the snapshot.
						require.Len(t, snap.ConnectProxy.DiscoveryChain, 1, "%+v", snap.ConnectProxy.DiscoveryChain)
						require.Contains(t, snap.ConnectProxy.DiscoveryChain, hcpCollectorUID)

						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1, "%+v", snap.ConnectProxy.WatchedUpstreams)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 1, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, hcpCollectorUID)

						expectUpstream := structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      apimod.HCPMetricsCollectorName,
							LocalBindSocketPath:  "/tmp/consul/hcp-metrics/gqmuzdHCUPAEY5mbF8vgkZCNI14.sock",
							Config: map[string]interface{}{
								"protocol": "grpc",
							},
						}
						uid := NewUpstreamID(&expectUpstream)

						require.Contains(t, snap.ConnectProxy.UpstreamConfig, uid)
						require.Equal(t, &expectUpstream, snap.ConnectProxy.UpstreamConfig[uid])

						// No endpoints have arrived yet.
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[hcpCollectorUID], 0, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
					},
				},
				{
					requiredWatches: map[string]verifyWatchRequest{
						fmt.Sprintf("upstream-target:%s.default.default.dc1:", apimod.HCPMetricsCollectorName) + hcpCollectorUID.String(): genVerifyServiceSpecificRequest(apimod.HCPMetricsCollectorName, "", "dc1", true),
					},
					events: []UpdateEvent{
						{
							CorrelationID: fmt.Sprintf("upstream-target:%s.default.default.dc1:", apimod.HCPMetricsCollectorName) + hcpCollectorUID.String(),
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "10.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      apimod.HCPMetricsCollectorName,
											Service: apimod.HCPMetricsCollectorName,
											Port:    8080,
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid())
						require.Equal(t, indexedRoots, snap.Roots)
						require.Equal(t, issuedCert, snap.ConnectProxy.Leaf)

						// Discovery chain for the HCP collector should still be stored in the snapshot.
						require.Len(t, snap.ConnectProxy.DiscoveryChain, 1, "%+v", snap.ConnectProxy.DiscoveryChain)
						require.Contains(t, snap.ConnectProxy.DiscoveryChain, hcpCollectorUID)

						require.Len(t, snap.ConnectProxy.WatchedUpstreams, 1, "%+v", snap.ConnectProxy.WatchedUpstreams)
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints, 1, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)
						require.Contains(t, snap.ConnectProxy.WatchedUpstreamEndpoints, hcpCollectorUID)

						// An endpoint arrived for the HCP collector, so it should be present in the snapshot.
						require.Len(t, snap.ConnectProxy.WatchedUpstreamEndpoints[hcpCollectorUID], 1, "%+v", snap.ConnectProxy.WatchedUpstreamEndpoints)

						nodes := structs.CheckServiceNodes{
							{
								Node: &structs.Node{
									Node:    "node1",
									Address: "10.0.0.1",
								},
								Service: &structs.NodeService{
									ID:      apimod.HCPMetricsCollectorName,
									Service: apimod.HCPMetricsCollectorName,
									Port:    8080,
								},
							},
						}
						target := fmt.Sprintf("%s.default.default.dc1", apimod.HCPMetricsCollectorName)
						require.Equal(t, nodes, snap.ConnectProxy.WatchedUpstreamEndpoints[hcpCollectorUID][target])
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			proxyID := ProxyID{ServiceID: tc.ns.CompoundServiceID()}

			sc := stateConfig{
				logger: testutil.Logger(t),
				source: &structs.QuerySource{
					Datacenter: tc.sourceDC,
				},
				dnsConfig: DNSConfig{
					Domain:    "consul.",
					AltDomain: "alt.consul.",
				},
			}
			wr := recordWatches(&sc)

			state, err := newState(proxyID, &tc.ns, testSource, aclToken, sc, rate.NewLimiter(rate.Inf, 0))

			// verify building the initial state worked
			require.NoError(t, err)
			require.NotNil(t, state)

			// setup the test logger to use the t.Log
			state.logger = testutil.Logger(t)

			// setup the ctx as initWatches expects this to be there
			var ctx context.Context
			ctx, state.cancel = context.WithCancel(context.Background())

			snap, err := state.handler.initialize(ctx)
			require.NoError(t, err)

			// --------------------------------------------------------------------
			//
			// All the nested subtests here are to make failures easier to
			// correlate back with the test table
			//
			// --------------------------------------------------------------------

			for idx, stage := range tc.stages {
				require.True(t, t.Run(fmt.Sprintf("stage-%d", idx), func(t *testing.T) {
					for correlationId, verifier := range stage.requiredWatches {
						require.True(t, t.Run(correlationId, func(t *testing.T) {
							wr.verify(t, correlationId, verifier)
						}))
					}

					// the state is not currently executing the run method in a goroutine
					// therefore we just tell it about the updates
					for eveIdx, event := range stage.events {
						require.True(t, t.Run(fmt.Sprintf("update-%d", eveIdx), func(t *testing.T) {
							require.NoError(t, state.handler.handleUpdate(ctx, event, &snap))
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

func Test_hostnameEndpoints(t *testing.T) {
	type testCase struct {
		name     string
		localKey GatewayKey
		nodes    structs.CheckServiceNodes
		want     structs.CheckServiceNodes
	}
	run := func(t *testing.T, tc testCase) {
		logger := hclog.New(nil)
		got := hostnameEndpoints(logger, tc.localKey, tc.nodes)
		require.Equal(t, tc.want, got)
	}

	cases := []testCase{
		{
			name:     "same locality and no LAN hostname endpoints",
			localKey: GatewayKey{Datacenter: "dc1", Partition: acl.PartitionOrDefault("")},
			nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"10.0.1.1", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-1.elb.notaws.com", Port: 443}),
				},
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"10.0.2.2", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
				},
			},
			want: nil,
		},
		{
			name:     "same locality and one LAN hostname endpoint",
			localKey: GatewayKey{Datacenter: "dc1", Partition: acl.PartitionOrDefault("")},
			nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"gateway.mydomain", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-1.elb.notaws.com", Port: 443}),
				},
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"10.0.2.2", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
				},
			},
			want: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"gateway.mydomain", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-1.elb.notaws.com", Port: 443}),
				},
			},
		},
		{
			name:     "different locality and one WAN hostname endpoint",
			localKey: GatewayKey{Datacenter: "dc2", Partition: acl.PartitionOrDefault("")},
			nodes: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"gateway.mydomain", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "8.8.8.8", Port: 443}),
				},
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"10.0.2.2", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
				},
			},
			want: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						Node:       "mesh-gateway",
						Datacenter: "dc1",
					},
					Service: structs.TestNodeServiceMeshGatewayWithAddrs(t,
						"10.0.2.2", 8443,
						structs.ServiceAddress{},
						structs.ServiceAddress{Address: "123.us-west-2.elb.notaws.com", Port: 443}),
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run(t, c)
		})
	}
}

const aclToken = "foo"
