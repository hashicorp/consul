// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"bytes"
	"path/filepath"
	"sort"
	"testing"
	"text/template"

	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/stretchr/testify/assert"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

type listenerTestCase struct {
	name   string
	create func(t testinf.T) *proxycfg.ConfigSnapshot
	// Setup is called before the test starts. It is passed the snapshot from
	// TestConfigSnapshot and is allowed to modify it in any way to setup the
	// test input.
	overrideGoldenName string
	generatorSetup     func(*ResourceGenerator)
}

func makeListenerDiscoChainTests(enterprise bool) []listenerTestCase {
	return []listenerTestCase{
		{
			name: "custom-upstream-ignored-with-disco-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", enterprise, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						if ns.Proxy.Upstreams[i].DestinationName != "db" {
							continue // only tweak the db upstream
						}
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						uid := proxycfg.NewUpstreamID(&ns.Proxy.Upstreams[i])

						ns.Proxy.Upstreams[i].Config["envoy_listener_json"] =
							customListenerJSON(t, customListenerJSONOptions{
								Name: uid.EnvoyID() + ":custom-upstream",
							})
					}
				}, nil)
			},
		},
		{
			name: "splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-http-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "http",
						},
					},
				)
			},
		},
		{
			name: "connect-proxy-with-http2-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "http2",
						},
					},
				)
			},
		},
		{
			name: "connect-proxy-with-grpc-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "grpc",
						},
					},
				)
			},
		},
		{
			name: "connect-proxy-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "external-sni", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-and-overrides",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple-with-overrides", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway", enterprise, nil, nil)
			},
		},
	}
}

