// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
					Kind: structs.ProxyConfigGlobal,
					Name: "global",
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
