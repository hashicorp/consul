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
		matchesByHostname: map[string][]hostnameMatch{},
		tcpRoutes: []structs.TCPRouteConfigEntry{
			route,
		},
	}

	gatewayChainSynthesizer := NewGatewayChainSynthesizer(datacenter, gateway)

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
			expectedMatchesByHostname: map[string][]hostnameMatch{},
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

			gatewayChainSynthesizer := NewGatewayChainSynthesizer(datacenter, gateway)

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
		expectedDiscoveryChain  *structs.CompiledDiscoveryChain
	}{
		// TODO Add tests for other synthesizer types.
		"TCPRoute-based listener": {
			synthesizer: NewGatewayChainSynthesizer("dc1", &structs.APIGatewayConfigEntry{
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
			expectedDiscoveryChain: &structs.CompiledDiscoveryChain{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			},
		},
		"HTTPRoute-based listener": {
			synthesizer: NewGatewayChainSynthesizer("dc1", &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			}),
			httpRoutes: []*structs.HTTPRouteConfigEntry{
				{
					Kind: structs.HTTPRoute,
					Name: "http-route",
				},
			},
			chain: &structs.CompiledDiscoveryChain{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			},
			extra:                   []*structs.CompiledDiscoveryChain{},
			expectedIngressServices: []structs.IngressService{},
			expectedDiscoveryChain: &structs.CompiledDiscoveryChain{
				ServiceName: "foo",
				Namespace:   "default",
				Datacenter:  "dc1",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, tcpRoute := range tc.tcpRoutes {
				tc.synthesizer.AddTCPRoute(*tcpRoute)
			}
			for _, httpRoute := range tc.httpRoutes {
				tc.synthesizer.AddHTTPRoute(*httpRoute)
			}

			chains := append([]*structs.CompiledDiscoveryChain{tc.chain}, tc.extra...)
			ingressServices, discoveryChain, err := tc.synthesizer.Synthesize(chains...)

			require.NoError(t, err)
			require.Equal(t, tc.expectedIngressServices, ingressServices)
			require.Equal(t, tc.expectedDiscoveryChain, discoveryChain)
		})
	}
}
