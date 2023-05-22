package xds

import (
	"path/filepath"
	"sort"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"

	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

var testTypeUrlToPrettyName = map[string]string{
	xdscommon.ListenerType: "listeners",
	xdscommon.RouteType:    "routes",
	xdscommon.ClusterType:  "clusters",
	xdscommon.EndpointType: "endpoints",
	xdscommon.SecretType:   "secrets",
}

type goldenTestCase struct {
	name   string
	create func(t testinf.T) *proxycfg.ConfigSnapshot
	// Setup is called before the test starts. It is passed the snapshot from
	// TestConfigSnapshot and is allowed to modify it in any way to setup the
	// test input.
	setup              func(snap *proxycfg.ConfigSnapshot)
	overrideGoldenName string
	generatorSetup     func(*ResourceGenerator)
}

func TestAllResourcesFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type testcase = goldenTestCase

	run := func(
		t *testing.T,
		sf xdscommon.SupportedProxyFeatures,
		envoyVersion string,
		latestEnvoyVersion string,
		tt testcase,
	) {
		// Sanity check default with no overrides first
		snap := tt.create(t)

		// TODO: it would be nice to be able to ensure these snapshots are always valid before we use them in a test.
		// require.True(t, snap.Valid())

		// We need to replace the TLS certs with deterministic ones to make golden
		// files workable. Note we don't update these otherwise they'd change
		// golden files for every test case and so not be any use!
		testcommon.SetupTLSRootsAndLeaf(t, snap)

		if tt.setup != nil {
			tt.setup(snap)
		}

		// Need server just for logger dependency
		g := NewResourceGenerator(testutil.Logger(t), nil, false)
		g.ProxyFeatures = sf
		if tt.generatorSetup != nil {
			tt.generatorSetup(g)
		}

		resources, err := g.AllResourcesFromSnapshot(snap)
		require.NoError(t, err)

		typeUrls := []string{
			xdscommon.ListenerType,
			xdscommon.RouteType,
			xdscommon.ClusterType,
			xdscommon.EndpointType,
			xdscommon.SecretType,
		}
		require.Len(t, resources, len(typeUrls))

		for _, typeUrl := range typeUrls {
			prettyName := testTypeUrlToPrettyName[typeUrl]
			t.Run(prettyName, func(t *testing.T) {
				items, ok := resources[typeUrl]
				require.True(t, ok)

				sort.Slice(items, func(i, j int) bool {
					switch typeUrl {
					case xdscommon.ListenerType:
						return items[i].(*envoy_listener_v3.Listener).Name < items[j].(*envoy_listener_v3.Listener).Name
					case xdscommon.RouteType:
						return items[i].(*envoy_route_v3.RouteConfiguration).Name < items[j].(*envoy_route_v3.RouteConfiguration).Name
					case xdscommon.ClusterType:
						return items[i].(*envoy_cluster_v3.Cluster).Name < items[j].(*envoy_cluster_v3.Cluster).Name
					case xdscommon.EndpointType:
						return items[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < items[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
					case xdscommon.SecretType:
						return items[i].(*envoy_tls_v3.Secret).Name < items[j].(*envoy_tls_v3.Secret).Name
					default:
						panic("not possible")
					}
				})

				r, err := createResponse(typeUrl, "00000001", "00000001", items)
				require.NoError(t, err)

				gotJSON := protoToJSON(t, r)

				gName := tt.name
				if tt.overrideGoldenName != "" {
					gName = tt.overrideGoldenName
				}

				expectedJSON := goldenEnvoy(t, filepath.Join(prettyName, gName), envoyVersion, latestEnvoyVersion, gotJSON)
				require.JSONEq(t, expectedJSON, gotJSON)
			})
		}
	}

	tests := []testcase{
		{
			name: "defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, nil)
			},
		},
		{
			name: "connect-proxy-exported-to-peers",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This test is only concerned about the SPIFFE cert validator config in the public listener
					// so we empty out the upstreams to avoid generating unnecessary upstream listeners.
					ns.Proxy.Upstreams = structs.Upstreams{}
				}, []proxycfg.UpdateEvent{
					{
						CorrelationID: "peering-trust-bundles",
						Result:        proxycfg.TestPeerTrustBundles(t),
					},
				})
			},
		},
		{
			name:   "transparent-proxy",
			create: proxycfg.TestConfigSnapshotTransparentProxy,
		},
		{
			name:   "connect-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeering,
		},
		{
			name:   "transparent-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeeringTProxy,
		},
		{
			name:   "local-mesh-gateway-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeeringLocalMeshGateway,
		},
		{
			name:   "telemetry-collector",
			create: proxycfg.TestConfigSnapshotTelemetryCollector,
		},
	}
	tests = append(tests, getConnectProxyTransparentProxyGoldenTestCases()...)
	tests = append(tests, getMeshGatewayPeeringGoldenTestCases()...)
	tests = append(tests, getTrafficControlPeeringGoldenTestCases(false)...)
	tests = append(tests, getEnterpriseGoldenTestCases()...)
	tests = append(tests, getAPIGatewayGoldenTestCases(t)...)

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					run(t, sf, envoyVersion, latestEnvoyVersion, tt)
				})
			}
		})
	}
}

func getConnectProxyTransparentProxyGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name:   "transparent-proxy-destination",
			create: proxycfg.TestConfigSnapshotTransparentProxyDestination,
		},
		{
			name:   "transparent-proxy-destination-http",
			create: proxycfg.TestConfigSnapshotTransparentProxyDestinationHTTP,
		},
		{
			name: "transparent-proxy-terminating-gateway-destinations-only",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGatewayDestinations(t, true, nil)
			},
		},
	}
}

func getMeshGatewayPeeringGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "mesh-gateway-with-exported-peered-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-tcp", nil, nil)
			},
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-http", nil, nil)
			},
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http-with-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "chain-and-l7-stuff", nil, nil)
			},
		},
		{
			name: "mesh-gateway-peering-control-plane",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "control-plane", nil, nil)
			},
		},
		{
			name: "mesh-gateway-with-imported-peered-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "imported-services", nil, nil)
			},
		},
		{
			name: "mesh-gateway-with-peer-through-mesh-gateway-enabled",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "peer-through-mesh-gateway", nil, nil)
			},
		},
	}
}

func getTrafficControlPeeringGoldenTestCases(enterprise bool) []goldenTestCase {
	cases := []goldenTestCase{
		{
			name: "connect-proxy-with-chain-and-failover-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-to-cluster-peer", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-and-redirect-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-cluster-peer", enterprise, nil, nil)
			},
		},
	}

	if enterprise {
		for i := range cases {
			cases[i].name = "enterprise-" + cases[i].name
		}
	}

	return cases
}

