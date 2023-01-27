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
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestBuiltinExtensionsFromSnapshot(t *testing.T) {
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

	makeLuaServiceDefaults := func(inbound bool) *structs.ServiceConfigEntry {
		listener := "inbound"
		if !inbound {
			listener = "outbound"
		}

		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "db",
			Protocol: "http",
			EnvoyExtensions: []structs.EnvoyExtension{
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
			},
		}
	}

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
	}{
		{
			name: "lambda-connect-proxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, makeLambdaServiceDefaults(false))
			},
		},
		// Make sure that if the upstream type is different from ExtensionConfiguration.Kind is, that the resources are not patched.
		{
			name: "lambda-connect-proxy-with-terminating-gateway-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", nil, nil, makeLambdaServiceDefaults(false))
			},
		},
		{
			name: "lambda-connect-proxy-opposite-meta",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, makeLambdaServiceDefaults(true))
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
			name: "lua-outbound-applies-to-upstreams",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, makeLuaServiceDefaults(false))
			},
		},
		{
			name: "lua-inbound-doesnt-applies-to-upstreams",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, makeLuaServiceDefaults(true))
			},
		},
		{
			name: "lua-inbound-applies-to-inbound",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
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
				}, nil)
			},
		},
		{
			name: "lua-outbound-doesnt-apply-to-inbound",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
						{
							Name: api.BuiltinLuaExtension,
							Arguments: map[string]interface{}{
								"ProxyType": "connect-proxy",
								"Listener":  "outbound",
								"Script": `
function envoy_on_request(request_handle)
  request_handle:headers():add("test", "test")
end`,
							},
						},
					}
				}, nil)
			},
		},
		{
			name: "lua-connect-proxy-with-terminating-gateway-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", nil, nil, makeLambdaServiceDefaults(false))
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
					// golden files for every test case and so not be any use!
					setupTLSRootsAndLeaf(t, snap)

					g := newResourceGenerator(testutil.Logger(t), nil, false)
					g.ProxyFeatures = sf

					res, err := g.allResourcesFromSnapshot(snap)
					require.NoError(t, err)

					indexedResources := indexResources(g.Logger, res)
					cfgs := xdscommon.GetExtensionConfigurations(snap)
					for _, extensions := range cfgs {
						for _, ext := range extensions {
							builtInExtension, ok := GetBuiltInExtension(ext)
							require.True(t, ok)
							err = builtInExtension.Validate(ext)
							require.NoError(t, err)
							indexedResources, err = builtInExtension.Extend(indexedResources, ext)
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
