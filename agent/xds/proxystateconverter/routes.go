// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxystateconverter

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"

	"github.com/hashicorp/consul/agent/xds/response"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func (s *Converter) routesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) error {
	if cfgSnap == nil {
		return errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.routesForConnectProxy(cfgSnap)
	// TODO(proxystate): Ingress Gateways will be added in the future.
	//case structs.ServiceKindIngressGateway:
	//	return s.routesForIngressGateway(cfgSnap)
	// TODO(proxystate): API Gateways will be added in the future.
	//case structs.ServiceKindAPIGateway:
	//	return s.routesForAPIGateway(cfgSnap)
	// TODO(proxystate): Terminating Gateways will be added in the future.
	//case structs.ServiceKindTerminatingGateway:
	//	return s.routesForTerminatingGateway(cfgSnap)
	// TODO(proxystate): Mesh Gateways will be added in the future.
	//case structs.ServiceKindMeshGateway:
	//	return s.routesForMeshGateway(cfgSnap)
	default:
		return fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func (s *Converter) routesForConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) error {
	for uid, chain := range cfgSnap.ConnectProxy.DiscoveryChain {
		if chain.Default {
			continue
		}

		// route already exists, don't clobber it.
		if _, ok := s.proxyState.Routes[uid.EnvoyID()]; ok {
			continue
		}

		virtualHost, err := s.makeUpstreamHostForDiscoveryChain(cfgSnap, uid, chain, []string{"*"}, false)
		if err != nil {
			return err
		}
		if virtualHost == nil {
			continue
		}

		route := &pbproxystate.Route{
			VirtualHosts: []*pbproxystate.VirtualHost{virtualHost},
		}
		s.proxyState.Routes[uid.EnvoyID()] = route
	}
	addressesMap := make(map[string]map[string]string)
	err := cfgSnap.ConnectProxy.DestinationsUpstream.ForEachKeyE(func(uid proxycfg.UpstreamID) error {
		svcConfig, ok := cfgSnap.ConnectProxy.DestinationsUpstream.Get(uid)
		if !ok || svcConfig == nil {
			return nil
		}
		if !structs.IsProtocolHTTPLike(svcConfig.Protocol) {
			// Routes can only be defined for HTTP services
			return nil
		}

		for _, address := range svcConfig.Destination.Addresses {

			routeName := clusterNameForDestination(cfgSnap, "~http", fmt.Sprintf("%d", svcConfig.Destination.Port), svcConfig.NamespaceOrDefault(), svcConfig.PartitionOrDefault())
			if _, ok := addressesMap[routeName]; !ok {
				addressesMap[routeName] = make(map[string]string)
			}
			// cluster name is unique per address/port so we should not be doing any override here
			clusterName := clusterNameForDestination(cfgSnap, svcConfig.Name, address, svcConfig.NamespaceOrDefault(), svcConfig.PartitionOrDefault())
			addressesMap[routeName][clusterName] = address
		}
		return nil
	})

	if err != nil {
		return err
	}

	for routeName, clusters := range addressesMap {
		route, err := s.makeRouteForAddresses(clusters)
		if err != nil {
			return err
		}
		if route != nil {
			s.proxyState.Routes[routeName] = route
		}
	}

	// TODO(rb): make sure we don't generate an empty result
	return nil
}

func (s *Converter) makeRouteForAddresses(addresses map[string]string) (*pbproxystate.Route, error) {
	route, err := makeAddressesRoute(addresses)
	if err != nil {
		s.Logger.Error("failed to make route", "cluster", "error", err)
		return nil, err
	}

	return route, nil
}

// TODO(proxystate): Terminating Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func routesForTerminatingGateway

// TODO(proxystate): Mesh Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func routesForMeshGateway

func makeAddressesRoute(addresses map[string]string) (*pbproxystate.Route, error) {
	route := &pbproxystate.Route{}
	for clusterName, address := range addresses {
		destination := makeRouteDestinationFromName(clusterName)
		virtualHost := &pbproxystate.VirtualHost{
			Name:    clusterName,
			Domains: []string{address},
			RouteRules: []*pbproxystate.RouteRule{
				{
					Match:       makeDefaultRouteMatch(),
					Destination: destination,
				},
			},
		}
		route.VirtualHosts = append(route.VirtualHosts, virtualHost)
	}

	// sort virtual hosts to have a stable order
	sort.SliceStable(route.VirtualHosts, func(i, j int) bool {
		return route.VirtualHosts[i].Name > route.VirtualHosts[j].Name
	})
	return route, nil
}

// makeRouteDestinationFromName (fka makeRouteActionFromName)
func makeRouteDestinationFromName(clusterName string) *pbproxystate.RouteDestination {
	return &pbproxystate.RouteDestination{
		Destination: &pbproxystate.RouteDestination_Cluster{
			Cluster: &pbproxystate.DestinationCluster{
				Name: clusterName,
			},
		},
	}
}

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func routesForIngressGateway

// TODO(proxystate): API Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func routesForAPIGateway
// func buildHTTPRouteUpstream

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func findIngressServiceMatchingUpstream

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func generateUpstreamIngressDomains

// TODO(proxystate): Ingress Gateways will be added in the future.
// Functions to add from agent/xds/clusters.go:
// func generateUpstreamAPIsDomains

func (s *Converter) makeUpstreamHostForDiscoveryChain(
	cfgSnap *proxycfg.ConfigSnapshot,
	uid proxycfg.UpstreamID,
	chain *structs.CompiledDiscoveryChain,
	serviceDomains []string,
	forMeshGateway bool,
) (*pbproxystate.VirtualHost, error) {
	var routeRules []*pbproxystate.RouteRule

	startNode := chain.Nodes[chain.StartNode]
	if startNode == nil {
		return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
	}

	upstreamsSnapshot, err := cfgSnap.ToConfigSnapshotUpstreams()
	if err != nil && !forMeshGateway {
		return nil, err
	}

	switch startNode.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		routeRules = make([]*pbproxystate.RouteRule, 0, len(startNode.Routes))

		for _, discoveryRoute := range startNode.Routes {
			routeMatch := makeRouteMatchForDiscoveryRoute(discoveryRoute)

			var (
				routeDestination *pbproxystate.RouteDestination
				err              error
			)

			nextNode := chain.Nodes[discoveryRoute.NextNode]

			var lb *structs.LoadBalancer
			if nextNode.LoadBalancer != nil {
				lb = nextNode.LoadBalancer
			}

			switch nextNode.Type {
			case structs.DiscoveryGraphNodeTypeSplitter:
				routeDestination, err = s.makeRouteDestinationForSplitter(upstreamsSnapshot, nextNode.Splits, chain, forMeshGateway)
				if err != nil {
					return nil, err
				}

			case structs.DiscoveryGraphNodeTypeResolver:
				rd, ok := s.makeRouteDestinationForChainCluster(upstreamsSnapshot, nextNode.Resolver.Target, chain, forMeshGateway)
				if !ok {
					continue
				}
				routeDestination = rd

			default:
				return nil, fmt.Errorf("unexpected graph node after route %q", nextNode.Type)
			}

			routeDestination.DestinationConfiguration = &pbproxystate.DestinationConfiguration{}
			if err := injectLBToDestinationConfiguration(lb, routeDestination.DestinationConfiguration); err != nil {
				return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
			}

			// TODO(rb): Better help handle the envoy case where you need (prefix=/foo/,rewrite=/) and (exact=/foo,rewrite=/) to do a full rewrite

			destination := discoveryRoute.Definition.Destination

			routeRule := &pbproxystate.RouteRule{}

			if destination != nil {
				routeDestinationConfiguration := routeDestination.DestinationConfiguration
				if destination.PrefixRewrite != "" {
					routeDestinationConfiguration.PrefixRewrite = destination.PrefixRewrite
				}

				if destination.RequestTimeout != 0 || destination.IdleTimeout != 0 {
					routeDestinationConfiguration.TimeoutConfig = &pbproxystate.TimeoutConfig{}
				}
				if destination.RequestTimeout > 0 {
					routeDestinationConfiguration.TimeoutConfig.Timeout = durationpb.New(destination.RequestTimeout)
				}
				// Disable the timeout if user specifies negative value. Setting 0 disables the timeout in Envoy.
				if destination.RequestTimeout < 0 {
					routeDestinationConfiguration.TimeoutConfig.Timeout = durationpb.New(0 * time.Second)
				}

				if destination.IdleTimeout > 0 {
					routeDestinationConfiguration.TimeoutConfig.IdleTimeout = durationpb.New(destination.IdleTimeout)
				}
				// Disable the timeout if user specifies negative value. Setting 0 disables the timeout in Envoy.
				if destination.IdleTimeout < 0 {
					routeDestinationConfiguration.TimeoutConfig.IdleTimeout = durationpb.New(0 * time.Second)
				}

				if destination.HasRetryFeatures() {
					routeDestinationConfiguration.RetryPolicy = getRetryPolicyForDestination(destination)
				}

				if err := injectHeaderManipToRoute(destination, routeRule); err != nil {
					return nil, fmt.Errorf("failed to apply header manipulation configuration to route: %v", err)
				}
			}

			routeRule.Match = routeMatch
			routeRule.Destination = routeDestination

			routeRules = append(routeRules, routeRule)
		}

	case structs.DiscoveryGraphNodeTypeSplitter:
		routeDestination, err := s.makeRouteDestinationForSplitter(upstreamsSnapshot, startNode.Splits, chain, forMeshGateway)
		if err != nil {
			return nil, err
		}
		var lb *structs.LoadBalancer
		if startNode.LoadBalancer != nil {
			lb = startNode.LoadBalancer
		}
		routeDestination.DestinationConfiguration = &pbproxystate.DestinationConfiguration{}
		if err := injectLBToDestinationConfiguration(lb, routeDestination.DestinationConfiguration); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
		}

		defaultRoute := &pbproxystate.RouteRule{
			Match:       makeDefaultRouteMatch(),
			Destination: routeDestination,
		}

		routeRules = []*pbproxystate.RouteRule{defaultRoute}

	case structs.DiscoveryGraphNodeTypeResolver:
		routeDestination, ok := s.makeRouteDestinationForChainCluster(upstreamsSnapshot, startNode.Resolver.Target, chain, forMeshGateway)
		if !ok {
			break
		}
		var lb *structs.LoadBalancer
		if startNode.LoadBalancer != nil {
			lb = startNode.LoadBalancer
		}
		routeDestination.DestinationConfiguration = &pbproxystate.DestinationConfiguration{}
		if err := injectLBToDestinationConfiguration(lb, routeDestination.DestinationConfiguration); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
		}

		// A request timeout can be configured on a resolver or router. If configured on a resolver, the timeout will
		// only apply if the start node is a resolver. This is because the timeout is attached to an (Envoy
		// RouteAction)[https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto#envoy-v3-api-msg-config-route-v3-routeaction]
		// If there is a splitter before this resolver, the branches of the split are configured within the same
		// RouteAction, and the timeout cannot be shared between branches of a split.
		if startNode.Resolver.RequestTimeout > 0 {
			to := &pbproxystate.TimeoutConfig{
				Timeout: durationpb.New(startNode.Resolver.RequestTimeout),
			}
			routeDestination.DestinationConfiguration.TimeoutConfig = to
		}
		// Disable the timeout if user specifies negative value. Setting 0 disables the timeout in Envoy.
		if startNode.Resolver.RequestTimeout < 0 {
			to := &pbproxystate.TimeoutConfig{
				Timeout: durationpb.New(0 * time.Second),
			}
			routeDestination.DestinationConfiguration.TimeoutConfig = to
		}
		defaultRoute := &pbproxystate.RouteRule{
			Match:       makeDefaultRouteMatch(),
			Destination: routeDestination,
		}

		routeRules = []*pbproxystate.RouteRule{defaultRoute}

	default:
		return nil, fmt.Errorf("unknown first node in discovery chain of type: %s", startNode.Type)
	}

	host := &pbproxystate.VirtualHost{
		Name:       uid.EnvoyID(),
		Domains:    serviceDomains,
		RouteRules: routeRules,
	}

	return host, nil
}

