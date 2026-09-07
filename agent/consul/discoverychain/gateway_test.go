// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

import (
	"fmt"
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
			ingressServices, discoveryChains, skipped, err := tc.synthesizer.Synthesize(chains...)

			require.NoError(t, err)
			require.Empty(t, skipped)
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
			_, discoveryChains, skipped, err := tc.synthesizer.Synthesize(chains...)

			require.NoError(t, err)
			require.Empty(t, skipped)
			require.Len(t, discoveryChains, 1)
			require.Equal(t, tc.expectedDiscoveryChain, discoveryChains[0])
		})
	}
}

// TestSynthesizeHTTPRouteDiscoveryChain_ComposedDestinationProtocol is a
// regression test for the protocol-resolution defect where a backend
// service-router's redirect target (e.g. "service-b") has no service-defaults
// entry and would normally be resolved as tcp in the synthetic gateway chain.
//
// The synthetic entry set intentionally omits proxy-defaults, so an entry-less
// destination falls through to the tcp default. The fix injects a synthetic
// http service-defaults for each composed destination so the gateway chain
// compiles cleanly, matching the treatment of directly-referenced destinations.
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
		"composed sub-destination must receive a synthetic http default so the "+
			"synthesized chain does not resolve it as tcp (noisy-neighbour regression)")
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
// It verifies two things:
//
//  1. The original regression is fixed: synthesis must NOT fail with
//     "inconsistent protocols" for a correctly-configured route, even when the
//     backend chain was compiled via proxy-defaults (not explicit service-defaults).
//
//  2. Noisy-neighbour isolation: a route that generates a compile error during
//     synthesis is skipped and its error returned; it does NOT abort the listener
//     or prevent other correctly-configured routes from being served.
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

	synthesize := func(t *testing.T, backend *structs.CompiledDiscoveryChain) ([]*structs.CompiledDiscoveryChain, []error, error) {
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
		_, chains, skipped, err := synth.Synthesize(backend)
		return chains, skipped, err
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

	// The original regression: synthesis must succeed even though y has no
	// service-defaults and the synthetic entry set carries no proxy-defaults.
	// appendComposedHTTPDefault injects a synthetic http default for y so the
	// chain compiles cleanly without needing explicit service-defaults for y.
	t.Run("gateway synthesis succeeds without explicit service-defaults for composed destination", func(t *testing.T) {
		chain, err := compileBackendChain(t, proxyDefaultsHTTP, xRouter)
		require.NoError(t, err)

		chains, skipped, fatalErr := synthesize(t, chain)
		require.NoError(t, fatalErr,
			"synthesis must not fail: the composed destination y must get a synthetic http default")
		require.Empty(t, skipped,
			"no routes should be skipped: the route is correctly configured (y is an http service via proxy-defaults)")
		require.NotEmpty(t, chains)
	})

	// With service-defaults=http for y explicitly set, the synthetic chain
	// resolves y as http and the route compiles cleanly — same outcome as above
	// but via an explicit config rather than the synthetic default injection.
	t.Run("route succeeds when composed destination has explicit service-defaults=http", func(t *testing.T) {
		chain, err := compileBackendChain(t, proxyDefaultsHTTP, xRouter, yServiceDefaultsHTTP)
		require.NoError(t, err)
		require.Equal(t, "http", chain.Protocol)

		chains, skipped, fatalErr := synthesize(t, chain)
		require.NoError(t, fatalErr)
		require.Empty(t, skipped, "no routes should be skipped when config is correct")
		require.NotEmpty(t, chains)
	})
}

// --- shared helpers for the noisy-neighbour partial-failure tests below ---
//
// These build the two recurring fixtures every test in this section needs:
// a "good" HTTPRoute/chain pair that always compiles cleanly, and a "bad"
// HTTPRoute/chain pair that always fails to compile for a real reason (not
// the protocol-fallback trigger appendComposedHTTPDefault already handles
// elsewhere in this file).

// newHTTPRouteToService builds a single-service, path-prefix HTTPRoute - the
// minimal shape needed to exercise Synthesize's per-route compile loop.
func newHTTPRouteToService(name, hostname, serviceName string) structs.HTTPRouteConfigEntry {
	return structs.HTTPRouteConfigEntry{
		Kind:      structs.HTTPRoute,
		Name:      name,
		Hostnames: []string{hostname},
		Rules: []structs.HTTPRouteRule{{
			Matches: []structs.HTTPMatch{{
				Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/"},
			}},
			Services: []structs.HTTPService{{Name: serviceName}},
		}},
	}
}

