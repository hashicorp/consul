package xds

import (
	"path/filepath"
	"sort"
	"testing"
	"time"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/golang/protobuf/ptypes"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestRoutesFromSnapshot(t *testing.T) {

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
			name:   "defaults-no-chain",
			create: proxycfg.TestConfigSnapshot,
			setup:  nil, // Default snapshot
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
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-splitter",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithSplitter,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-grpc-router",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithGRPCRouter,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-router",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithRouter,
			setup:  nil,
		},
		{
			name:   "connect-proxy-lb-in-resolver",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithLB,
			setup:  nil,
		},
		// TODO(rb): test match stanza skipped for grpc
		// Start ingress gateway test cases
		{
			name:   "ingress-defaults-no-chain",
			create: proxycfg.TestConfigSnapshotIngressGateway,
			setup:  nil, // Default snapshot
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
			name:   "ingress-splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-splitter",
			create: proxycfg.TestConfigSnapshotIngressWithSplitter,
			setup:  nil,
		},
		{
			name:   "ingress-with-grpc-router",
			create: proxycfg.TestConfigSnapshotIngressWithGRPCRouter,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-router",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup:  nil,
		},
		{
			name:   "ingress-lb-in-resolver",
			create: proxycfg.TestConfigSnapshotIngressWithLB,
			setup:  nil,
		},
		{
			name:   "ingress-http-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_HTTPMultipleServices,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.IngressGateway.Upstreams = map[proxycfg.IngressListenerKey]structs.Upstreams{
					{Protocol: "http", Port: 8080}: {
						{
							DestinationName: "foo",
							LocalBindPort:   8080,
							IngressHosts: []string{
								"test1.example.com",
								"test2.example.com",
								"test2.example.com:8080",
							},
						},
						{
							DestinationName: "bar",
							LocalBindPort:   8080,
						},
					},
					{Protocol: "http", Port: 443}: {
						{
							DestinationName: "baz",
							LocalBindPort:   443,
						},
						{
							DestinationName: "qux",
							LocalBindPort:   443,
						},
					},
				}
				snap.IngressGateway.Listeners = map[proxycfg.IngressListenerKey]structs.IngressListener{
					{Protocol: "http", Port: 8080}: {
						Port: 8080,
						Services: []structs.IngressService{
							{
								Name: "foo",
							},
							{
								Name: "bar",
							},
						},
					},
					{Protocol: "http", Port: 443}: {
						Port: 443,
						Services: []structs.IngressService{
							{
								Name: "baz",
							},
							{
								Name: "qux",
							},
						},
					},
				}

				// We do not add baz/qux here so that we test the chain.IsDefault() case
				entries := []structs.ConfigEntry{
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "http",
						},
					},
					&structs.ServiceResolverConfigEntry{
						Kind:           structs.ServiceResolver,
						Name:           "foo",
						ConnectTimeout: 22 * time.Second,
					},
					&structs.ServiceResolverConfigEntry{
						Kind:           structs.ServiceResolver,
						Name:           "bar",
						ConnectTimeout: 22 * time.Second,
					},
				}
				fooChain := discoverychain.TestCompileConfigEntries(t, "foo", "default", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil, entries...)
				barChain := discoverychain.TestCompileConfigEntries(t, "bar", "default", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil, entries...)
				bazChain := discoverychain.TestCompileConfigEntries(t, "baz", "default", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil, entries...)
				quxChain := discoverychain.TestCompileConfigEntries(t, "qux", "default", "default", "dc1", connect.TestClusterID+".consul", "dc1", nil, entries...)

				snap.IngressGateway.DiscoveryChain = map[string]*structs.CompiledDiscoveryChain{
					"foo": fooChain,
					"bar": barChain,
					"baz": bazChain,
					"qux": quxChain,
				}
			},
		},
		{
			name:   "ingress-with-chain-and-router-header-manip",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				k := proxycfg.IngressListenerKey{Port: 9191, Protocol: "http"}
				l := snap.IngressGateway.Listeners[k]
				l.Services[0].RequestHeaders = &structs.HTTPHeaderModifiers{
					Add: map[string]string{
						"foo": "bar",
					},
					Set: map[string]string{
						"bar": "baz",
					},
					Remove: []string{"qux"},
				}
				l.Services[0].ResponseHeaders = &structs.HTTPHeaderModifiers{
					Add: map[string]string{
						"foo": "bar",
					},
					Set: map[string]string{
						"bar": "baz",
					},
					Remove: []string{"qux"},
				}
				snap.IngressGateway.Listeners[k] = l
			},
		},
		{
			name:   "ingress-with-sds-listener-level",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup: setupIngressWithTwoHTTPServices(t, ingressSDSOpts{
				// Listener-level SDS means all services share the default route.
				listenerSDS: true,
			}),
		},
		{
			name:   "ingress-with-sds-listener-level-wildcard",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup: setupIngressWithTwoHTTPServices(t, ingressSDSOpts{
				// Listener-level SDS means all services share the default route.
				listenerSDS: true,
				wildcard:    true,
			}),
		},
		{
			name:   "ingress-with-sds-service-level",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup: setupIngressWithTwoHTTPServices(t, ingressSDSOpts{
				listenerSDS: false,
				// Services should get separate routes and no default since they all
				// have custom certs.
				webSDS: true,
				fooSDS: true,
			}),
		},
		{
			name:   "ingress-with-sds-service-level-mixed-tls",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup: setupIngressWithTwoHTTPServices(t, ingressSDSOpts{
				listenerSDS: false,
				// Web needs a separate route as it has custom filter chain but foo
				// should use default route for listener.
				webSDS: true,
				fooSDS: false,
			}),
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
							HashPolicies: []structs.HashPolicy{
								{
									Field:      structs.HashPolicyCookie,
									FieldValue: "chocolate-chip",
									Terminal:   true,
								},
								{
									Field:      structs.HashPolicyHeader,
									FieldValue: "x-user-id",
								},
								{
									SourceIP: true,
									Terminal: true,
								},
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
	}

	latestEnvoyVersion := proxysupport.EnvoyVersions[0]
	latestEnvoyVersion_v2 := proxysupport.EnvoyVersionsV2[0]
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

					if tt.setup != nil {
						tt.setup(snap)
					}

					g := newResourceGenerator(testutil.Logger(t), nil, nil, false)
					g.ProxyFeatures = sf

					routes, err := g.routesFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(routes, func(i, j int) bool {
						return routes[i].(*envoy_route_v3.RouteConfiguration).Name < routes[j].(*envoy_route_v3.RouteConfiguration).Name
					})
					r, err := createResponse(RouteType, "00000001", "00000001", routes)
					require.NoError(t, err)

					t.Run("current", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("routes", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})

					t.Run("v2-compat", func(t *testing.T) {
						if !stringslice.Contains(proxysupport.EnvoyVersionsV2, envoyVersion) {
							t.Skip()
						}
						respV2, err := convertDiscoveryResponseToV2(r)
						require.NoError(t, err)

						gotJSON := protoToJSON(t, respV2)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						gName += ".v2compat"

						// It's easy to miss a new type that encodes a version from just
						// looking at the golden files so lets make it an error here. If
						// there are ever false positives we can maybe include an allow list
						// here as it seems safer to assume something was missed than to
						// assume we'll notice the golden file being wrong. Note the first
						// one matches both resourceApiVersion and transportApiVersion. I
						// left it as a suffix in case there are other field names that
						// follow that convention now or in the future.
						require.NotContains(t, gotJSON, `ApiVersion": "V3"`)
						require.NotContains(t, gotJSON, `type.googleapis.com/envoy.api.v3`)

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("routes", gName), envoyVersion, latestEnvoyVersion_v2, gotJSON), gotJSON)
					})
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
								Ttl:  ptypes.DurationProto(0 * time.Second),
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
								Ttl:  ptypes.DurationProto(10 * time.Second),
								Path: "/oven",
							},
						},
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "chocolate-chip",
								Ttl:  ptypes.DurationProto(0 * time.Second),
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

