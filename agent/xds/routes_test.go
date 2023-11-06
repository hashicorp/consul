// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"path/filepath"
	"sort"
	"testing"
	"time"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxystateconverter"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/agent/xdsv2"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
)

type routeTestCase struct {
	name               string
	create             func(t testinf.T) *proxycfg.ConfigSnapshot
	overrideGoldenName string
	alsoRunTestForV2   bool
}

func makeRouteDiscoChainTests(enterprise bool) []routeTestCase {
	return []routeTestCase{
		{
			name: "connect-proxy-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "external-sni", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-splitter-overweight",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-overweight", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-overrides",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple-with-overrides", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-splitter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-splitter", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-grpc-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "grpc-router", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-router", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "lb-resolver", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-route-to-lb-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-lb-node", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-resolver-with-lb",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "resolver-with-lb", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
	}
}

func TestRoutesFromSnapshot(t *testing.T) {
	// TODO: we should move all of these to TestAllResourcesFromSnapshot
	// eventually to test all of the xDS types at once with the same input,
	// just as it would be triggered by our xDS server.
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []routeTestCase{
		// TODO(rb): test match stanza skipped for grpc
		// Start ingress gateway test cases
		{
			name: "ingress-config-entry-nil",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway_NilConfigEntry(t)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-defaults-no-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp",
					"default", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"external-sni", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"splitter-with-resolver-redirect-multidc", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-splitter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"chain-and-splitter", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-grpc-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"grpc-router", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"chain-and-router", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"lb-resolver", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-http-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_HTTPMultipleServices,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-grpc-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_GRPCMultipleServices,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-router-header-manip",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "router-header-manip", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-listener-level",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-listener-level", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-listener-level-wildcard",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-listener-level-wildcard", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-service-level",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-service-level", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-service-level-mixed-tls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-service-level-mixed-tls", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-lb-config",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayLBConfig,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "api-gateway-with-multiple-hostnames",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "http",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
							Hostname: "*.example.com",
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "http",
							Routes: []structs.ResourceReference{
								{Kind: structs.HTTPRoute, Name: "backend-route"},
								{Kind: structs.HTTPRoute, Name: "frontend-route"},
								{Kind: structs.HTTPRoute, Name: "generic-route"},
							}},
					}
				},
					[]structs.BoundRoute{
						&structs.HTTPRouteConfigEntry{
							Kind:      structs.HTTPRoute,
							Name:      "backend-route",
							Hostnames: []string{"backend.example.com"},
							Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{Services: []structs.HTTPService{{Name: "backend"}}},
							},
						},
						&structs.HTTPRouteConfigEntry{
							Kind:      structs.HTTPRoute,
							Name:      "frontend-route",
							Hostnames: []string{"frontend.example.com"},
							Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{Services: []structs.HTTPService{{Name: "frontend"}}},
							},
						},
						&structs.HTTPRouteConfigEntry{
							Kind:    structs.HTTPRoute,
							Name:    "generic-route",
							Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{
									Matches:  []structs.HTTPMatch{{Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/frontend"}}},
									Services: []structs.HTTPService{{Name: "frontend"}},
								},
								{
									Matches:  []structs.HTTPMatch{{Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/backend"}}},
									Services: []structs.HTTPService{{Name: "backend"}},
								},
							},
						},
					}, nil, nil)
			},
		},
	}

	tests = append(tests, makeRouteDiscoChainTests(false)...)

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

					routes, err := g.routesFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(routes, func(i, j int) bool {
						return routes[i].(*envoy_route_v3.RouteConfiguration).Name < routes[j].(*envoy_route_v3.RouteConfiguration).Name
					})
					r, err := response.CreateResponse(xdscommon.RouteType, "00000001", "00000001", routes)
					require.NoError(t, err)

					t.Run("current-xdsv1", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("routes", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})

					if tt.alsoRunTestForV2 {
						generator := xdsv2.NewResourceGenerator(testutil.Logger(t))

						converter := proxystateconverter.NewConverter(testutil.Logger(t), &mockCfgFetcher{addressLan: "10.10.10.10"})
						proxyState, err := converter.ProxyStateFromSnapshot(snap)
						require.NoError(t, err)

						res, err := generator.AllResourcesFromIR(proxyState)
						require.NoError(t, err)

						routes = res[xdscommon.RouteType]
						// The order of routes returned via RDS isn't relevant, so it's safe
						// to sort these for the purposes of test comparisons.
						sort.Slice(routes, func(i, j int) bool {
							return routes[i].(*envoy_route_v3.Route).Name < routes[j].(*envoy_route_v3.Route).Name
						})

						r, err := response.CreateResponse(xdscommon.RouteType, "00000001", "00000001", routes)
						require.NoError(t, err)

						t.Run("current-xdsv2", func(t *testing.T) {
							gotJSON := protoToJSON(t, r)

							gName := tt.name
							if tt.overrideGoldenName != "" {
								gName = tt.overrideGoldenName
							}

							expectedJSON := goldenEnvoy(t, filepath.Join("routes", gName), envoyVersion, latestEnvoyVersion, gotJSON)
							require.JSONEq(t, expectedJSON, gotJSON)
						})
					}
				})
			}
		})
	}
}