// mustCompileHTTPChain compiles a real, always-succeeding discovery chain for
// a plain http service with no router - the "good" half of these tests.
func mustCompileHTTPChain(t *testing.T, serviceName string) *structs.CompiledDiscoveryChain {
	t.Helper()

	set := configentry.NewDiscoveryChainSet()
	set.AddServices(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     serviceName,
		Protocol: "http",
	})
	chain, err := Compile(CompileRequest{
		ServiceName:           serviceName,
		EvaluateInNamespace:   "default",
		EvaluateInPartition:   "default",
		EvaluateInDatacenter:  "dc1",
		EvaluateInTrustDomain: "domain",
		Entries:               set,
	})
	require.NoError(t, err)
	return chain
}

// newBrokenSubsetChain hand-builds a chain rather than running it through
// Compile(): it only needs to give serviceRouterRulesFromChains a router
// whose route composes a destination pinned to a subset that is never
// defined. mergeServiceRouteDestination preserves that subset as-is when
// composing it into the route's synthetic router, and resolverEntriesFromChains
// has no matching subset in this chain's (empty) Targets to inject a resolver
// for it, so Compile() falls back to the default (subset-less) resolver and
// fails with "does not have a subset named" - a genuine failure
// appendComposedHTTPDefault has no way to paper over, since it's unrelated to
// protocol resolution. This mirrors a real scenario where a backend router's
// target subset was removed or never matched by the time synthesis runs.
func newBrokenSubsetChain(serviceName, downstreamServiceName, missingSubset string) *structs.CompiledDiscoveryChain {
	routerNode := "router:" + serviceName
	return &structs.CompiledDiscoveryChain{
		ServiceName: serviceName,
		Namespace:   "default",
		Partition:   "default",
		Datacenter:  "dc1",
		StartNode:   routerNode,
		Nodes: map[string]*structs.DiscoveryGraphNode{
			routerNode: {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: serviceName + "-router",
				Routes: []*structs.DiscoveryRoute{{
					Definition: &structs.ServiceRoute{
						Destination: &structs.ServiceRouteDestination{
							Service:       downstreamServiceName,
							Namespace:     "default",
							Partition:     "default",
							ServiceSubset: missingSubset,
						},
					},
				}},
			},
		},
	}
}

// TestGatewaySynthesis_NoisyNeighbourIsolation verifies that a route that
// produces a compile error during synthesis is skipped and its error returned
// without aborting the rest of the listener.
//
// This verifies, with a real skip in hand:
//
//  1. Synthesize does not return a fatal error.
//  2. The failing (bad-svc) route appears in the returned skipped slice, with
//     the real underlying compile error preserved.
//  3. The other (good-svc) route on the same listener is still compiled and
//     returned — not the bad one.
func TestGatewaySynthesis_NoisyNeighbourIsolation(t *testing.T) {
	t.Parallel()

	gateway := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "gateway",
	}

	goodRoute := newHTTPRouteToService("good-route", "good.example.com", "good-svc")
	goodChain := mustCompileHTTPChain(t, "good-svc")

	badRoute := newHTTPRouteToService("bad-route", "bad.example.com", "bad-svc")
	badChain := newBrokenSubsetChain("bad-svc", "downstream-tcp-svc", "ghost-subset")

	synth := NewGatewayChainSynthesizer("dc1", "domain", "listener", gateway)
	synth.SetHostname("*")
	synth.AddHTTPRoute(goodRoute)
	synth.AddHTTPRoute(badRoute)

	services, compiledChains, skipped, fatalErr := synth.Synthesize(goodChain, badChain)
	require.NoError(t, fatalErr, "a compile error for one route must never abort the entire listener")

	require.Len(t, skipped, 1, "exactly the bad-svc route should be skipped")
	require.ErrorContains(t, skipped[0], "does not have a subset named",
		"the skipped error should surface the real compile failure for downstream-tcp-svc")

	require.Len(t, services, 1, "only the good-svc ingress service should survive")
	require.Equal(t, []string{"good.example.com"}, services[0].Hosts,
		"the surviving service must be the good-svc route, not bad-svc")
	require.Len(t, compiledChains, 1, "only the good-svc compiled chain should survive")
}