func getRetryPolicyForDestination(destination *structs.ServiceRouteDestination) *pbproxystate.RetryPolicy {
	retryPolicy := &pbproxystate.RetryPolicy{}
	if destination.NumRetries > 0 {
		retryPolicy.NumRetries = response.MakeUint32Value(int(destination.NumRetries))
	}

	// The RetryOn magic values come from: https://www.envoyproxy.io/docs/envoy/v1.10.0/configuration/http_filters/router_filter#config-http-filters-router-x-envoy-retry-on
	var retryStrings []string

	if len(destination.RetryOn) > 0 {
		retryStrings = append(retryStrings, destination.RetryOn...)
	}

	if destination.RetryOnConnectFailure {
		// connect-failure can be enabled by either adding connect-failure to the RetryOn list or by using the legacy RetryOnConnectFailure option
		// Check that it's not already in the RetryOn list, so we don't set it twice
		connectFailureExists := false
		for _, r := range retryStrings {
			if r == "connect-failure" {
				connectFailureExists = true
			}
		}
		if !connectFailureExists {
			retryStrings = append(retryStrings, "connect-failure")
		}
	}

	if len(destination.RetryOnStatusCodes) > 0 {
		retryStrings = append(retryStrings, "retriable-status-codes")
		retryPolicy.RetriableStatusCodes = destination.RetryOnStatusCodes
	}

	retryPolicy.RetryOn = strings.Join(retryStrings, ",")

	return retryPolicy
}

