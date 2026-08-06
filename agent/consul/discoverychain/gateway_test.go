// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
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
		"no hostnames": {
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
						Filters: structs.HTTPFilters{
							Headers: []structs.HTTPHeaderFilter{
								{
									Add:    map[string]string{"add me to the rule request": "present"},
									Set:    map[string]string{"set me on the rule request": "present"},
									Remove: []string{"remove me from the rule request"},
								},
								{
									Add: map[string]string{"add me to the rule and service request": "rule"},
									Set: map[string]string{"set me on the rule and service request": "rule"},
								},
								{
									Remove: []string{"remove me from the rule and service request"},
								},
							},
						},
						ResponseFilters: structs.HTTPResponseFilters{
							Headers: []structs.HTTPHeaderFilter{{
								Add: map[string]string{
									"add me to the rule response":             "present",
									"add me to the rule and service response": "rule",
								},
								Set: map[string]string{
									"set me on the rule response":             "present",
									"set me on the rule and service response": "rule",
								},
								Remove: []string{
									"remove me from the rule response",
									"remove me from the rule and service response",
								},
							}},
						},
						Services: []structs.HTTPService{{
							Name: "foo",
							Filters: structs.HTTPFilters{
								Headers: []structs.HTTPHeaderFilter{
									{
										Add: map[string]string{"add me to the service request": "present"},
									},
									{
										Set:    map[string]string{"set me on the service request": "present"},
										Remove: []string{"remove me from the service request"},
									},
									{
										Add:    map[string]string{"add me to the rule and service request": "service"},
										Set:    map[string]string{"set me on the rule and service request": "service"},
										Remove: []string{"remove me from the rule and service request"},
									},
								},
							},
							ResponseFilters: structs.HTTPResponseFilters{
								Headers: []structs.HTTPHeaderFilter{
									{
										Add:    map[string]string{"add me to the service response": "present"},
										Set:    map[string]string{"set me on the service response": "present"},
										Remove: []string{"remove me from the service response"},
									},
									{
										Add:    map[string]string{"add me to the rule and service response": "service"},
										Set:    map[string]string{"set me on the rule and service response": "service"},
										Remove: []string{"remove me from the rule and service response"},
									},
								},
							},
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
										Add: map[string]string{
											"add me to the rule request":             "present",
											"add me to the service request":          "present",
											"add me to the rule and service request": "service",
										},
										Set: map[string]string{
											"set me on the rule request":             "present",
											"set me on the service request":          "present",
											"set me on the rule and service request": "service",
										},
										Remove: []string{
											"remove me from the rule request",
											"remove me from the rule and service request",
											"remove me from the service request",
											"remove me from the rule and service request",
										},
									},
									ResponseHeaders: &structs.HTTPHeaderModifiers{
										Add: map[string]string{
											"add me to the rule response":             "present",
											"add me to the service response":          "present",
											"add me to the rule and service response": "service",
										},
										Set: map[string]string{
											"set me on the rule response":             "present",
											"set me on the service response":          "present",
											"set me on the rule and service response": "service",
										},
										Remove: []string{
											"remove me from the rule response",
											"remove me from the rule and service response",
											"remove me from the service response",
											"remove me from the rule and service response",
										},
									},
								},
							},
							NextNode: "resolver:foo.default.default.dc1",
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
		"HTTPRoute with virtual resolver": {
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
				Partition:   "default",
				Datacenter:  "dc1",
				StartNode:   "resolver:foo-2.default.default.dc2",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:foo-2.default.default.dc2": {
						Type: "resolver",
						Name: "foo-2.default.default.dc2",
						Resolver: &structs.DiscoveryResolver{
							Target:         "foo-2.default.default.dc2",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
				},
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
									ResponseHeaders: &structs.HTTPHeaderModifiers{
										Add: make(map[string]string),
										Set: make(map[string]string),
									},
								},
							},
							NextNode: "resolver:foo-2.default.default.dc2",
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
					"resolver:foo-2.default.default.dc2": {
						Type: "resolver",
						Name: "foo-2.default.default.dc2",
						Resolver: &structs.DiscoveryResolver{
							Target:         "foo-2.default.default.dc2",
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

func TestGatewayChainSynthesizer_ComplexChain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		synthesizer            *GatewayChainSynthesizer
		route                  *structs.HTTPRouteConfigEntry
		entries                []structs.ConfigEntry
		expectedDiscoveryChain *structs.CompiledDiscoveryChain
	}{
		"HTTP-Route with nested splitters": {
			synthesizer: NewGatewayChainSynthesizer("dc1", "domain", "suffix", &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			}),
			route: &structs.HTTPRouteConfigEntry{
				Kind: structs.HTTPRoute,
				Name: "test",
				Rules: []structs.HTTPRouteRule{{
					Services: []structs.HTTPService{{
						Name: "splitter-one",
					}},
				}},
			},
			entries: []structs.ConfigEntry{
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "splitter-one",
					Splits: []structs.ServiceSplit{{
						Service: "service-one",
						Weight:  50,
					}, {
						Service: "splitter-two",
						Weight:  50,
					}},
				},
				&structs.ServiceSplitterConfigEntry{
					Kind: structs.ServiceSplitter,
					Name: "splitter-two",
					Splits: []structs.ServiceSplit{{
						Service: "service-two",
						Weight:  50,
					}, {
						Service: "service-three",
						Weight:  50,
					}},
				},
				&structs.ProxyConfigEntry{
					Kind:     structs.ProxyConfigGlobal,
					Name:     "global",
					Protocol: "http",
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
			},
			expectedDiscoveryChain: &structs.CompiledDiscoveryChain{
				ServiceName: "gateway-suffix-9b9265b",
				Namespace:   "default",
				Partition:   "default",
				Datacenter:  "dc1",
				Protocol:    "http",
				StartNode:   "router:gateway-suffix-9b9265b.default.default",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"resolver:service-one.default.default.dc1": {
						Type: "resolver",
						Name: "service-one.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "service-one.default.default.dc1",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
					"resolver:service-three.default.default.dc1": {
						Type: "resolver",
						Name: "service-three.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "service-three.default.default.dc1",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
					"resolver:service-two.default.default.dc1": {
						Type: "resolver",
						Name: "service-two.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "service-two.default.default.dc1",
							Default:        true,
							ConnectTimeout: 5000000000,
						},
					},
					"resolver:splitter-one.default.default.dc1": {
						Type: "resolver",
						Name: "splitter-one.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target:         "splitter-one.default.default.dc1",
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
									Service:   "splitter-one",
									Partition: "default",
									Namespace: "default",
									RequestHeaders: &structs.HTTPHeaderModifiers{
										Add: make(map[string]string),
										Set: make(map[string]string),
									},
									ResponseHeaders: &structs.HTTPHeaderModifiers{
										Add: make(map[string]string),
										Set: make(map[string]string),
									},
								},
							},
							NextNode: "splitter:splitter-one.default.default",
						}},
					},
					"splitter:splitter-one.default.default": {
						Type: structs.DiscoveryGraphNodeTypeSplitter,
						Name: "splitter-one.default.default",
						Splits: []*structs.DiscoverySplit{{
							Definition: &structs.ServiceSplit{
								Weight:  50,
								Service: "service-one",
							},
							Weight:   50,
							NextNode: "resolver:service-one.default.default.dc1",
						}, {
							Definition: &structs.ServiceSplit{
								Weight:  50,
								Service: "service-two",
							},
							Weight:   25,
							NextNode: "resolver:service-two.default.default.dc1",
						}, {
							Definition: &structs.ServiceSplit{
								Weight:  50,
								Service: "service-three",
							},
							Weight:   25,
							NextNode: "resolver:service-three.default.default.dc1",
						}},
					},
				}, Targets: map[string]*structs.DiscoveryTarget{
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
					"service-one.default.default.dc1": {
						ID:             "service-one.default.default.dc1",
						Service:        "service-one",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "service-one.default.dc1.internal.domain",
						Name:           "service-one.default.dc1.internal.domain",
					},
					"service-three.default.default.dc1": {
						ID:             "service-three.default.default.dc1",
						Service:        "service-three",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "service-three.default.dc1.internal.domain",
						Name:           "service-three.default.dc1.internal.domain",
					},
					"service-two.default.default.dc1": {
						ID:             "service-two.default.default.dc1",
						Service:        "service-two",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "service-two.default.dc1.internal.domain",
						Name:           "service-two.default.dc1.internal.domain",
					},
					"splitter-one.default.default.dc1": {
						ID:             "splitter-one.default.default.dc1",
						Service:        "splitter-one",
						Datacenter:     "dc1",
						Partition:      "default",
						Namespace:      "default",
						ConnectTimeout: 5000000000,
						SNI:            "splitter-one.default.dc1.internal.domain",
						Name:           "splitter-one.default.dc1.internal.domain",
					},
				}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			service := tc.entries[0]
			entries := configentry.NewDiscoveryChainSet()
			entries.AddEntries(tc.entries...)
			compiled, err := Compile(CompileRequest{
				ServiceName:           service.GetName(),
				EvaluateInNamespace:   service.GetEnterpriseMeta().NamespaceOrDefault(),
				EvaluateInPartition:   service.GetEnterpriseMeta().PartitionOrDefault(),
				EvaluateInDatacenter:  "dc1",
				EvaluateInTrustDomain: "domain",
				Entries:               entries,
			})
			require.NoError(t, err)

			tc.synthesizer.SetHostname("*")
			tc.synthesizer.AddHTTPRoute(*tc.route)

			chains := []*structs.CompiledDiscoveryChain{compiled}
			_, discoveryChains, err := tc.synthesizer.Synthesize(chains...)

			require.NoError(t, err)
			require.Len(t, discoveryChains, 1)
			require.Equal(t, tc.expectedDiscoveryChain, discoveryChains[0])
		})
	}
}

