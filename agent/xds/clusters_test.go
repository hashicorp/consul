package xds

import (
	"bytes"
	"path/filepath"
	"sort"
	"testing"
	"text/template"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"

	"github.com/golang/protobuf/ptypes/wrappers"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestClustersFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// create func and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:   "defaults",
			create: proxycfg.TestConfigSnapshot,
			setup:  nil, // Default snapshot
		},
		{
			name:   "custom-local-app",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_local_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name: "mylocal",
					})
			},
		},
		{
			name:   "custom-upstream",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name: "myservice",
					})
			},
		},
		{
			name:   "custom-upstream-default-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChainDefault,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name: "myservice",
					})
				snap.ConnectProxy.UpstreamConfig = map[string]*structs.Upstream{
					"db": {
						// The local bind port is overridden by the escape hatch, but is required for explicit upstreams.
						LocalBindPort: 9191,
						Config: map[string]interface{}{
							"envoy_cluster_json": customAppClusterJSON(t, customClusterJSONOptions{
								Name: "myservice",
							}),
						},
					},
				}
			},
		},
		{
			name:               "custom-upstream-ignores-tls",
			create:             proxycfg.TestConfigSnapshot,
			overrideGoldenName: "custom-upstream", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name: "myservice",
						// Attempt to override the TLS context should be ignored
						TLSContext: `"allowRenegotiation": false`,
					})
			},
		},
		{
			name:   "custom-timeouts",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["local_connect_timeout_ms"] = 1234
				snap.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
			},
		},
		{
			name:   "custom-limits-max-connections-only",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					// We check if Config is nil because the prepared_query upstream is
					// initialized without a Config map. Use Upstreams[i] syntax to
					// modify the actual ConfigSnapshot instead of copying the Upstream
					// in the range.
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections": 500,
					}
				}
			},
		},
		{
			name:   "custom-limits-set-to-zero",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections":         0,
						"max_pending_requests":    0,
						"max_concurrent_requests": 0,
					}
				}
			},
		},
		{
			name:   "custom-limits",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections":         500,
						"max_pending_requests":    600,
						"max_concurrent_requests": 700,
					}
				}
			},
		},
		{
			name:   "connect-proxy-with-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotDiscoveryChainExternalSNI,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithOverrides,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-failover",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailover,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "connect-proxy-lb-in-resolver",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithLB,
			setup:  nil,
		},
		{
			name:   "expose-paths-local-app-paths",
			create: proxycfg.TestConfigSnapshotExposeConfig,
		},
		{
			name:   "downstream-service-with-unix-sockets",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Address = ""
				snap.Port = 0
				snap.Proxy.LocalServiceAddress = ""
				snap.Proxy.LocalServicePort = 0
				snap.Proxy.LocalServiceSocketPath = "/tmp/downstream_proxy.sock"
			},
		},
		{
			name:   "expose-paths-new-cluster-http2",
			create: proxycfg.TestConfigSnapshotExposeConfig,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Expose.Paths[1] = structs.ExposePath{
					LocalPathPort: 9090,
					Path:          "/grpc.health.v1.Health/Check",
					ListenerPort:  21501,
					Protocol:      "http2",
				}
			},
		},
		{
			name:   "expose-paths-grpc-new-cluster-http1",
			create: proxycfg.TestConfigSnapshotGRPCExposeHTTP1,
		},
		{
			name:   "mesh-gateway",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-using-federation-states",
			create: proxycfg.TestConfigSnapshotMeshGatewayUsingFederationStates,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-no-services",
			create: proxycfg.TestConfigSnapshotMeshGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-ignore-extra-resolvers",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("notfound", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "notfound",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-service-timeouts",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind:           structs.ServiceResolver,
						Name:           "bar",
						ConnectTimeout: 10 * time.Second,
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-non-hash-lb-injected",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "least_request",
							LeastRequestConfig: &structs.LeastRequestConfig{
								ChoiceCount: 5,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-hash-lb-ignored",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "ring_hash",
							RingHashConfig: &structs.RingHashConfig{
								MinimumRingSize: 20,
								MaximumRingSize: 50,
							},
						},
					},
				}
			},
		},
		{
			name:   "ingress-gateway",
			create: proxycfg.TestConfigSnapshotIngressGateway,
			setup:  nil,
		},
		{
			name:   "ingress-gateway-no-services",
			create: proxycfg.TestConfigSnapshotIngressGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain",
			create: proxycfg.TestConfigSnapshotIngress,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotIngressExternalSNI,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotIngressWithOverrides,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-failover",
			create: proxycfg.TestConfigSnapshotIngressWithFailover,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "ingress-lb-in-resolver",
			create: proxycfg.TestConfigSnapshotIngressWithLB,
			setup:  nil,
		},
		{
			name:   "terminating-gateway",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-no-services",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind: structs.ServiceResolver,
						Name: "web",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("cache", nil): {
						Kind: structs.ServiceResolver,
						Name: "cache",
						Subsets: map[string]structs.ServiceResolverSubset{
							"prod": {
								Filter: "Service.Meta.Env == prod",
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("cache", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-hostname-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("api", nil): {
						Kind: structs.ServiceResolver,
						Name: "api",
						Subsets: map[string]structs.ServiceResolverSubset{
							"alt": {
								Filter: "Service.Meta.domain == alt",
							},
						},
					},
					structs.NewServiceName("cache", nil): {
						Kind: structs.ServiceResolver,
						Name: "cache",
						Subsets: map[string]structs.ServiceResolverSubset{
							"prod": {
								Filter: "Service.Meta.Env == prod",
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("api", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("cache", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-ignore-extra-resolvers",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("notfound", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "notfound",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-lb-config",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "ring_hash",
							RingHashConfig: &structs.RingHashConfig{
								MinimumRingSize: 20,
								MaximumRingSize: 50,
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfg.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
			setup:  nil,
		},
		{
			name:   "transparent-proxy",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Mode = structs.ProxyModeTransparent
				snap.ConnectProxy.MeshConfigSet = true
			},
		},
		{
			name:   "transparent-proxy-catalog-destinations-only",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Mode = structs.ProxyModeTransparent

				snap.ConnectProxy.MeshConfigSet = true
				snap.ConnectProxy.MeshConfig = &structs.MeshConfigEntry{
					TransparentProxy: structs.TransparentProxyMeshConfig{
						MeshDestinationsOnly: true,
					},
				}
			},
		},
		{
			name:   "transparent-proxy-dial-instances-directly",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Mode = structs.ProxyModeTransparent
				kafka := structs.NewServiceName("kafka", nil)
				mongo := structs.NewServiceName("mongo", nil)

				snap.ConnectProxy.IntentionUpstreams = map[string]struct{}{
					kafka.String(): {},
					mongo.String(): {},
				}

				// We add a passthrough cluster for each upstream service name
				snap.ConnectProxy.PassthroughUpstreams = map[string]proxycfg.ServicePassthroughAddrs{
					kafka.String(): {
						SNI: "kafka.default.dc1.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul",
						SpiffeID: connect.SpiffeIDService{
							Host:       "e5b08d03-bfc3-c870-1833-baddb116e648.consul",
							Namespace:  "default",
							Datacenter: "dc1",
							Service:    "kafka",
						},
						Addrs: map[string]struct{}{
							"9.9.9.9": {},
						},
					},
					mongo.String(): {
						SNI: "mongo.default.dc1.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul",
						SpiffeID: connect.SpiffeIDService{
							Host:       "e5b08d03-bfc3-c870-1833-baddb116e648.consul",
							Namespace:  "default",
							Datacenter: "dc1",
							Service:    "mongo",
						},
						Addrs: map[string]struct{}{
							"10.10.10.10": {},
							"10.10.10.12": {},
						},
					},
				}

				// There should still be a cluster for non-passthrough requests
				snap.ConnectProxy.DiscoveryChain[mongo.String()] = discoverychain.TestCompileConfigEntries(t, "mongo", "default", "default", "dc1", connect.TestClusterID+".consul", nil)
				snap.ConnectProxy.WatchedUpstreamEndpoints[mongo.String()] = map[string]structs.CheckServiceNodes{
					"mongo.default.default.dc1": {
						structs.CheckServiceNode{
							Node: &structs.Node{
								Datacenter: "dc1",
							},
							Service: &structs.NodeService{
								Service: "mongo",
								Address: "7.7.7.7",
								Port:    27017,
								TaggedAddresses: map[string]structs.ServiceAddress{
									"virtual": {Address: "6.6.6.6"},
								},
							},
						},
					},
				}
			},
		},
	}

	latestEnvoyVersion := proxysupport.EnvoyVersions[0]
	for _, envoyVersion := range proxysupport.EnvoyVersions {
		sf, err := determineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golder files for every test case and so not be any use!
					setupTLSRootsAndLeaf(t, snap)

					if tt.setup != nil {
						tt.setup(snap)
					}

					// Need server just for logger dependency
					g := newResourceGenerator(testutil.Logger(t), nil, nil, false)
					g.ProxyFeatures = sf

					clusters, err := g.clustersFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(clusters, func(i, j int) bool {
						return clusters[i].(*envoy_cluster_v3.Cluster).Name < clusters[j].(*envoy_cluster_v3.Cluster).Name
					})

					r, err := createResponse(ClusterType, "00000001", "00000001", clusters)
					require.NoError(t, err)

					t.Run("current", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("clusters", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})
				})
			}
		})
	}
}