func makeRouteMatchForDiscoveryRoute(discoveryRoute *structs.DiscoveryRoute) *pbproxystate.RouteMatch {
	match := discoveryRoute.Definition.Match
	if match == nil || match.IsEmpty() {
		return makeDefaultRouteMatch()
	}

	routeMatch := &pbproxystate.RouteMatch{}

	switch {
	case match.HTTP.PathExact != "":
		routeMatch.PathMatch = &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Exact{
				Exact: match.HTTP.PathExact,
			},
		}
	case match.HTTP.PathPrefix != "":
		routeMatch.PathMatch = &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Prefix{
				Prefix: match.HTTP.PathPrefix,
			},
		}
	case match.HTTP.PathRegex != "":
		routeMatch.PathMatch = &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Regex{
				Regex: match.HTTP.PathRegex,
			},
		}
	default:
		routeMatch.PathMatch = &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Prefix{
				Prefix: "/",
			},
		}
	}

	if len(match.HTTP.Header) > 0 {
		routeMatch.HeaderMatches = make([]*pbproxystate.HeaderMatch, 0, len(match.HTTP.Header))
		for _, hdr := range match.HTTP.Header {
			headerMatch := &pbproxystate.HeaderMatch{
				Name: hdr.Name,
			}

			switch {
			case hdr.Exact != "":
				headerMatch.Match = &pbproxystate.HeaderMatch_Exact{
					Exact: hdr.Exact,
				}
			case hdr.Regex != "":
				headerMatch.Match = &pbproxystate.HeaderMatch_Regex{
					Regex: hdr.Regex,
				}
			case hdr.Prefix != "":
				headerMatch.Match = &pbproxystate.HeaderMatch_Prefix{
					Prefix: hdr.Prefix,
				}
			case hdr.Suffix != "":
				headerMatch.Match = &pbproxystate.HeaderMatch_Suffix{
					Suffix: hdr.Suffix,
				}
			case hdr.Present:
				headerMatch.Match = &pbproxystate.HeaderMatch_Present{
					Present: true,
				}
			default:
				continue // skip this impossible situation
			}

			if hdr.Invert {
				headerMatch.InvertMatch = true
			}

			routeMatch.HeaderMatches = append(routeMatch.HeaderMatches, headerMatch)
		}
	}

	if len(match.HTTP.Methods) > 0 {
		routeMatch.MethodMatches = append(routeMatch.MethodMatches, match.HTTP.Methods...)
	}

	if len(match.HTTP.QueryParam) > 0 {
		routeMatch.QueryParameterMatches = make([]*pbproxystate.QueryParameterMatch, 0, len(match.HTTP.QueryParam))
		for _, qm := range match.HTTP.QueryParam {

			queryMatcher := &pbproxystate.QueryParameterMatch{
				Name: qm.Name,
			}

			switch {
			case qm.Exact != "":
				queryMatcher.Match = &pbproxystate.QueryParameterMatch_Exact{
					Exact: qm.Exact,
				}
			case qm.Regex != "":
				queryMatcher.Match = &pbproxystate.QueryParameterMatch_Regex{
					Regex: qm.Regex,
				}
			case qm.Present:
				queryMatcher.Match = &pbproxystate.QueryParameterMatch_Present{
					Present: true,
				}
			default:
				continue // skip this impossible situation
			}

			routeMatch.QueryParameterMatches = append(routeMatch.QueryParameterMatches, queryMatcher)
		}
	}

	return routeMatch
}

