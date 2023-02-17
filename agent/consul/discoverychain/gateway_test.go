package discoverychain

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestGatewayChainSynthesizer_AddTCPRoute(t *testing.T) {
	t.Parallel()

	datacenter := "dc1"
	gateway := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "gateway",
	}
	route := structs.TCPRouteConfigEntry{
		Kind: structs.TCPRoute,
		Name: "route",
	}

	expected := GatewayChainSynthesizer{
		datacenter:        datacenter,
		gateway:           gateway,
		trustDomain:       "domain",
		suffix:            "suffix",
		matchesByHostname: map[string][]hostnameMatch{},
		tcpRoutes: []structs.TCPRouteConfigEntry{
			route,
		},
	}

	gatewayChainSynthesizer := NewGatewayChainSynthesizer(datacenter, "domain", "suffix", gateway)

	// Add a TCP route
	gatewayChainSynthesizer.AddTCPRoute(route)

	require.Equal(t, expected, *gatewayChainSynthesizer)
}

func TestGatewayChainSynthesizer_AddHTTPRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		route                     structs.HTTPRouteConfigEntry
		expectedMatchesByHostname map[string][]hostnameMatch
	}{
		"no hostanames": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"*": {},
			},
		},
		"single hostname with no rules": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {},
			},
		},
		"single hostname with a single rule and no matches": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters:  structs.HTTPFilters{},
						Matches:  []structs.HTTPMatch{},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "/",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
		"single hostname with a single rule and a single match": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "foo-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
		"single hostname with a single rule and multiple matches": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "foo-",
								},
							},
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "bar-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "bar-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
		"multiple hostnames with a single rule and a single match": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
					"example.net",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "foo-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
				"example.net": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
		"multiple hostnames with a single rule and multiple matches": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
					"example.net",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "foo-",
								},
							},
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "bar-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "bar-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
				"example.net": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "bar-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
		"multiple hostnames with multiple rules and multiple matches": {
			route: structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "route",
				Hostnames: []string{
					"example.com",
					"example.net",
				},
				Rules: []structs.HTTPRouteRule{
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "foo-",
								},
							},
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "bar-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
					{
						Filters: structs.HTTPFilters{},
						Matches: []structs.HTTPMatch{
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "baz-",
								},
							},
							{
								Path: structs.HTTPPathMatch{
									Match: "prefix",
									Value: "qux-",
								},
							},
						},
						Services: []structs.HTTPService{},
					},
				},
			},
			expectedMatchesByHostname: map[string][]hostnameMatch{
				"example.com": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "bar-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "baz-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "qux-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
				"example.net": {
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "foo-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "bar-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "baz-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
					{
						match: structs.HTTPMatch{
							Path: structs.HTTPPathMatch{
								Match: "prefix",
								Value: "qux-",
							},
						},
						filters:  structs.HTTPFilters{},
						services: []structs.HTTPService{},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			datacenter := "dc1"
			gateway := &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			}

			gatewayChainSynthesizer := NewGatewayChainSynthesizer(datacenter, "domain", "suffix", gateway)

			gatewayChainSynthesizer.SetHostname("*")
			gatewayChainSynthesizer.AddHTTPRoute(tc.route)

			require.Equal(t, tc.expectedMatchesByHostname, gatewayChainSynthesizer.matchesByHostname)
		})
	}
}