// TestSynthesizeHTTPRouteDiscoveryChain_ComposedDestinationProtocol is a
// regression test for the protocol-resolution defect introduced by #12479.
//
// When an HTTPRoute's backend service has its own ServiceRouter, gateway
// synthesis composes that router's routes into the synthetic gateway chain. A
// composed route may point at a *different* downstream service than the one the
// HTTPRoute named directly. That downstream service can be entry-less (no
// service-defaults), relying on the global proxy-defaults protocol fallback.
//
// The synthetic entry set intentionally carries no proxy-defaults, so the
// fallback the real compiler applies is unavailable during synthesis. Without a
// synthetic http default for the composed destination, the synthesized chain
// resolves it as tcp and fails with "uses inconsistent protocols" against the
// http gateway chain — dropping the gateway's entire xDS config batch.
func TestSynthesizeHTTPRouteDiscoveryChain_ComposedDestinationProtocol(t *testing.T) {
	t.Parallel()

	backend := structs.HTTPService{Name: "service-a"}

	// service-a's own ServiceRouter routes to service-b, which has no
	// service-defaults entry of its own.
	serviceRouters := map[structs.ServiceName][]*structs.ServiceRoute{
		backend.ServiceName(): {
			{
				Match: &structs.ServiceRouteMatch{
					HTTP: &structs.ServiceRouteHTTPMatch{PathPrefix: "/b"},
				},
				Destination: &structs.ServiceRouteDestination{
					Service: "service-b",
				},
			},
		},
	}

	route := structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "test-route",
		Rules: []structs.HTTPRouteRule{{
			Matches: []structs.HTTPMatch{{
				Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/"},
			}},
			Services: []structs.HTTPService{backend},
		}},
	}

	_, router, _, defaults := synthesizeHTTPRouteDiscoveryChain(route, serviceRouters)

	// The composition must have produced a route to service-b.
	var composedToB bool
	for _, r := range router.Routes {
		if r.Destination != nil && r.Destination.Service == "service-b" {
			composedToB = true
		}
	}
	require.True(t, composedToB, "expected a composed route to service-b")

	protocols := make(map[string]string)
	for _, d := range defaults {
		protocols[d.Name] = d.Protocol
	}

	require.Equal(t, "http", protocols["service-a"], "direct destination should have an http default")
	require.Equal(t, "http", protocols["service-b"],
		"composed sub-destination must have a synthetic http default so the "+
			"synthesized chain does not resolve it as tcp (regression #12479)")
}

