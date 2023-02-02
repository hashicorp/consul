package discoverychain

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
)

func hostsKey(hosts ...string) string {
	sort.Strings(hosts)
	hostsHash := crc32.NewIEEE()
	for _, h := range hosts {
		if _, err := hostsHash.Write([]byte(h)); err != nil {
			continue
		}
	}
	return strconv.FormatUint(uint64(hostsHash.Sum32()), 16)
}

type hostnameMatch struct {
	match    structs.HTTPMatch
	filters  structs.HTTPFilters
	services []structs.HTTPService
}

type GatewayChainSynthesizer struct {
	datacenter        string
	gateway           *structs.APIGatewayConfigEntry
	matchesByHostname map[string][]hostnameMatch
	tcpRoutes         []structs.TCPRouteConfigEntry
}

func NewGatewayChainSynthesizer(datacenter string, gateway *structs.APIGatewayConfigEntry) *GatewayChainSynthesizer {
	return &GatewayChainSynthesizer{
		datacenter:        datacenter,
		gateway:           gateway,
		matchesByHostname: map[string][]hostnameMatch{},
	}
}

// AddTCPRoute adds a TCPRoute to use in synthesizing a discovery chain
func (l *GatewayChainSynthesizer) AddTCPRoute(route structs.TCPRouteConfigEntry) {
	l.tcpRoutes = append(l.tcpRoutes, route)
}

// AddHTTPRoute takes a new route and flattens its rule matches out per hostname.
// This is required since a single route can specify multiple hostnames, and a
// single hostname can be specified in multiple routes. Routing for a given
// hostname must behave based on the aggregate of all rules that apply to it.
func (l *GatewayChainSynthesizer) AddHTTPRoute(route structs.HTTPRouteConfigEntry) {
	for _, host := range route.Hostnames {
		matches, ok := l.matchesByHostname[host]
		if !ok {
			matches = []hostnameMatch{}
		}

		for _, rule := range route.Rules {
			// If a rule has no matches defined, add default match
			if len(rule.Matches) == 0 {
				rule.Matches = []structs.HTTPMatch{{
					Path: structs.HTTPPathMatch{
						Match: structs.HTTPPathMatchPrefix,
						Value: "/",
					},
				}}
			}

			// Add all matches for this rule to the list for this hostname
			for _, match := range rule.Matches {
				matches = append(matches, hostnameMatch{
					match:    match,
					filters:  rule.Filters,
					services: rule.Services,
				})
			}
		}

		l.matchesByHostname[host] = matches
	}
}

// consolidateHTTPRoutes combines all rules into the shortest possible list of routes
// with one route per hostname containing all rules for that hostname.
func (l *GatewayChainSynthesizer) consolidateHTTPRoutes() []structs.HTTPRouteConfigEntry {
	var routes []structs.HTTPRouteConfigEntry

	for hostname, rules := range l.matchesByHostname {
		// Create route for this hostname
		route := structs.HTTPRouteConfigEntry{
			Kind:           structs.HTTPRoute,
			Name:           fmt.Sprintf("%s-%s", l.gateway.Name, hostsKey(hostname)),
			Hostnames:      []string{hostname},
			Rules:          make([]structs.HTTPRouteRule, 0, len(rules)),
			Meta:           l.gateway.Meta,
			EnterpriseMeta: l.gateway.EnterpriseMeta,
		}

		// Sort rules for this hostname in order of precedence
		sort.SliceStable(rules, func(i, j int) bool {
			return compareHTTPRules(rules[i].match, rules[j].match)
		})

		// Add all rules for this hostname
		for _, rule := range rules {
			route.Rules = append(route.Rules, structs.HTTPRouteRule{
				Matches:  []structs.HTTPMatch{rule.match},
				Filters:  rule.filters,
				Services: rule.services,
			})
		}

		routes = append(routes, route)
	}

	return routes
}

func (l *GatewayChainSynthesizer) synthesizeEntries() ([]structs.IngressService, *configentry.DiscoveryChainSet) {
	services := []structs.IngressService{}
	entries := configentry.NewDiscoveryChainSet()

	for _, route := range l.consolidateHTTPRoutes() {
		ingress, router, splitters, defaults := synthesizeHTTPRouteDiscoveryChain(route)
		entries.AddRouters(router)
		entries.AddSplitters(splitters...)
		entries.AddServices(defaults...)
		services = append(services, ingress)
	}

	for _, route := range l.tcpRoutes {
		services = append(services, synthesizeTCPRouteDiscoveryChain(route)...)
	}

	return services, entries
}

// Synthesize assembles a synthetic discovery chain from multiple other discovery chains
// that have StartNodes that are referenced by routers or splitters in the entries for the
// given CompileRequest.
//
// This is currently used to help API gateways masquarade as ingress gateways
// by providing a set of virtual config entries that change the routing behavior
// to upstreams referenced in the given HTTPRoutes or TCPRoutes.
func (l *GatewayChainSynthesizer) Synthesize(chain *structs.CompiledDiscoveryChain, extra ...*structs.CompiledDiscoveryChain) ([]structs.IngressService, *structs.CompiledDiscoveryChain, error) {
	extra = append([]*structs.CompiledDiscoveryChain{chain}, extra...)

	services, entries := l.synthesizeEntries()

	if entries.IsEmpty() {
		// we can't actually compile a discovery chain, i.e. we're using a TCPRoute-based listener, instead, just return the ingresses
		// and the first pre-compiled discovery chain
		return services, chain, nil
	}

	compiled, err := Compile(CompileRequest{
		ServiceName:          l.gateway.Name,
		EvaluateInNamespace:  l.gateway.NamespaceOrDefault(),
		EvaluateInPartition:  l.gateway.PartitionOrDefault(),
		EvaluateInDatacenter: l.datacenter,
		Entries:              entries,
	})
	if err != nil {
		return nil, nil, err
	}

	for _, c := range extra {
		for id, target := range c.Targets {
			compiled.Targets[id] = target
		}
		for id, node := range c.Nodes {
			compiled.Nodes[id] = node
		}
		compiled.EnvoyExtensions = append(compiled.EnvoyExtensions, c.EnvoyExtensions...)
	}

	return services, compiled, nil
}
