package discoverychain

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestAddTCPRoute(t *testing.T) {
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

func TestAddHTTPRoute(t *testing.T) {
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