func makeDefaultRouteMatch() *pbproxystate.RouteMatch {
	return &pbproxystate.RouteMatch{
		PathMatch: &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Prefix{
				Prefix: "/",
			},
		},
		// TODO(banks) Envoy supports matching only valid GRPC
		// requests which might be nice to add here for gRPC services
		// but it's not supported in our current envoy SDK version
		// although docs say it was supported by 1.8.0. Going to defer
		// that until we've updated the deps.
	}
}

func (s *Converter) makeRouteDestinationForChainCluster(
	upstreamsSnapshot *proxycfg.ConfigSnapshotUpstreams,
	targetID string,
	chain *structs.CompiledDiscoveryChain,
	forMeshGateway bool,
) (*pbproxystate.RouteDestination, bool) {
	clusterName := s.getTargetClusterName(upstreamsSnapshot, chain, targetID, forMeshGateway)
	if clusterName == "" {
		return nil, false
	}
	return makeRouteDestinationFromName(clusterName), true
}

func (s *Converter) makeRouteDestinationForSplitter(
	upstreamsSnapshot *proxycfg.ConfigSnapshotUpstreams,
	splits []*structs.DiscoverySplit,
	chain *structs.CompiledDiscoveryChain,
	forMeshGateway bool,
) (*pbproxystate.RouteDestination, error) {
	clusters := make([]*pbproxystate.L7WeightedDestinationCluster, 0, len(splits))
	for _, split := range splits {
		nextNode := chain.Nodes[split.NextNode]

		if nextNode.Type != structs.DiscoveryGraphNodeTypeResolver {
			return nil, fmt.Errorf("unexpected splitter destination node type: %s", nextNode.Type)
		}
		targetID := nextNode.Resolver.Target

		clusterName := s.getTargetClusterName(upstreamsSnapshot, chain, targetID, forMeshGateway)
		if clusterName == "" {
			continue
		}

		// The smallest representable weight is 1/10000 or .01% but envoy
		// deals with integers so scale everything up by 100x.
		weight := int(split.Weight * 100)

		clusterWeight := &pbproxystate.L7WeightedDestinationCluster{
			Name:   clusterName,
			Weight: response.MakeUint32Value(weight),
		}
		if err := injectHeaderManipToWeightedCluster(split.Definition, clusterWeight); err != nil {
			return nil, err
		}

		clusters = append(clusters, clusterWeight)
	}

	if len(clusters) <= 0 {
		return nil, fmt.Errorf("number of clusters in splitter must be > 0; got %d", len(clusters))
	}

	return &pbproxystate.RouteDestination{
		Destination: &pbproxystate.RouteDestination_WeightedClusters{
			WeightedClusters: &pbproxystate.L7WeightedClusterGroup{
				Clusters: clusters,
			},
		},
	}, nil
}