// TestGatewaySynthesis_ProxyDefaultsFallback_StateFaithful reproduces the
// customer's production regression the way proxycfg actually does it: the
// backend service's discovery chain is compiled through the *real* compiler with
// a full entry set (proxy-defaults present) — exactly what watchDiscoveryChain
// stores in snap.APIGateway.DiscoveryChain — and *that* compiled chain is then
// fed to the gateway synthesizer.
//
// This is the faithful model of:
//
//	API gateway (HTTP) --route--> service "x" (has a service-router w/ header
//	routing) --router--> service "y" (entry-less, relies on proxy-defaults=http)
//
// It also answers the two questions raised by the field team's mitigation
// evidence — does proxy-defaults clear the synthesis error? does service-defaults
// on the router destination clear it? — by toggling those entries on the *real*
// (backend) compile side and observing the synthesis outcome.
func TestGatewaySynthesis_ProxyDefaultsFallback_StateFaithful(t *testing.T) {
	t.Parallel()

	proxyDefaultsHTTP := &structs.ProxyConfigEntry{
		Kind:     structs.ProxyDefaults,
		Name:     structs.ProxyConfigGlobal,
		Protocol: "http",
		Config:   map[string]interface{}{"protocol": "http"},
	}

	// service "x" has a real service-router doing header-based routing
	// (X-Forwarded-For), whose destination is service "y".
	xRouter := &structs.ServiceRouterConfigEntry{
		Kind: structs.ServiceRouter,
		Name: "x",
		Routes: []structs.ServiceRoute{{
			Match: &structs.ServiceRouteMatch{
				HTTP: &structs.ServiceRouteHTTPMatch{
					Header: []structs.ServiceRouteHTTPMatchHeader{{
						Name:    "X-Forwarded-For",
						Present: true,
					}},
				},
			},
			Destination: &structs.ServiceRouteDestination{Service: "y"},
		}},
	}

	yServiceDefaultsHTTP := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "y",
		Protocol: "http",
	}

	// compileBackendChain mimics watchDiscoveryChain: compile x's chain via the
	// real compiler against the given entry set.
	compileBackendChain := func(t *testing.T, entries ...structs.ConfigEntry) (*structs.CompiledDiscoveryChain, error) {
		t.Helper()
		set := configentry.NewDiscoveryChainSet()
		set.AddEntries(entries...)
		return Compile(CompileRequest{
			ServiceName:           "x",
			EvaluateInNamespace:   "default",
			EvaluateInPartition:   "default",
			EvaluateInDatacenter:  "dc1",
			EvaluateInTrustDomain: "domain",
			Entries:               set,
		})
	}

	synthesize := func(t *testing.T, backend *structs.CompiledDiscoveryChain) ([]*structs.CompiledDiscoveryChain, error) {
		t.Helper()
		synth := NewGatewayChainSynthesizer("dc1", "domain", "listener", &structs.APIGatewayConfigEntry{
			Kind: structs.APIGateway,
			Name: "gateway",
		})
		synth.SetHostname("*")
		synth.AddHTTPRoute(structs.HTTPRouteConfigEntry{
			Kind: structs.HTTPRoute,
			Name: "route",
			Rules: []structs.HTTPRouteRule{{
				Matches: []structs.HTTPMatch{{
					Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/"},
				}},
				Services: []structs.HTTPService{{Name: "x"}},
			}},
		})
		_, chains, err := synth.Synthesize(backend)
		return chains, err
	}

	// Sanity: with proxy-defaults=http, the REAL chain for x resolves http and
	// compiles cleanly (y is http via the fallback) — i.e. /v1/discovery-chain
	// would return http. This is the precondition that distinguishes the
	// regression from a genuine protocol mismatch.
	t.Run("real backend chain resolves http via proxy-defaults", func(t *testing.T) {
		chain, err := compileBackendChain(t, proxyDefaultsHTTP, xRouter)
		require.NoError(t, err)
		require.Equal(t, "http", chain.Protocol,
			"real compiler applies the proxy-defaults fallback for entry-less y")
	})

	// The regression itself: real chain is http, but gateway synthesis must not
	// fail with "inconsistent protocols". With the fix in place this passes;
	// before the fix this returned that error (the composed destination y had no
	// synthetic http default). proxy-defaults alone does NOT prevent it — it is
	// never part of the synthetic entry set — which is why the field team saw
	// "proxy-defaults was set all along and it still failed".
	t.Run("gateway synthesis succeeds (regression fixed)", func(t *testing.T) {
		chain, err := compileBackendChain(t, proxyDefaultsHTTP, xRouter)
		require.NoError(t, err)

		chains, err := synthesize(t, chain)
		require.NoError(t, err,
			"gateway synthesis must not spuriously resolve y as tcp when the real chain is http")
		require.NotEmpty(t, chains)
	})

	// Field-team mitigation check: adding service-defaults=http for y on the
	// real/backend side. It keeps x's real chain http (already was) but note it
	// does NOT feed the synthesizer's synthetic entry set — the synthesizer only
	// receives x's compiled chain, not y's config. The fix (not the mitigation)
	// is what makes synthesis pass here.
	t.Run("service-defaults on y keeps real chain http", func(t *testing.T) {
		chain, err := compileBackendChain(t, proxyDefaultsHTTP, xRouter, yServiceDefaultsHTTP)
		require.NoError(t, err)
		require.Equal(t, "http", chain.Protocol)

		chains, err := synthesize(t, chain)
		require.NoError(t, err)
		require.NotEmpty(t, chains)
	})
}