type ingressSDSOpts struct {
	listenerSDS, webSDS, fooSDS, wildcard bool
	entMetas                              map[string]*structs.EnterpriseMeta
}

// setupIngressWithTwoHTTPServices can be used with
// proxycfg.TestConfigSnapshotIngressWithRouter to generate a setup func for an
// ingress listener with multiple HTTP services and varying SDS configurations
// since those affect how we generate routes.
func setupIngressWithTwoHTTPServices(t *testing.T, o ingressSDSOpts) func(snap *proxycfg.ConfigSnapshot) {
	return func(snap *proxycfg.ConfigSnapshot) {

		snap.IngressGateway.TLSConfig.SDS = nil

		webUpstream := structs.Upstream{
			DestinationName: "web",
			// We use empty not default here because of the way upstream identifiers
			// vary between OSS and Enterprise currently causing test conflicts. In
			// real life `proxycfg` always sets ingress upstream namespaces to
			// `NamespaceOrDefault` which shouldn't matter because we should be
			// consistent within a single binary it's just inconvenient if OSS and
			// enterprise tests generate different output.
			DestinationNamespace: o.entMetas["web"].NamespaceOrEmpty(),
			DestinationPartition: o.entMetas["web"].PartitionOrEmpty(),
			LocalBindPort:        9191,
			IngressHosts: []string{
				"www.example.com",
			},
		}
		fooUpstream := structs.Upstream{
			DestinationName:      "foo",
			DestinationNamespace: o.entMetas["foo"].NamespaceOrEmpty(),
			DestinationPartition: o.entMetas["foo"].PartitionOrEmpty(),
			LocalBindPort:        9191,
			IngressHosts: []string{
				"foo.example.com",
			},
		}

		// Setup additional HTTP service on same listener with default router
		snap.IngressGateway.Upstreams = map[proxycfg.IngressListenerKey]structs.Upstreams{
			{Protocol: "http", Port: 9191}: {webUpstream, fooUpstream},
		}
		il := structs.IngressListener{
			Port: 9191,
			Services: []structs.IngressService{
				{
					Name:  "web",
					Hosts: []string{"www.example.com"},
				},
				{
					Name:  "foo",
					Hosts: []string{"foo.example.com"},
				},
			},
		}
		for i, svc := range il.Services {
			if em, ok := o.entMetas[svc.Name]; ok && em != nil {
				il.Services[i].EnterpriseMeta = *em
			}
		}

		// Now set the appropriate SDS configs
		if o.listenerSDS {
			il.TLS = &structs.GatewayTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "listener-cluster",
					CertResource: "listener-cert",
				},
			}
		}
		if o.webSDS {
			il.Services[0].TLS = &structs.GatewayServiceTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "web-cluster",
					CertResource: "www-cert",
				},
			}
		}
		if o.fooSDS {
			il.Services[1].TLS = &structs.GatewayServiceTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "foo-cluster",
					CertResource: "foo-cert",
				},
			}
		}

		if o.wildcard {
			// undo all that and set just a single wildcard config with no TLS to test
			// the lookup path where we have to compare an actual resolved upstream to
			// a wildcard config.
			il.Services = []structs.IngressService{
				{
					Name: "*",
				},
			}
			// We also don't support user-specified hosts with wildcard so remove
			// those from the upstreams.
			ups := snap.IngressGateway.Upstreams[proxycfg.IngressListenerKey{Protocol: "http", Port: 9191}]
			for i := range ups {
				ups[i].IngressHosts = nil
			}
			snap.IngressGateway.Upstreams[proxycfg.IngressListenerKey{Protocol: "http", Port: 9191}] = ups
		}

		snap.IngressGateway.Listeners[proxycfg.IngressListenerKey{Protocol: "http", Port: 9191}] = il

		entries := []structs.ConfigEntry{
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "web",
				ConnectTimeout: 22 * time.Second,
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "foo",
				ConnectTimeout: 22 * time.Second,
			},
		}
		for i, e := range entries {
			switch v := e.(type) {
			// Add other Service types here if we ever need them above
			case *structs.ServiceResolverConfigEntry:
				if em, ok := o.entMetas[v.Name]; ok && em != nil {
					v.EnterpriseMeta = *em
					entries[i] = v
				}
			}
		}

		webChain := discoverychain.TestCompileConfigEntries(t, "web",
			o.entMetas["web"].NamespaceOrDefault(), "dc1",
			connect.TestClusterID+".consul", "dc1", nil, entries...)
		fooChain := discoverychain.TestCompileConfigEntries(t, "foo",
			o.entMetas["foo"].NamespaceOrDefault(), "dc1",
			connect.TestClusterID+".consul", "dc1", nil, entries...)

		snap.IngressGateway.DiscoveryChain[webUpstream.Identifier()] = webChain
		snap.IngressGateway.DiscoveryChain[fooUpstream.Identifier()] = fooChain
	}
}