func injectLBToDestinationConfiguration(lb *structs.LoadBalancer, destinationConfig *pbproxystate.DestinationConfiguration) error {
	if lb == nil || !lb.IsHashBased() {
		return nil
	}

	result := make([]*pbproxystate.LoadBalancerHashPolicy, 0, len(lb.HashPolicies))
	for _, policy := range lb.HashPolicies {
		if policy.SourceIP {
			p := &pbproxystate.LoadBalancerHashPolicy{
				Policy: &pbproxystate.LoadBalancerHashPolicy_ConnectionProperties{
					ConnectionProperties: &pbproxystate.ConnectionPropertiesPolicy{
						SourceIp: true,
						Terminal: policy.Terminal,
					},
				},
			}
			result = append(result, p)
			continue
		}

		switch policy.Field {
		case structs.HashPolicyHeader:
			p := &pbproxystate.LoadBalancerHashPolicy{
				Policy: &pbproxystate.LoadBalancerHashPolicy_Header{
					Header: &pbproxystate.HeaderPolicy{
						Name:     policy.FieldValue,
						Terminal: policy.Terminal,
					},
				},
			}
			result = append(result, p)

		case structs.HashPolicyCookie:

			cookie := &pbproxystate.CookiePolicy{
				Name:     policy.FieldValue,
				Terminal: policy.Terminal,
			}
			if policy.CookieConfig != nil {
				cookie.Path = policy.CookieConfig.Path

				if policy.CookieConfig.TTL != 0*time.Second {
					cookie.Ttl = durationpb.New(policy.CookieConfig.TTL)
				}

				// Envoy will generate a session cookie if the ttl is present and zero.
				if policy.CookieConfig.Session {
					cookie.Ttl = durationpb.New(0 * time.Second)
				}
			}
			p := &pbproxystate.LoadBalancerHashPolicy{
				Policy: &pbproxystate.LoadBalancerHashPolicy_Cookie{
					Cookie: cookie,
				},
			}
			result = append(result, p)

		case structs.HashPolicyQueryParam:
			p := &pbproxystate.LoadBalancerHashPolicy{
				Policy: &pbproxystate.LoadBalancerHashPolicy_QueryParameter{
					QueryParameter: &pbproxystate.QueryParameterPolicy{
						Name:     policy.FieldValue,
						Terminal: policy.Terminal,
					},
				},
			}
			result = append(result, p)

		default:
			return fmt.Errorf("unsupported load balancer hash policy field: %v", policy.Field)
		}
	}

	destinationConfig.HashPolicies = result
	return nil
}

