// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package discoverychain

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
)

// GatewayChainSynthesizer is used to synthesize a discovery chain for a
// gateway from its configuration and multiple other discovery chains.
type GatewayChainSynthesizer struct {
	datacenter        string
	trustDomain       string
	suffix            string
	gateway           *structs.APIGatewayConfigEntry
	hostname          string
	matchesByHostname map[string][]hostnameMatch
	tcpRoutes         []structs.TCPRouteConfigEntry
}

type hostnameMatch struct {
	match    structs.HTTPMatch
	filters  structs.HTTPFilters
	services []structs.HTTPService
}

// NewGatewayChainSynthesizer creates a new GatewayChainSynthesizer for the
// given gateway and datacenter.
func NewGatewayChainSynthesizer(datacenter, trustDomain, suffix string, gateway *structs.APIGatewayConfigEntry) *GatewayChainSynthesizer {
	return &GatewayChainSynthesizer{
		datacenter:        datacenter,
		trustDomain:       trustDomain,
		suffix:            suffix,
		gateway:           gateway,
		matchesByHostname: map[string][]hostnameMatch{},
	}
}

// AddTCPRoute adds a TCPRoute to use in synthesizing a discovery chain
func (l *GatewayChainSynthesizer) AddTCPRoute(route structs.TCPRouteConfigEntry) {
	l.tcpRoutes = append(l.tcpRoutes, route)
}

// SetHostname sets the base hostname for a listener that this is being synthesized for
func (l *GatewayChainSynthesizer) SetHostname(hostname string) {
	l.hostname = hostname
}

// AddHTTPRoute takes a new route and flattens its rule matches out per hostname.
// This is required since a single route can specify multiple hostnames, and a
// single hostname can be specified in multiple routes. Routing for a given
// hostname must behave based on the aggregate of all rules that apply to it.
func (l *GatewayChainSynthesizer) AddHTTPRoute(route structs.HTTPRouteConfigEntry) {
	l.matchesByHostname = getHostMatches(l.hostname, &route, l.matchesByHostname)
}