const (
	gatewayTestPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAx95Opa6t4lGEpiTUogEBptqOdam2ch4BHQGhNhX/MrDwwuZQ
httBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2jQlhqTodElkbsd5vWY8R/bxJWQSo
NvVE12TlzECxGpJEiHt4W0r8pGffk+rvpljiUyCfnT1kGF3znOSjK1hRMTn6RKWC
yYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409g9X5VU88/Bmmrz4cMyxce86Kc2ug
5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftrXOvuCbO5IBRHMOBHiHTZ4rtGuhMa
Ir21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+WmQIDAQABAoIBACYvceUzp2MK4gYA
GWPOP2uKbBdM0l+hHeNV0WAM+dHMfmMuL4pkT36ucqt0ySOLjw6rQyOZG5nmA6t9
sv0g4ae2eCMlyDIeNi1Yavu4Wt6YX4cTXbQKThm83C6W2X9THKbauBbxD621bsDK
7PhiGPN60yPue7YwFQAPqqD4YaK+s22HFIzk9gwM/rkvAUNwRv7SyHMiFe4Igc1C
Eev7iHWzvj5Heoz6XfF+XNF9DU+TieSUAdjd56VyUb8XL4+uBTOhHwLiXvAmfaMR
HvpcxeKnYZusS6NaOxcUHiJnsLNWrxmJj9WEGgQzuLxcLjTe4vVmELVZD8t3QUKj
PAxu8tUCgYEA7KIWVn9dfVpokReorFym+J8FzLwSktP9RZYEMonJo00i8aii3K9s
u/aSwRWQSCzmON1ZcxZzWhwQF9usz6kGCk//9+4hlVW90GtNK0RD+j7sp4aT2JI8
9eLEjTG+xSXa7XWe98QncjjL9lu/yrRncSTxHs13q/XP198nn2aYuQ8CgYEA2Dnt
sRBzv0fFEvzzFv7G/5f85mouN38TUYvxNRTjBLCXl9DeKjDkOVZ2b6qlfQnYXIru
H+W+v+AZEb6fySXc8FRab7lkgTMrwE+aeI4rkW7asVwtclv01QJ5wMnyT84AgDD/
Dgt/RThFaHgtU9TW5GOZveL+l9fVPn7vKFdTJdcCgYEArJ99zjHxwJ1whNAOk1av
09UmRPm6TvRo4heTDk8oEoIWCNatoHI0z1YMLuENNSnT9Q280FFDayvnrY/qnD7A
kktT/sjwJOG8q8trKzIMqQS4XWm2dxoPcIyyOBJfCbEY6XuRsUuePxwh5qF942EB
yS9a2s6nC4Ix0lgPrqAIr48CgYBgS/Q6riwOXSU8nqCYdiEkBYlhCJrKpnJxF9T1
ofa0yPzKZP/8ZEfP7VzTwHjxJehQ1qLUW9pG08P2biH1UEKEWdzo8vT6wVJT1F/k
HtTycR8+a+Hlk2SHVRHqNUYQGpuIe8mrdJ1as4Pd0d/F/P0zO9Rlh+mAsGPM8HUM
T0+9gwKBgHDoerX7NTskg0H0t8O+iSMevdxpEWp34ZYa9gHiftTQGyrRgERCa7Gj
nZPAxKb2JoWyfnu3v7G5gZ8fhDFsiOxLbZv6UZJBbUIh1MjJISpXrForDrC2QNLX
kHrHfwBFDB3KMudhQknsJzEJKCL/KmFH6o0MvsoaT9yzEl3K+ah/
-----END RSA PRIVATE KEY-----`
	gatewayTestCertificate = `-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCQMDsYO8FrPjANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJV
UzAeFw0yMjEyMjAxNzUwMjVaFw0yNzEyMTkxNzUwMjVaMA0xCzAJBgNVBAYTAlVT
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAx95Opa6t4lGEpiTUogEB
ptqOdam2ch4BHQGhNhX/MrDwwuZQhttBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2
jQlhqTodElkbsd5vWY8R/bxJWQSoNvVE12TlzECxGpJEiHt4W0r8pGffk+rvplji
UyCfnT1kGF3znOSjK1hRMTn6RKWCyYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409
g9X5VU88/Bmmrz4cMyxce86Kc2ug5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftr
XOvuCbO5IBRHMOBHiHTZ4rtGuhMaIr21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+W
mQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBfCqoUIdPf/HGSbOorPyZWbyizNtHJ
GL7x9cAeIYxpI5Y/WcO1o5v94lvrgm3FNfJoGKbV66+JxOge731FrfMpHplhar1Z
RahYIzNLRBTLrwadLAZkApUpZvB8qDK4knsTWFYujNsylCww2A6ajzIMFNU4GkUK
NtyHRuD+KYRmjXtyX1yHNqfGN3vOQmwavHq2R8wHYuBSc6LAHHV9vG+j0VsgMELO
qwxn8SmLkSKbf2+MsQVzLCXXN5u+D8Yv+4py+oKP4EQ5aFZuDEx+r/G/31rTthww
AAJAMaoXmoYVdgXV+CPuBb2M4XCpuzLu3bcA2PXm5ipSyIgntMKwXV7r
-----END CERTIFICATE-----`
)

func getAPIGatewayGoldenTestCases(t *testing.T) []goldenTestCase {
	t.Helper()

	service := structs.NewServiceName("service", nil)
	serviceUID := proxycfg.NewUpstreamIDFromServiceName(service)
	serviceChain := discoverychain.TestCompileConfigEntries(t, "service", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

	return []goldenTestCase{
		{
			name: "api-gateway-with-tcp-route-and-inline-certificate",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								}},
							},
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
							Routes: []structs.ResourceReference{{
								Kind: structs.TCPRoute,
								Name: "route",
							}},
						},
					}
				}, []structs.BoundRoute{
					&structs.TCPRouteConfigEntry{
						Kind: structs.TCPRoute,
						Name: "route",
						Services: []structs.TCPService{{
							Name: "service",
						}},
					},
				}, []structs.InlineCertificateConfigEntry{{
					Kind:        structs.InlineCertificate,
					Name:        "certificate",
					PrivateKey:  gatewayTestPrivateKey,
					Certificate: gatewayTestCertificate,
				}}, nil)
			},
		},
		{
			name: "api-gateway-with-http-route-and-inline-certificate",
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
							Routes: []structs.ResourceReference{{
								Kind: structs.HTTPRoute,
								Name: "route",
							}},
						},
					}
				}, []structs.BoundRoute{
					&structs.HTTPRouteConfigEntry{
						Kind: structs.HTTPRoute,
						Name: "route",
						Rules: []structs.HTTPRouteRule{{
							Services: []structs.HTTPService{{
								Name: "service",
							}},
						}},
					},
				}, nil, []proxycfg.UpdateEvent{{
					CorrelationID: "discovery-chain:" + serviceUID.String(),
					Result: &structs.DiscoveryChainResponse{
						Chain: serviceChain,
					},
				}, {
					CorrelationID: "upstream-target:" + serviceChain.ID() + ":" + serviceUID.String(),
					Result: &structs.IndexedCheckServiceNodes{
						Nodes: proxycfg.TestUpstreamNodes(t, "service"),
					},
				}})
			},
		},
	}
}