func TestListenersFromSnapshot(t *testing.T) {
	// TODO: we should move all of these to TestAllResourcesFromSnapshot
	// eventually to test all of the xDS types at once with the same input,
	// just as it would be triggered by our xDS server.
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []listenerTestCase{
		{
			name: "connect-proxy-with-tls-outgoing-min-version-auto",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSVersionAuto,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-incoming-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-incoming-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-incoming-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										CipherSuites: []types.TLSCipherSuite{
											types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
											types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "grpc-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "grpc"
				}, nil)
			},
		},
		{
			name: "listener-bind-address",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_address"] = "127.0.0.2"
				}, nil)
			},
		},
		{
			name: "listener-bind-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_port"] = 8888
				}, nil)
			},
		},
		{
			name: "listener-bind-address-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_address"] = "127.0.0.2"
					ns.Proxy.Config["bind_port"] = 8888
				}, nil)
			},
		},
		{
			name: "listener-unix-domain-socket",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].LocalBindAddress = ""
					ns.Proxy.Upstreams[0].LocalBindPort = 0
					ns.Proxy.Upstreams[0].LocalBindSocketPath = "/tmp/service-mesh/client-1/grpc-employee-server"
					ns.Proxy.Upstreams[0].LocalBindSocketMode = "0640"
				}, nil)
			},
		},
		{
			name: "listener-max-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["max_inbound_connections"] = 222
				}, nil)
			},
		},
		{
			name: "http2-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http2"
				}, nil)
			},
		},
		{
			name: "listener-balance-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["balance_inbound_connections"] = "exact_balance"
				}, nil)
			},
		},
		{
			name: "listener-balance-outbound-connections-bind-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["balance_outbound_connections"] = "exact_balance"
				}, nil)
			},
		},
		{
			name: "http-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
				}, nil)
			},
		},
		{
			name: "http-public-listener-no-xfcc",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t,
					func(ns *structs.NodeService) {
						ns.Proxy.Config["protocol"] = "http"
					},
					[]proxycfg.UpdateEvent{
						{
							CorrelationID: "mesh",
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									HTTP: &structs.MeshHTTPConfig{
										SanitizeXForwardedClientCert: true,
									},
								},
							},
						},
					})
			},
		},
		{
			name: "http-listener-with-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["local_connect_timeout_ms"] = 1234
					ns.Proxy.Config["local_request_timeout_ms"] = 2345
					ns.Proxy.Config["local_idle_timeout_ms"] = 3456
				}, nil)
			},
		},
		{
			name: "http-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["protocol"] = "http"
				}, nil)
			},
		},
		{
			name: "custom-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customHTTPListenerJSON(t, customHTTPListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http-2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customHTTPListenerJSON(t, customHTTPListenerJSONOptions{
							Name:                      "custom-public-listen",
							HTTPConnectionManagerName: httpConnectionManagerNewName,
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http-missing",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name:               "custom-public-listener-ignores-tls",
			overrideGoldenName: "custom-public-listener", // should be the same
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
							// Attempt to override the TLS context should be ignored
							TLSContext: `"allowRenegotiation": false`,
						})
				}, nil)
			},
		},
		{
			name: "custom-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						if ns.Proxy.Upstreams[i].DestinationName != "db" {
							continue // only tweak the db upstream
						}
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						uid := proxycfg.NewUpstreamID(&ns.Proxy.Upstreams[i])

						ns.Proxy.Upstreams[i].Config["envoy_listener_json"] =
							customListenerJSON(t, customListenerJSONOptions{
								Name: uid.EnvoyID() + ":custom-upstream",
							})
					}
				}, nil)
			},
		},
		{
			name: "connect-proxy-upstream-defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for _, v := range ns.Proxy.Upstreams {
						// Prepared queries do not get centrally configured upstream defaults merged into them.
						if v.DestinationType == structs.UpstreamDestTypePreparedQuery {
							continue
						}
						// Represent upstream config as if it came from centrally configured upstream defaults.
						// The name/namespace must not make it onto the cluster name attached to the outbound listener.
						v.CentrallyConfigured = true
						v.DestinationNamespace = structs.WildcardSpecifier
						v.DestinationName = structs.WildcardSpecifier
					}
				}, nil)
			},
		},
		{
			name: "expose-paths-local-app-paths",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotExposeConfig(t, nil)
			},
		},
		{
			name: "expose-paths-new-cluster-http2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotExposeConfig(t, func(ns *structs.NodeService) {
					ns.Proxy.Expose.Paths[1] = structs.ExposePath{
						LocalPathPort: 9090,
						Path:          "/grpc.health.v1.Health/Check",
						ListenerPort:  21501,
						Protocol:      "http2",
					}
				})
			},
		},
		{
			// NOTE: if IPv6 is not supported in the kernel per
			// kernelSupportsIPv6() then this test will fail because the golden
			// files were generated assuming ipv6 support was present
			name:   "expose-checks",
			create: proxycfg.TestConfigSnapshotExposeChecks,
			generatorSetup: func(s *ResourceGenerator) {
				s.CfgFetcher = configFetcherFunc(func() string {
					return "192.0.2.1"
				})
			},
		},
		{
			name: "mesh-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", nil, nil)
			},
		},
		{
			name: "mesh-gateway-using-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "federation-states", nil, nil)
			},
		},
		{
			name: "mesh-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "no-services", nil, nil)
			},
		},
		{
			name: "mesh-gateway-tagged-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_mesh_gateway_no_default_bind":       true,
						"envoy_mesh_gateway_bind_tagged_addresses": true,
					}
				}, nil)
			},
		},
		{
			name: "mesh-gateway-custom-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_mesh_gateway_bind_addresses": map[string]structs.ServiceAddress{
							"foo": {
								Address: "198.17.2.3",
								Port:    8080,
							},
							"bar": {
								Address: "2001:db8::ff",
								Port:    9999,
							},
							"baz": {
								Address: "127.0.0.1",
								Port:    8765,
							},
						},
					}
				}, nil)
			},
		},
		{
			name: "api-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, nil, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-nil-config-entry",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway_NilConfigEntry(t)
			},
		},
		{
			name: "api-gateway-tcp-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
						},
					}
				}, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-tcp-listener-with-tcp-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Routes: []structs.ResourceReference{
								{
									Name: "tcp-route",
									Kind: structs.TCPRoute,
								},
							},
						},
					}

				}, []structs.BoundRoute{
					&structs.TCPRouteConfigEntry{
						Name: "tcp-route",
						Kind: structs.TCPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Services: []structs.TCPService{
							{Name: "tcp-service"},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "api-gateway-http-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name:   "listener",
							Routes: []structs.ResourceReference{},
						},
					}
				}, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-http-listener-with-http-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Routes: []structs.ResourceReference{
								{
									Name: "http-route",
									Kind: structs.HTTPRoute,
								},
							},
						},
					}
				}, []structs.BoundRoute{
					&structs.HTTPRouteConfigEntry{
						Name: "http-route",
						Kind: structs.HTTPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Rules: []structs.HTTPRouteRule{
							{
								Services: []structs.HTTPService{
									{Name: "http-service"},
								},
							},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "api-gateway-tcp-listener-with-tcp-and-http-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener-tcp",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
						{
							Name:     "listener-http",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8081,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener-tcp",
							Routes: []structs.ResourceReference{
								{
									Name: "tcp-route",
									Kind: structs.TCPRoute,
								},
							},
						},
						{
							Name: "listener-http",
							Routes: []structs.ResourceReference{
								{
									Name: "http-route",
									Kind: structs.HTTPRoute,
								},
							},
						},
					}
				}, []structs.BoundRoute{
					&structs.TCPRouteConfigEntry{
						Name: "tcp-route",
						Kind: structs.TCPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Services: []structs.TCPService{
							{Name: "tcp-service"},
						},
					},
					&structs.HTTPRouteConfigEntry{
						Name: "http-route",
						Kind: structs.HTTPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Rules: []structs.HTTPRouteRule{
							{
								Services: []structs.HTTPService{
									{Name: "http-service"},
								},
							},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "ingress-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, nil, nil)
			},
		},
		{
			name: "ingress-gateway-nil-config-entry",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway_NilConfigEntry(t)
			},
		},
		{
			name: "ingress-gateway-bind-addrs",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", func(ns *structs.NodeService) {
					//
					ns.TaggedAddresses = map[string]structs.ServiceAddress{
						"lan": {Address: "10.0.0.1"},
						"wan": {Address: "172.16.0.1"},
					}
					ns.Proxy.Config = map[string]interface{}{
						"envoy_gateway_no_default_bind":       true,
						"envoy_gateway_bind_tagged_addresses": true,
						"envoy_gateway_bind_addresses": map[string]structs.ServiceAddress{
							"foo": {Address: "8.8.8.8"},
						},
					}
				}, nil, nil)
			},
		},
		{
			name: "ingress-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp",
					"default", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"external-sni", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-remote-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-local-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"splitter-with-resolver-redirect-multidc", nil, nil, nil)
			},
		},
		{
			name: "terminating-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
			},
		},
		{
			name: "terminating-gateway-with-tls-incoming-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "terminating-gateway-with-tls-incoming-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "terminating-gateway-with-tls-incoming-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										CipherSuites: []types.TLSCipherSuite{
											types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
											types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "terminating-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, false, nil, nil)
			},
		},
		{
			name: "terminating-gateway-custom-and-tagged-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_gateway_no_default_bind":       true,
						"envoy_gateway_bind_tagged_addresses": true,
						"envoy_gateway_bind_addresses": map[string]structs.ServiceAddress{
							// This bind address should not get a listener due to deduplication and it sorts to the end
							"z-duplicate-of-tagged-wan-addr": {
								Address: "198.18.0.1",
								Port:    443,
							},
							"foo": {
								Address: "198.17.2.3",
								Port:    8080,
							},
						},
					}
				}, nil)
			},
		},
		{
			name:   "terminating-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayServiceSubsets,
		},
		{
			name:   "ingress-http-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_HTTPMultipleServices,
		},
		{
			name:   "ingress-grpc-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_GRPCMultipleServices,
		},
		{
			name: "terminating-gateway-no-api-cert",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				api := structs.NewServiceName("api", nil)
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "service-leaf:" + api.String(), // serviceLeafIDPrefix
						Result:        nil,                            // tombstone this
					},
				})
			},
		},
		{
			name: "ingress-with-tls-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
					}, nil)
			},
		},
		{
			name: "ingress-with-tls-listener-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.TLSMinVersion = types.TLSv1_3
					}, nil)
			},
		},
		{
			name: "ingress-with-tls-listener-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.TLSMaxVersion = types.TLSv1_2
					}, nil)
			},
		},
		{
			name: "ingress-with-tls-listener-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.CipherSuites = []types.TLSCipherSuite{
							types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
							types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
						}
					}, nil)
			},
		},
		{
			name:   "ingress-with-tls-mixed-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_MixedListeners,
		},
		{
			name:   "ingress-with-tls-min-version-listeners-gateway-defaults",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMinVersionListenersGatewayDefaults,
		},
		{
			name:   "ingress-with-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener,
		},
		{
			name:   "ingress-with-tls-mixed-min-version-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMixedMinVersionListeners,
		},
		{
			name:   "ingress-with-sds-listener-gw-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayLevel,
		},
		{
			name:   "ingress-with-sds-listener-listener-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel,
		},
		{
			name:   "ingress-with-sds-listener-gw-level-http",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel_HTTP,
		},
		{
			name:   "ingress-with-sds-listener-gw-level-mixed-tls",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayLevel_MixedTLS,
		},
		{
			name:   "ingress-with-sds-service-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_ServiceLevel,
		},
		{
			name:   "ingress-with-sds-listener+service-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_ListenerAndServiceLevel,
		},
		{
			name:   "ingress-with-sds-service-level-mixed-no-tls",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_MixedNoTLS,
		},
		{
			name:   "ingress-with-grpc-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener_GRPC,
		},
		{
			name:   "ingress-with-http2-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener_HTTP2,
		},
		{
			name:   "ingress-with-http2-and-grpc-multiple-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_MultiTLSListener_MixedHTTP2gRPC,
		},
		{
			name:   "ingress-with-http2-and-grpc-multiple-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_GWTLSListener_MixedHTTP2gRPC,
		},
		{
			name: "transparent-proxy-http-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t)
			},
		},
		{
			name:   "transparent-proxy-with-resolver-redirect-upstream",
			create: proxycfg.TestConfigSnapshotTransparentProxyResolverRedirectUpstream,
		},
		{
			name:   "transparent-proxy-catalog-destinations-only",
			create: proxycfg.TestConfigSnapshotTransparentProxyCatalogDestinationsOnly,
		},
		{
			name:   "transparent-proxy-dial-instances-directly",
			create: proxycfg.TestConfigSnapshotTransparentProxyDialDirectly,
		},
		{
			name:   "transparent-proxy-terminating-gateway",
			create: proxycfg.TestConfigSnapshotTransparentProxyTerminatingGatewayCatalogDestinationsOnly,
		},
		{
			name: "custom-trace-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_listener_tracing_json"] = customTraceJSON(t)
				}, nil)
			},
		},
		{
			name: "access-logs-defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled: true,
					}
				},
					nil)
			},
		},
		{
			name: "access-logs-json-file",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled:    true,
						Type:       structs.FileLogSinkType,
						Path:       "/tmp/accesslog.txt",
						JSONFormat: "{ \"custom_start_time\": \"%START_TIME%\" }",
					}
				},
					nil)
			},
		},
		{
			name: "access-logs-text-stderr-disablelistenerlogs",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                structs.StdErrLogSinkType,
						TextFormat:          "CUSTOM FORMAT %START_TIME%",
					}
				},
					nil)
			},
		},
		{
			name: "connect-proxy-with-tproxy-and-permissive-mtls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.MutualTLSMode = structs.MutualTLSModePermissive
					ns.Proxy.Mode = structs.ProxyModeTransparent
				},
					nil)
			},
		},
		{
			name: "connect-proxy-without-tproxy-and-permissive-mtls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.MutualTLSMode = structs.MutualTLSModePermissive
				},
					nil)
			},
		},
	}

	tests = append(tests, makeListenerDiscoChainTests(false)...)

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// TODO: it would be nice to be able to ensure these snapshots are always valid before we use them in a test.
					// require.True(t, snap.Valid())

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golder files for every test case and so not be any use!
					testcommon.SetupTLSRootsAndLeaf(t, snap)

					// Need server just for logger dependency
					g := NewResourceGenerator(testutil.Logger(t), nil, false)
					g.ProxyFeatures = sf
					if tt.generatorSetup != nil {
						tt.generatorSetup(g)
					}

					listeners, err := g.listenersFromSnapshot(snap)
					require.NoError(t, err)

					// The order of listeners returned via LDS isn't relevant, so it's safe
					// to sort these for the purposes of test comparisons.
					sort.Slice(listeners, func(i, j int) bool {
						return listeners[i].(*envoy_listener_v3.Listener).Name < listeners[j].(*envoy_listener_v3.Listener).Name
					})

					r, err := createResponse(xdscommon.ListenerType, "00000001", "00000001", listeners)
					require.NoError(t, err)

					t.Run("current", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						expectedJSON := goldenEnvoy(t, filepath.Join("listeners", gName), envoyVersion, latestEnvoyVersion, gotJSON)
						require.JSONEq(t, expectedJSON, gotJSON)
					})
				})
			}
		})
	}
}