func injectHeaderManipToRoute(dest *structs.ServiceRouteDestination, r *pbproxystate.RouteRule) error {
	if !dest.RequestHeaders.IsZero() {
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeRequestHeaderAdd(dest.RequestHeaders.Add, true)...,
		)
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeRequestHeaderAdd(dest.RequestHeaders.Set, false)...,
		)
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeRequestHeaderRemove(dest.RequestHeaders.Remove),
		)
	}
	if !dest.ResponseHeaders.IsZero() {
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeResponseHeaderAdd(dest.ResponseHeaders.Add, true)...,
		)
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeResponseHeaderAdd(dest.ResponseHeaders.Set, false)...,
		)
		r.HeaderMutations = append(
			r.HeaderMutations,
			makeResponseHeaderRemove(dest.ResponseHeaders.Remove),
		)
	}
	return nil
}

func injectHeaderManipToWeightedCluster(split *structs.ServiceSplit, c *pbproxystate.L7WeightedDestinationCluster) error {
	if !split.RequestHeaders.IsZero() {
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeRequestHeaderAdd(split.RequestHeaders.Add, true)...,
		)
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeRequestHeaderAdd(split.RequestHeaders.Set, false)...,
		)
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeRequestHeaderRemove(split.RequestHeaders.Remove),
		)
	}
	if !split.ResponseHeaders.IsZero() {
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeResponseHeaderAdd(split.ResponseHeaders.Add, true)...,
		)
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeResponseHeaderAdd(split.ResponseHeaders.Set, false)...,
		)
		c.HeaderMutations = append(
			c.HeaderMutations,
			makeResponseHeaderRemove(split.ResponseHeaders.Remove),
		)
	}
	return nil
}

func makeRequestHeaderAdd(vals map[string]string, add bool) []*pbproxystate.HeaderMutation {
	mutations := make([]*pbproxystate.HeaderMutation, 0, len(vals))

	appendAction := pbproxystate.AppendAction_APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD
	if add {
		appendAction = pbproxystate.AppendAction_APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD
	}

	for k, v := range vals {
		m := &pbproxystate.HeaderMutation{
			Action: &pbproxystate.HeaderMutation_RequestHeaderAdd{
				RequestHeaderAdd: &pbproxystate.RequestHeaderAdd{
					Header: &pbproxystate.Header{
						Key:   k,
						Value: v,
					},
					AppendAction: appendAction,
				},
			},
		}
		mutations = append(mutations, m)
	}
	return mutations
}

func makeRequestHeaderRemove(values []string) *pbproxystate.HeaderMutation {
	return &pbproxystate.HeaderMutation{
		Action: &pbproxystate.HeaderMutation_RequestHeaderRemove{
			RequestHeaderRemove: &pbproxystate.RequestHeaderRemove{
				HeaderKeys: values,
			},
		},
	}
}

func makeResponseHeaderAdd(vals map[string]string, add bool) []*pbproxystate.HeaderMutation {
	mutations := make([]*pbproxystate.HeaderMutation, 0, len(vals))

	appendAction := pbproxystate.AppendAction_APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD
	if add {
		appendAction = pbproxystate.AppendAction_APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD
	}

	for k, v := range vals {
		m := &pbproxystate.HeaderMutation{
			Action: &pbproxystate.HeaderMutation_ResponseHeaderAdd{
				ResponseHeaderAdd: &pbproxystate.ResponseHeaderAdd{
					Header: &pbproxystate.Header{
						Key:   k,
						Value: v,
					},
					AppendAction: appendAction,
				},
			},
		}
		mutations = append(mutations, m)
	}
	return mutations
}

func makeResponseHeaderRemove(values []string) *pbproxystate.HeaderMutation {
	return &pbproxystate.HeaderMutation{
		Action: &pbproxystate.HeaderMutation_ResponseHeaderRemove{
			ResponseHeaderRemove: &pbproxystate.ResponseHeaderRemove{
				HeaderKeys: values,
			},
		},
	}
}