type customClusterJSONOptions struct {
	Name       string
	TLSContext string
}

var customAppClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	{{ if .TLSContext -}}
	"transport_socket": {
		"name": "tls",
		"typed_config": {
			"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
			{{ .TLSContext }}
		}
	},
	{{- end }}
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
		"endpoints": [
			{
				"lbEndpoints": [
					{
						"endpoint": {
							"address": {
								"socketAddress": {
									"address": "127.0.0.1",
									"portValue": 8080
								}
							}
						}
					}
				]
			}
		]
	}
}`

var customAppClusterJSONTemplate = template.Must(template.New("").Parse(customAppClusterJSONTpl))

func customAppClusterJSON(t *testing.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customAppClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

func setupTLSRootsAndLeaf(t *testing.T, snap *proxycfg.ConfigSnapshot) {
	if snap.Leaf() != nil {
		switch snap.Kind {
		case structs.ServiceKindConnectProxy:
			snap.ConnectProxy.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.ConnectProxy.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		case structs.ServiceKindIngressGateway:
			snap.IngressGateway.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.IngressGateway.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		}
	}
	if snap.Roots != nil {
		snap.Roots.Roots[0].RootCert = loadTestResource(t, "test-root-cert")
	}
}

func TestEnvoyLBConfig_InjectToCluster(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected *envoy_cluster_v3.Cluster
	}{
		{
			name: "skip empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			expected: &envoy_cluster_v3.Cluster{},
		},
		{
			name: "round robin",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRoundRobin,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_ROUND_ROBIN},
		},
		{
			name: "random",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRandom,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_RANDOM},
		},
		{
			name: "maglev",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_MAGLEV},
		},
		{
			name: "ring_hash",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRingHash,
				RingHashConfig: &structs.RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_RING_HASH,
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						MinimumRingSize: &wrappers.UInt64Value{Value: 3},
						MaximumRingSize: &wrappers.UInt64Value{Value: 7},
					},
				},
			},
		},
		{
			name: "least_request",
			lb: &structs.LoadBalancer{
				Policy: "least_request",
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_LEAST_REQUEST,
				LbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
					LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
						ChoiceCount: &wrappers.UInt32Value{Value: 3},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c envoy_cluster_v3.Cluster
			err := injectLBToCluster(tc.lb, &c)
			require.NoError(t, err)

			require.Equal(t, tc.expected, &c)
		})
	}
}
