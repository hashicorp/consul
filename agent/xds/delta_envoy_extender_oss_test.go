// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package xds

import (
	"path/filepath"
	"sort"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/envoyextensions"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/extensionruntime"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestEnvoyExtenderWithSnapshot(t *testing.T) {
	// If opposite is true, the returned service defaults config entry will have
	// payload-passthrough=true and invocation-mode=asynchronous.
	// Otherwise payload-passthrough=false and invocation-mode=synchronous.
	// This is used to test all the permutations.
	makeLambdaServiceDefaults := func(opposite bool) *structs.ServiceConfigEntry {
		payloadPassthrough := true
		if opposite {
			payloadPassthrough = false
		}

		invocationMode := "synchronous"
		if opposite {
			invocationMode = "asynchronous"
		}

		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "db",
			Protocol: "http",
			EnvoyExtensions: []structs.EnvoyExtension{
				{
					Name: api.BuiltinAWSLambdaExtension,
					Arguments: map[string]interface{}{
						"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
						"PayloadPassthrough": payloadPassthrough,
						"InvocationMode":     invocationMode,
					},
				},
			},
		}
	}

	// Apply Lua extension to the local service and ensure http is used so the extension can be applied.
	makeLuaNsFunc := func(inbound bool) func(ns *structs.NodeService) {
		listener := "inbound"
		if !inbound {
			listener = "outbound"
		}

		return func(ns *structs.NodeService) {
			ns.Proxy.Config["protocol"] = "http"
			ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
				{
					Name: api.BuiltinLuaExtension,
					Arguments: map[string]interface{}{
						"ProxyType": "connect-proxy",
						"Listener":  listener,
						"Script": `
function envoy_on_request(request_handle)
  request_handle:headers():add("test", "test")
end`,
					},
				},
			}
		}
	}

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
	}{
		{
			name: "lambda-connect-proxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil, makeLambdaServiceDefaults(false))
			},
		},
		{
			name: "lambda-connect-proxy-tproxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				extra := makeLambdaServiceDefaults(false)
				extra.Name = "google"
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, extra)
			},
		},
		// Make sure that if the upstream type is different from ExtensionConfiguration.Kind is, that the resources are not patched.
		{
			name: "lambda-connect-proxy-with-terminating-gateway-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", false, nil, nil, makeLambdaServiceDefaults(false))
			},
		},
		{
			name: "lambda-connect-proxy-opposite-meta",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil, makeLambdaServiceDefaults(true))
			},
		},
		{
			name: "lambda-terminating-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGatewayWithLambdaService(t)
			},
		},
		{
			name:   "lambda-terminating-gateway-with-service-resolvers",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayWithLambdaServiceAndServiceResolvers,
		},
		{
			name: "lua-outbound-applies-to-local-upstreams",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// upstreams need to be http in order for lua to be applied to listeners.
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, makeLuaNsFunc(false), nil, &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "db",
					Protocol: "http",
				}, &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "geo-cache",
					Protocol: "http",
				})
			},
		},
		{
			// We expect an inbound public listener lua filter here because the extension targets inbound.
			// The only difference between goldens for this and lua-inbound-applies-to-inbound
			// should be that db has HTTP filters rather than TCP.
			name: "lua-inbound-doesnt-apply-to-local-upstreams",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// db is made an HTTP upstream so that the extension _could_ apply, but does not because
				// the direction for the extension is inbound.
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, makeLuaNsFunc(true), nil, &structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "db",
					Protocol: "http",
				})
			},
		},
		{
			name: "lua-inbound-applies-to-inbound",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, makeLuaNsFunc(true), nil)
			},
		},
		{
			// We expect _no_ lua filters here, because the extension targets outbound, but there are
			// no upstream HTTP services. We also should not see public listener, which is HTTP, patched.
			name: "lua-outbound-doesnt-apply-to-inbound",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, makeLuaNsFunc(false), nil)
			},
		},
		{
			name: "lua-connect-proxy-with-terminating-gateway-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", false, nil, nil, makeLambdaServiceDefaults(false))
			},
		},
		{
			name: "lambda-and-lua-connect-proxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				nsFunc := func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
						{
							Name: api.BuiltinLuaExtension,
							Arguments: map[string]interface{}{
								"ProxyType": "connect-proxy",
								"Listener":  "inbound",
								"Script": `
function envoy_on_request(request_handle)
  request_handle:headers():add("test", "test")
end`,
							},
						},
					}
				}
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nsFunc, nil, makeLambdaServiceDefaults(true))
			},
		},
		{
			name: "http-local-ratelimit-applyto-filter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
						{
							Name: api.BuiltinLocalRatelimitExtension,
							Arguments: map[string]interface{}{
								"ProxyType":      "connect-proxy",
								"MaxTokens":      3,
								"TokensPerFill":  2,
								"FillInterval":   10,
								"FilterEnabled":  100,
								"FilterEnforced": 100,
							},
						},
					}
				}, nil)
			},
		},
		{
			name: "wasm-http-local-file",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
						{
							Name: api.BuiltinWasmExtension,
							Arguments: map[string]interface{}{
								"Protocol":     "http",
								"ListenerType": "inbound",
								"PluginConfig": map[string]interface{}{
									"VmConfig": map[string]interface{}{
										"Code": map[string]interface{}{
											"Local": map[string]interface{}{
												"Filename": "/path/to/extension.wasm",
											},
										},
									},
									"Configuration": `{"foo": "bar"}`,
								},
							},
						},
					}
				}, nil)
			},
		},
		{
			name: "wasm-http-remote-file",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
						{
							Name: api.BuiltinWasmExtension,
							Arguments: map[string]interface{}{
								"Protocol":     "http",
								"ListenerType": "inbound",
								"PluginConfig": map[string]interface{}{
									"VmConfig": map[string]interface{}{
										"Code": map[string]interface{}{
											"Remote": map[string]interface{}{
												"HttpURI": map[string]interface{}{
													"Service": map[string]interface{}{
														"Name": "db",
													},
													"URI": "https://db/plugin.wasm",
												},
												"SHA256": "d05d88b0ce8a8f1d5176481e0af3ae5c65ed82cbfb8c61506c5354b076078545",
											},
										},
									},
									"Configuration": `{"foo": "bar"}`,
								},
							},
						},
					}
				}, nil)
			},
		},
		{
			// Insert an HTTP ext_authz filter at the start of the filter chain with the default gRPC config options.
			name: "ext-authz-http-local-grpc-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "http"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension(
						"grpc",
						"dest=local",
					)
				}, nil)
			},
		},
		{
			// Insert an ext_authz HTTP filter after all the header_to_metadata filters, with the default HTTP config options.
			name: "ext-authz-http-local-http-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "http"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension(
						"http",
						"dest=local",
						"insert=AfterLastMatch:envoy.filters.http.header_to_metadata",
					)
				}, nil)
			},
		},
		{
			// Insert an ext_authz HTTP filter before the router filter, specifying all gRPC config options.
			name: "ext-authz-http-upstream-grpc-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "http"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension(
						"grpc",
						"required=true",
						"dest=upstream",
						"insert=BeforeFirstMatch:envoy.filters.http.router",
						"config-type=full",
					)
				}, nil)
			},
		},
		{
			// Insert an ext_authz HTTP filter after intentions, specifying all HTTP config options.
			name: "ext-authz-http-upstream-http-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "http"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension(
						"http",
						"required=true",
						"dest=upstream",
						"insert=AfterLastMatch:envoy.filters.http.rbac",
						"config-type=full",
					)
				}, nil)
			},
		},
		{
			// Insert an ext_authz TCP filter at the start of the filter chain, with the default gRPC config options.
			name: "ext-authz-tcp-local-grpc-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "tcp"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension("grpc")
				}, nil)
			},
		},
		{
			// Insert an ext_authz TCP filter after intentions, specifying all gRPC config options.
			name: "ext-authz-tcp-upstream-grpc-service",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]any{"protocol": "tcp"}
					ns.Proxy.EnvoyExtensions = makeExtAuthzEnvoyExtension(
						"grpc",
						"required=true",
						"dest=upstream",
						"insert=AfterLastMatch:envoy.filters.network.rbac",
						"config-type=full",
					)
				}, nil)
			},
		},
	}

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golden files for every test case and so not be any use!
					testcommon.SetupTLSRootsAndLeaf(t, snap)

					g := NewResourceGenerator(testutil.Logger(t), nil, false)
					g.ProxyFeatures = sf

					res, err := g.AllResourcesFromSnapshot(snap)
					require.NoError(t, err)

					indexedResources := xdscommon.IndexResources(g.Logger, res)
					cfgs := extensionruntime.GetRuntimeConfigurations(snap)
					for _, extensions := range cfgs {
						for _, ext := range extensions {
							extender, err := envoyextensions.ConstructExtension(ext.EnvoyExtension)
							require.NoError(t, err)
							err = extender.Validate(&ext)
							require.NoError(t, err)
							indexedResources, err = extender.Extend(indexedResources, &ext)
							require.NoError(t, err)
						}
					}

					entities := []struct {
						name   string
						key    string
						sorter func([]proto.Message) func(int, int) bool
					}{
						{
							name: "clusters",
							key:  xdscommon.ClusterType,
							sorter: func(msgs []proto.Message) func(int, int) bool {
								return func(i, j int) bool {
									return msgs[i].(*envoy_cluster_v3.Cluster).Name < msgs[j].(*envoy_cluster_v3.Cluster).Name
								}
							},
						},
						{
							name: "listeners",
							key:  xdscommon.ListenerType,
							sorter: func(msgs []proto.Message) func(int, int) bool {
								return func(i, j int) bool {
									return msgs[i].(*envoy_listener_v3.Listener).Name < msgs[j].(*envoy_listener_v3.Listener).Name
								}
							},
						},
						{
							name: "routes",
							key:  xdscommon.RouteType,
							sorter: func(msgs []proto.Message) func(int, int) bool {
								return func(i, j int) bool {
									return msgs[i].(*envoy_route_v3.RouteConfiguration).Name < msgs[j].(*envoy_route_v3.RouteConfiguration).Name
								}
							},
						},
						{
							name: "endpoints",
							key:  xdscommon.EndpointType,
							sorter: func(msgs []proto.Message) func(int, int) bool {
								return func(i, j int) bool {
									return msgs[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < msgs[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
								}
							},
						},
					}

					for _, entity := range entities {
						var msgs []proto.Message
						for _, e := range indexedResources.Index[entity.key] {
							msgs = append(msgs, e)
						}

						sort.Slice(msgs, entity.sorter(msgs))
						r, err := createResponse(entity.key, "00000001", "00000001", msgs)
						require.NoError(t, err)

						t.Run(entity.name, func(t *testing.T) {
							gotJSON := protoToJSON(t, r)

							require.JSONEq(t, goldenEnvoy(t,
								filepath.Join("builtin_extension", entity.name, tt.name),
								envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
						})
					}
				})
			}
		})
	}
}