func TestGatewayChainSynthesizer_Synthesize(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		synthesizer             *GatewayChainSynthesizer
		tcpRoutes               []*structs.TCPRouteConfigEntry
		httpRoutes              []*structs.HTTPRouteConfigEntry
		chain                   *structs.CompiledDiscoveryChain
		extra                   []*structs.CompiledDiscoveryChain
		expectedIngressServices []structs.IngressService
		expectedDiscoveryChains []*structs.CompiledDiscoveryChain
	}{
		// TODO Add tests for other synthesizer types.
		"TCPRoute-based listener": {
			synthesizer: NewGatewayChainSynthesizer("dc1", "domain", "suffix", &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			}),
			tcpRoutes: []*structs.TCPRouteConfigEntry{
				{
					Kind: structs.TCPRoute,
					Name: "tcp-route",
				},
			},
			chain: &structs.CompiledDiscoveryChain{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			},
			extra:                   []*structs.CompiledDiscoveryChain{},
			expectedIngressServices: []structs.IngressService{},
			expectedDiscoveryChains: []*structs.CompiledDiscoveryChain{{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			}},
		},
		"HTTPRoute-based listener": {
			synthesizer: NewGatewayChainSynthesizer("dc1", "domain", "suffix", &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			}),
			httpRoutes: []*structs.HTTPRouteConfigEntry{
				{
					Kind: structs.HTTPRoute,
					Name: "http-route",
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name: "foo",
						}},
					}},
				},
			},
			chain: &structs.CompiledDiscoveryChain{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			},
			extra: []*structs.CompiledDiscoveryChain{},
			expectedIngressServices: []structs.IngressService{{
				Name:  "gateway-suffix-9b9265b",
				Hosts: []string{"*"},
			}},
			expectedDiscoveryChains: []*structs.CompiledDiscoveryChain{{
				ServiceName: "gateway-suffix-9b9265b",
				Partition:   "default",
				Namespace:   "default",
				Datacenter:  "dc1",
				Protocol:    "http",
				StartNode:   "router:gateway-suffix-9b9265b.default.default",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:gateway-suffix-9b9265b.default.default.dc1": {
						Type: "resolver",
						Name: "gateway-suffix-9b9265b.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "gateway-suffix-9b9265b.default.default.dc1",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
					"router:gateway-suffix-9b9265b.default.default": {
						Type: "router",
						Name: "gateway-suffix-9b9265b.default.default",
						Routes: []*structs.DiscoveryRoute{{
							Definition: &structs.ServiceRoute{
								Match: &structs.ServiceRouteMatch{
									HTTP: &structs.ServiceRouteHTTPMatch{
										PathPrefix: "/",
									},
								},
								Destination: &structs.ServiceRouteDestination{
									Service:   "foo",
									Partition: "default",
									Namespace: "default",
									RequestHeaders: &structs.HTTPHeaderModifiers{
										Add: make(map[string]string),
										Set: make(map[string]string),
									},
								},
							},
							NextNode: "resolver:foo.default.default.dc1",
						}, {
							Definition: &structs.ServiceRoute{
								Match: &structs.ServiceRouteMatch{
									HTTP: &structs.ServiceRouteHTTPMatch{
										PathPrefix: "/",
									},
								},
								Destination: &structs.ServiceRouteDestination{
									Service:   "gateway-suffix-9b9265b",
									Partition: "default",
									Namespace: "default",
								},
							},
							NextNode: "resolver:gateway-suffix-9b9265b.default.default.dc1",
						}},
					},
					"resolver:foo.default.default.dc1": {
						Type: "resolver",
						Name: "foo.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "foo.default.default.dc1",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"gateway-suffix-9b9265b.default.default.dc1": {
						ID:             "gateway-suffix-9b9265b.default.default.dc1",
						Service:        "gateway-suffix-9b9265b",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "gateway-suffix-9b9265b.default.dc1.internal.domain",
						Name:           "gateway-suffix-9b9265b.default.dc1.internal.domain",
					},
					"foo.default.default.dc1": {
						ID:             "foo.default.default.dc1",
						Service:        "foo",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "foo.default.dc1.internal.domain",
						Name:           "foo.default.dc1.internal.domain",
					},
				},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.synthesizer.SetHostname("*")

			for _, tcpRoute := range tc.tcpRoutes {
				tc.synthesizer.AddTCPRoute(*tcpRoute)
			}
			for _, httpRoute := range tc.httpRoutes {
				tc.synthesizer.AddHTTPRoute(*httpRoute)
			}

			chains := append([]*structs.CompiledDiscoveryChain{tc.chain}, tc.extra...)
			ingressServices, discoveryChains, err := tc.synthesizer.Synthesize(chains...)

			require.NoError(t, err)
			require.Equal(t, tc.expectedIngressServices, ingressServices)
			require.Equal(t, tc.expectedDiscoveryChains, discoveryChains)
		})
	}
}