// TestGatewaySynthesis_AllRoutesFail verifies the boundary the "one bad route
// among several" cases don't exercise: what happens when every route on a
// listener fails to compile. Synthesize must still not return a fatal error,
// and must return empty (not nil-panicking, not partially-populated) service
// and chain lists, with one skipped entry per broken route.
func TestGatewaySynthesis_AllRoutesFail(t *testing.T) {
	t.Parallel()

	gateway := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "gateway",
	}

	cases := map[string]struct {
		routeCount int
	}{
		"single broken route":    {routeCount: 1},
		"multiple broken routes": {routeCount: 3},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synth := NewGatewayChainSynthesizer("dc1", "domain", "listener", gateway)
			synth.SetHostname("*")

			chains := make([]*structs.CompiledDiscoveryChain, 0, tc.routeCount)
			for i := 0; i < tc.routeCount; i++ {
				svcName := fmt.Sprintf("bad-svc-%d", i)
				synth.AddHTTPRoute(newHTTPRouteToService(
					fmt.Sprintf("bad-route-%d", i),
					fmt.Sprintf("bad-%d.example.com", i),
					svcName,
				))
				chains = append(chains, newBrokenSubsetChain(svcName, "downstream-tcp-svc", "ghost-subset"))
			}

			services, compiledChains, skipped, fatalErr := synth.Synthesize(chains...)
			require.NoError(t, fatalErr, "even a listener with zero working routes must not fail synthesis fatally")
			require.Empty(t, services, "no ingress services should survive when every route is broken")
			require.Empty(t, compiledChains, "no compiled chains should survive when every route is broken")
			require.Len(t, skipped, tc.routeCount, "every broken route should be recorded in skipped")
		})
	}
}

// TestGatewaySynthesis_PartialFailure_PreservesAlignment verifies the
// invariant recompileDiscoveryChains depends on (agent/proxycfg/api_gateway.go,
// the "compiled[i].ServiceName != service.Name" check): the returned services
// and compiledChains slices must stay pairwise aligned even when failures are
// interleaved with successes, not just when a single failure sits at the end.
func TestGatewaySynthesis_PartialFailure_PreservesAlignment(t *testing.T) {
	t.Parallel()

	gateway := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "gateway",
	}

	synth := NewGatewayChainSynthesizer("dc1", "domain", "listener", gateway)
	synth.SetHostname("*")

	// good1, bad1, good2, bad2, good3 - failures interleaved on both sides of
	// surviving routes, not just trailing them.
	order := []struct {
		hostname string
		service  string
		good     bool
	}{
		{"good1.example.com", "good-svc-1", true},
		{"bad1.example.com", "bad-svc-1", false},
		{"good2.example.com", "good-svc-2", true},
		{"bad2.example.com", "bad-svc-2", false},
		{"good3.example.com", "good-svc-3", true},
	}

	chains := make([]*structs.CompiledDiscoveryChain, 0, len(order))
	for i, route := range order {
		synth.AddHTTPRoute(newHTTPRouteToService(fmt.Sprintf("route-%d", i), route.hostname, route.service))
		if route.good {
			chains = append(chains, mustCompileHTTPChain(t, route.service))
		} else {
			chains = append(chains, newBrokenSubsetChain(route.service, "downstream-tcp-svc", "ghost-subset"))
		}
	}

	services, compiledChains, skipped, fatalErr := synth.Synthesize(chains...)
	require.NoError(t, fatalErr, "interleaved failures must never abort the entire listener")
	require.Len(t, skipped, 2, "exactly the two bad routes should be skipped")

	require.Len(t, services, 3, "exactly the three good routes should survive")
	require.Len(t, compiledChains, 3, "compiledChains must stay the same length as services")

	// consolidateHTTPRoutes iterates a map keyed by hostname, so the relative
	// order of survivors is not guaranteed - assert the set, not the order.
	gotHosts := make([]string, len(services))
	for i, svc := range services {
		gotHosts[i] = svc.Hosts[0]
		// This is the actual invariant recompileDiscoveryChains depends on
		// (agent/proxycfg/api_gateway.go's "compiled[i].ServiceName !=
		// service.Name" check): whatever order the survivors end up in,
		// services[i] and compiledChains[i] must describe the same route.
		require.Equal(t, svc.Name, compiledChains[i].ServiceName,
			"services[%d] and compiledChains[%d] must describe the same synthesized route", i, i)
	}
	require.ElementsMatch(t, []string{"good1.example.com", "good2.example.com", "good3.example.com"}, gotHosts,
		"surviving services must be exactly the three good routes")
}