type customListenerJSONOptions struct {
	Name       string
	TLSContext string
}

const customListenerJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.listener.v3.Listener",
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			{{ if .TLSContext -}}
			"transport_socket": {
				"name": "tls",
				"typed_config": {
					"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
					{{ .TLSContext }}
				}
			},
			{{- end }}
			"filters": [
				{
					"name": "envoy.filters.network.tcp_proxy",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
							"cluster": "random-cluster",
							"statPrefix": "foo-stats"
						}
				}
			]
		}
	]
}`

type customHTTPListenerJSONOptions struct {
	Name                      string
	HTTPConnectionManagerName string
}

const customHTTPListenerJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.listener.v3.Listener",
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			"filters": [
				{
					"name": "{{ .HTTPConnectionManagerName }}",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
						"http_filters": [
							{
								"name": "envoy.filters.http.router"
							}
						],
						"route_config": {
							"name": "public_listener",
							"virtual_hosts": [
								{
									"domains": [
										"*"
									],
									"name": "public_listener",
									"routes": [
										{
											"match": {
												"prefix": "/"
											},
											"route": {
												"cluster": "random-cluster"
											}
										}
									]
								}
							]
						}
					}
				}
			]
		}
	]
}`

var (
	customListenerJSONTemplate     = template.Must(template.New("").Parse(customListenerJSONTpl))
	customHTTPListenerJSONTemplate = template.Must(template.New("").Parse(customHTTPListenerJSONTpl))
)