func TestEnvoyLBConfig_InjectToRouteAction(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected *envoy_route_v3.RouteAction
	}{
		{
			name: "empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			// we only modify route actions for hash-based LB policies
			expected: &envoy_route_v3.RouteAction{},
		},
		{
			name: "least request",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyLeastRequest,
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
			// we only modify route actions for hash-based LB policies
			expected: &envoy_route_v3.RouteAction{},
		},
		{
			name: "headers",
			lb: &structs.LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: &structs.RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyHeader,
						FieldValue: "x-route-key",
						Terminal:   true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Header_{
							Header: &envoy_route_v3.RouteAction_HashPolicy_Header{
								HeaderName: "x-route-key",
							},
						},
						Terminal: true,
					},
				},
			},
		},
		{
			name: "cookies",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "red-velvet",
						Terminal:   true,
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "red-velvet",
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
							},
						},
					},
				},
			},
		},
		{
			name: "non-zero session ttl gets zeroed out",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							TTL:     10 * time.Second,
							Session: true,
						},
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Ttl:  durationpb.New(0 * time.Second),
							},
						},
					},
				},
			},
		},
		{
			name: "zero value ttl omitted if not session cookie",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							Path: "/oven",
						},
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Path: "/oven",
								Ttl:  nil,
							},
						},
					},
				},
			},
		},
		{
			name: "source addr",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
						Terminal: true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
							ConnectionProperties: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties{
								SourceIp: true,
							},
						},
						Terminal: true,
					},
				},
			},
		},
		{
			name: "kitchen sink",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
						Terminal: true,
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							TTL:  10 * time.Second,
							Path: "/oven",
						},
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "chocolate-chip",
						CookieConfig: &structs.CookieConfig{
							Session: true,
							Path:    "/oven",
						},
					},
					{
						Field:      structs.HashPolicyHeader,
						FieldValue: "special-header",
						Terminal:   true,
					},
					{
						Field:      structs.HashPolicyQueryParam,
						FieldValue: "my-pretty-param",
						Terminal:   true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
							ConnectionProperties: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties{
								SourceIp: true,
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Ttl:  durationpb.New(10 * time.Second),
								Path: "/oven",
							},
						},
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "chocolate-chip",
								Ttl:  durationpb.New(0 * time.Second),
								Path: "/oven",
							},
						},
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Header_{
							Header: &envoy_route_v3.RouteAction_HashPolicy_Header{
								HeaderName: "special-header",
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter_{
							QueryParameter: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter{
								Name: "my-pretty-param",
							},
						},
						Terminal: true,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ra envoy_route_v3.RouteAction
			err := injectLBToRouteAction(tc.lb, &ra)
			require.NoError(t, err)

			require.Equal(t, tc.expected, &ra)
		})
	}
}