func getHostMatches(hostname string, route *structs.HTTPRouteConfigEntry, currentMatches map[string][]hostnameMatch) map[string][]hostnameMatch {
	hostnames := route.FilteredHostnames(hostname)
	for _, host := range hostnames {
		matches, ok := currentMatches[host]
		if !ok {
			matches = []hostnameMatch{}
		}

		for _, rule := range route.Rules {
			// If a rule has no matches defined, add default match
			if rule.Matches == nil {
				rule.Matches = []structs.HTTPMatch{}
			}
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

		currentMatches[host] = matches
	}
	return currentMatches
}

// Synthesize assembles a synthetic discovery chain from multiple other discovery chains
// that have StartNodes that are referenced by routers or splitters in the entries for the
// given CompileRequest.
//
// This is currently used to help API gateways masquarade as ingress gateways
// by providing a set of virtual config entries that change the routing behavior
// to upstreams referenced in the given HTTPRoutes or TCPRoutes.
func (l *GatewayChainSynthesizer) Synthesize(chains ...*structs.CompiledDiscoveryChain) ([]structs.IngressService, []*structs.CompiledDiscoveryChain, error) {
	if len(chains) == 0 {
		return nil, nil, fmt.Errorf("must provide at least one compiled discovery chain")
	}

	services, set := l.synthesizeEntries()

	if len(set) == 0 {
		// we can't actually compile a discovery chain, i.e. we're using a TCPRoute-based listener, instead, just return the ingresses
		// and the pre-compiled discovery chains
		return services, chains, nil
	}

	compiledChains := make([]*structs.CompiledDiscoveryChain, 0, len(set))
	for i, service := range services {

		entries := set[i]

		compiled, err := Compile(CompileRequest{
			ServiceName:           service.Name,
			EvaluateInNamespace:   service.NamespaceOrDefault(),
			EvaluateInPartition:   service.PartitionOrDefault(),
			EvaluateInDatacenter:  l.datacenter,
			EvaluateInTrustDomain: l.trustDomain,
			Entries:               entries,
		})
		if err != nil {
			return nil, nil, err
		}

		node := compiled.Nodes[compiled.StartNode]
		if node.IsRouter() {
			resolverPrefix := structs.DiscoveryGraphNodeTypeResolver + ":" + node.Name

			// clean out the clusters that will get added for the router
			for name := range compiled.Nodes {
				if strings.HasPrefix(name, resolverPrefix) {
					delete(compiled.Nodes, name)
				}
			}

			// clean out the route rules that'll get added for the router
			filtered := []*structs.DiscoveryRoute{}
			for _, route := range node.Routes {
				if strings.HasPrefix(route.NextNode, resolverPrefix) {
					continue
				}
				filtered = append(filtered, route)
			}
			node.Routes = filtered
		}
		compiled.Nodes[compiled.StartNode] = node

		// fix up the nodes for the terminal targets to either be a splitter or resolver if there is no splitter present
		for name, node := range compiled.Nodes {
			switch node.Type {
			// we should only have these two types
			case structs.DiscoveryGraphNodeTypeRouter:
				for i, route := range node.Routes {
					node.Routes[i].NextNode = targetForResolverNode(route.NextNode, chains)
				}
			case structs.DiscoveryGraphNodeTypeSplitter:
				for i, split := range node.Splits {
					node.Splits[i].NextNode = targetForResolverNode(split.NextNode, chains)
				}
			}
			compiled.Nodes[name] = node
		}

		for _, c := range chains {
			for id, target := range c.Targets {
				compiled.Targets[id] = target
			}
			for id, node := range c.Nodes {
				compiled.Nodes[id] = node
			}
			compiled.EnvoyExtensions = append(compiled.EnvoyExtensions, c.EnvoyExtensions...)
		}
		compiledChains = append(compiledChains, compiled)
	}

	return services, compiledChains, nil
}

// consolidateHTTPRoutes combines all rules into the shortest possible list of routes
// with one route per hostname containing all rules for that hostname.
func (l *GatewayChainSynthesizer) consolidateHTTPRoutes() []structs.HTTPRouteConfigEntry {
	return consolidateHTTPRoutes(l.matchesByHostname, l.suffix, l.gateway)
}

// consolidateHTTPRoutes combines all rules into the shortest possible list of routes
// with one route per hostname containing all rules for that hostname.
func consolidateHTTPRoutes(matchesByHostname map[string][]hostnameMatch, suffix string, gateway *structs.APIGatewayConfigEntry) []structs.HTTPRouteConfigEntry {
	var routes []structs.HTTPRouteConfigEntry

	for hostname, rules := range matchesByHostname {
		// Create route for this hostname
		route := structs.HTTPRouteConfigEntry{
			Kind:           structs.HTTPRoute,
			Name:           fmt.Sprintf("%s-%s-%s", gateway.Name, suffix, hostsKey(hostname)),
			Hostnames:      []string{hostname},
			Rules:          make([]structs.HTTPRouteRule, 0, len(rules)),
			Meta:           gateway.Meta,
			EnterpriseMeta: gateway.EnterpriseMeta,
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

func targetForResolverNode(nodeName string, chains []*structs.CompiledDiscoveryChain) string {
	resolverPrefix := structs.DiscoveryGraphNodeTypeResolver + ":"
	splitterPrefix := structs.DiscoveryGraphNodeTypeSplitter + ":"

	if !strings.HasPrefix(nodeName, resolverPrefix) {
		return nodeName
	}

	splitterName := splitterPrefix + strings.TrimPrefix(nodeName, resolverPrefix)

	for _, c := range chains {
		for name, node := range c.Nodes {
			if node.IsSplitter() && strings.HasPrefix(splitterName, name) {
				return name
			}
		}
	}

	return nodeName
}

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

func (l *GatewayChainSynthesizer) synthesizeEntries() ([]structs.IngressService, []*configentry.DiscoveryChainSet) {
	services := []structs.IngressService{}
	entries := []*configentry.DiscoveryChainSet{}

	for _, route := range l.consolidateHTTPRoutes() {
		ingress, router, splitters, defaults := synthesizeHTTPRouteDiscoveryChain(route)

		services = append(services, ingress)

		entrySet := configentry.NewDiscoveryChainSet()
		entrySet.AddRouters(router)
		entrySet.AddSplitters(splitters...)
		entrySet.AddServices(defaults...)
		entries = append(entries, entrySet)
	}

	for _, route := range l.tcpRoutes {
		services = append(services, synthesizeTCPRouteDiscoveryChain(route)...)
	}

	return services, entries
}