func customListenerJSON(t testinf.T, opts customListenerJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, customListenerJSONTemplate.Execute(&buf, opts))
	return buf.String()
}

func customHTTPListenerJSON(t testinf.T, opts customHTTPListenerJSONOptions) string {
	t.Helper()
	if opts.HTTPConnectionManagerName == "" {
		opts.HTTPConnectionManagerName = httpConnectionManagerNewName
	}
	var buf bytes.Buffer
	require.NoError(t, customHTTPListenerJSONTemplate.Execute(&buf, opts))
	return buf.String()
}

func customTraceJSON(t testinf.T) string {
	t.Helper()
	return `
	{
        "@type" : "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.Tracing",
        "provider" : {
          "name" : "envoy.tracers.zipkin",
          "typed_config" : {
            "@type" : "type.googleapis.com/envoy.config.trace.v3.ZipkinConfig",
            "collector_cluster" : "otelcolector",
            "collector_endpoint" : "/api/v2/spans",
            "collector_endpoint_version" : "HTTP_JSON",
            "shared_span_context" : false
          }
        },
        "custom_tags" : [
          {
            "tag" : "custom_header",
            "request_header" : {
              "name" : "x-custom-traceid",
              "default_value" : ""
            }
          },
          {
            "tag" : "alloc_id",
            "environment" : {
              "name" : "NOMAD_ALLOC_ID"
            }
          }
        ]
      }
	`
}

type configFetcherFunc func() string

var _ ConfigFetcher = (configFetcherFunc)(nil)

func (f configFetcherFunc) AdvertiseAddrLAN() string {
	return f()
}

func TestResolveListenerSDSConfig(t *testing.T) {
	type testCase struct {
		name    string
		gwSDS   *structs.GatewayTLSSDSConfig
		lisSDS  *structs.GatewayTLSSDSConfig
		want    *structs.GatewayTLSSDSConfig
		wantErr string
	}

	run := func(tc testCase) {
		// fake a snapshot with just the data we care about
		snap := proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "sds-cluster",
					CertResource: "cert-resource",
				},
			}
		}, nil)
		// Override TLS configs
		snap.IngressGateway.TLSConfig.SDS = tc.gwSDS
		var listenerCfg structs.IngressListener
		for k, lisCfg := range snap.IngressGateway.Listeners {
			if tc.lisSDS == nil {
				lisCfg.TLS = nil
			} else {
				lisCfg.TLS = &structs.GatewayTLSConfig{
					SDS: tc.lisSDS,
				}
			}
			// Override listener cfg in map
			snap.IngressGateway.Listeners[k] = lisCfg
			// Save the last cfg doesn't matter which as we set same for all.
			listenerCfg = lisCfg
		}

		got, err := resolveListenerSDSConfig(snap.IngressGateway.TLSConfig.SDS, listenerCfg.TLS, listenerCfg.Port)
		if tc.wantErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		}
	}

	cases := []testCase{
		{
			name:   "no SDS config",
			gwSDS:  nil,
			lisSDS: nil,
			want:   nil,
		},
		{
			name: "all cluster-level SDS config",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
			lisSDS: nil,
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name:  "all listener-level SDS config",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name: "mixed level SDS config",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name: "override cert",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "gw-cert",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "lis-cert",
			},
		},
		{
			name: "override both",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "gw-cluster",
				CertResource: "gw-cert",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "lis-cluster",
				CertResource: "lis-cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "lis-cluster",
				CertResource: "lis-cert",
			},
		},
		{
			name:  "missing cluster listener",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			wantErr: "missing SDS cluster name",
		},
		{
			name:  "missing cert listener",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			wantErr: "missing SDS cert resource",
		},
		{
			name: "missing cluster gw",
			gwSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			lisSDS:  nil,
			wantErr: "missing SDS cluster name",
		},
		{
			name: "missing cert gw",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			lisSDS:  nil,
			wantErr: "missing SDS cert resource",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(tc)
		})
	}

}

func TestGetAlpnProtocols(t *testing.T) {
	tests := map[string]struct {
		protocol string
		want     []string
	}{
		"http": {
			protocol: "http",
			want:     []string{"http/1.1"},
		},
		"http2": {
			protocol: "http2",
			want:     []string{"h2", "http/1.1"},
		},
		"grpc": {
			protocol: "grpc",
			want:     []string{"h2", "http/1.1"},
		},
		"tcp": {
			protocol: "",
			want:     nil,
		},
		"empty": {
			protocol: "",
			want:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := getAlpnProtocols(tc.protocol)
			assert.Equal(t, tc.want, got)
		})
	}
}
