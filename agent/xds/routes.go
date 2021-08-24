package xds

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func (s *ResourceGenerator) routesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.routesForConnectProxy(cfgSnap.ConnectProxy.DiscoveryChain)
	case structs.ServiceKindIngressGateway:
		return s.routesForIngressGateway(
			cfgSnap.IngressGateway.Listeners,
			cfgSnap.IngressGateway.Upstreams,
			cfgSnap.IngressGateway.DiscoveryChain,
		)
	case structs.ServiceKindTerminatingGateway:
		return s.routesFromSnapshotTerminatingGateway(cfgSnap)
	case structs.ServiceKindMeshGateway:
		return nil, nil // mesh gateways will never have routes
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func (s *ResourceGenerator) routesForConnectProxy(chains map[string]*structs.CompiledDiscoveryChain) ([]proto.Message, error) {
	var resources []proto.Message
	for id, chain := range chains {
		if chain.IsDefault() {
			continue
		}

		virtualHost, err := makeUpstreamRouteForDiscoveryChain(id, chain, []string{"*"})
		if err != nil {
			return nil, err
		}

		route := &envoy_route_v3.RouteConfiguration{
			Name:         id,
			VirtualHosts: []*envoy_route_v3.VirtualHost{virtualHost},
			// ValidateClusters defaults to true when defined statically and false
			// when done via RDS. Re-set the reasonable value of true to prevent
			// null-routing traffic.
			ValidateClusters: makeBoolValue(true),
		}
		resources = append(resources, route)
	}

	// TODO(rb): make sure we don't generate an empty result
	return resources, nil
}

// routesFromSnapshotTerminatingGateway returns the xDS API representation of the "routes" in the snapshot.
// For any HTTP service we will return a default route.
func (s *ResourceGenerator) routesFromSnapshotTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	var resources []proto.Message
	for _, svc := range cfgSnap.TerminatingGateway.ValidServices() {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
		resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]

		svcConfig := cfgSnap.TerminatingGateway.ServiceConfigs[svc]

		cfg, err := ParseProxyConfig(svcConfig.ProxyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream config: %v", err)
		}
		if !structs.IsProtocolHTTPLike(cfg.Protocol) {
			// Routes can only be defined for HTTP services
			continue
		}

		if !hasResolver {
			// Use a zero value resolver with no timeout and no subsets
			resolver = &structs.ServiceResolverConfigEntry{}
		}

		var lb *structs.LoadBalancer
		if resolver.LoadBalancer != nil {
			lb = resolver.LoadBalancer
		}
		route, err := makeNamedDefaultRouteWithLB(clusterName, lb, true)
		if err != nil {
			s.Logger.Error("failed to make route", "cluster", clusterName, "error", err)
			continue
		}
		resources = append(resources, route)

		// If there is a service-resolver for this service then also setup routes for each subset
		for name := range resolver.Subsets {
			clusterName = connect.ServiceSNI(svc.Name, name, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
			route, err := makeNamedDefaultRouteWithLB(clusterName, lb, true)
			if err != nil {
				s.Logger.Error("failed to make route", "cluster", clusterName, "error", err)
				continue
			}
			resources = append(resources, route)
		}
	}

	return resources, nil
}

func makeNamedDefaultRouteWithLB(clusterName string, lb *structs.LoadBalancer, autoHostRewrite bool) (*envoy_route_v3.RouteConfiguration, error) {
	action := makeRouteActionFromName(clusterName)

	if err := injectLBToRouteAction(lb, action.Route); err != nil {
		return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
	}

	// Configure Envoy to rewrite Host header
	if autoHostRewrite {
		action.Route.HostRewriteSpecifier = &envoy_route_v3.RouteAction_AutoHostRewrite{
			AutoHostRewrite: makeBoolValue(true),
		}
	}

	return &envoy_route_v3.RouteConfiguration{
		Name: clusterName,
		VirtualHosts: []*envoy_route_v3.VirtualHost{
			{
				Name:    clusterName,
				Domains: []string{"*"},
				Routes: []*envoy_route_v3.Route{
					{
						Match:  makeDefaultRouteMatch(),
						Action: action,
					},
				},
			},
		},
		// ValidateClusters defaults to true when defined statically and false
		// when done via RDS. Re-set the reasonable value of true to prevent
		// null-routing traffic.
		ValidateClusters: makeBoolValue(true),
	}, nil
}

// routesForIngressGateway returns the xDS API representation of the
// "routes" in the snapshot.
func (s *ResourceGenerator) routesForIngressGateway(
	listeners map[proxycfg.IngressListenerKey]structs.IngressListener,
	upstreams map[proxycfg.IngressListenerKey]structs.Upstreams,
	chains map[string]*structs.CompiledDiscoveryChain,
) ([]proto.Message, error) {

	var result []proto.Message
	for listenerKey, upstreams := range upstreams {
		// Do not create any route configuration for TCP listeners
		if listenerKey.Protocol == "tcp" {
			continue
		}

		// Depending on their TLS config, upstreams are either attached to the
		// default route or have their own routes. We'll add any upstreams that
		// don't have custom filter chains and routes to this.
		defaultRoute := &envoy_route_v3.RouteConfiguration{
			Name: listenerKey.RouteName(),
			// ValidateClusters defaults to true when defined statically and false
			// when done via RDS. Re-set the reasonable value of true to prevent
			// null-routing traffic.
			ValidateClusters: makeBoolValue(true),
		}

		for _, u := range upstreams {
			upstreamID := u.Identifier()
			chain := chains[upstreamID]
			if chain == nil {
				continue
			}

			domains := generateUpstreamIngressDomains(listenerKey, u)
			virtualHost, err := makeUpstreamRouteForDiscoveryChain(upstreamID, chain, domains)
			if err != nil {
				return nil, err
			}

			// Lookup listener and service config details from ingress gateway
			// definition.
			lCfg, ok := listeners[listenerKey]
			if !ok {
				return nil, fmt.Errorf("missing ingress listener config (listener on port %d)", listenerKey.Port)
			}
			svc := findIngressServiceMatchingUpstream(lCfg, u)
			if svc == nil {
				return nil, fmt.Errorf("missing service in listener config (service %q listener on port %d)",
					u.DestinationID(), listenerKey.Port)
			}

			if err := injectHeaderManipToVirtualHost(svc, virtualHost); err != nil {
				return nil, err
			}

			// See if this upstream has it's own route/filter chain
			svcRouteName, err := routeNameForUpstream(lCfg, *svc)
			if err != nil {
				return nil, err
			}

			// If the routeName is the same as the default one, merge the virtual host
			// to the default route
			if svcRouteName == defaultRoute.Name {
				defaultRoute.VirtualHosts = append(defaultRoute.VirtualHosts, virtualHost)
			} else {
				svcRoute := &envoy_route_v3.RouteConfiguration{
					Name:             svcRouteName,
					ValidateClusters: makeBoolValue(true),
					VirtualHosts:     []*envoy_route_v3.VirtualHost{virtualHost},
				}
				result = append(result, svcRoute)
			}
		}

		if len(defaultRoute.VirtualHosts) > 0 {
			result = append(result, defaultRoute)
		}
	}

	return result, nil
}

func makeHeadersValueOptions(vals map[string]string, add bool) []*envoy_core_v3.HeaderValueOption {
	opts := make([]*envoy_core_v3.HeaderValueOption, 0, len(vals))
	for k, v := range vals {
		o := &envoy_core_v3.HeaderValueOption{
			Header: &envoy_core_v3.HeaderValue{
				Key:   k,
				Value: v,
			},
			Append: makeBoolValue(add),
		}
		opts = append(opts, o)
	}
	return opts
}

func findIngressServiceMatchingUpstream(l structs.IngressListener, u structs.Upstream) *structs.IngressService {
	// Hunt through for the matching service. We validate now that there is
	// only one IngressService for each unique name although originally that
	// wasn't checked as it didn't matter. Assume there is only one now
	// though!
	wantSID := u.DestinationID()
	var foundSameNSWildcard *structs.IngressService
	for _, s := range l.Services {
		sid := structs.NewServiceID(s.Name, &s.EnterpriseMeta)
		if wantSID.Matches(sid) {
			return &s
		}
		if s.Name == structs.WildcardSpecifier &&
			s.NamespaceOrDefault() == wantSID.NamespaceOrDefault() {
			foundSameNSWildcard = &s
		}
	}
	// Didn't find an exact match. Return the wildcard from same service if we
	// found one.
	return foundSameNSWildcard
}

func generateUpstreamIngressDomains(listenerKey proxycfg.IngressListenerKey, u structs.Upstream) []string {
	var domains []string
	domainsSet := make(map[string]bool)

	namespace := u.GetEnterpriseMeta().NamespaceOrDefault()
	switch {
	case len(u.IngressHosts) > 0:
		// If a user has specified hosts, do not add the default
		// "<service-name>.ingress.*" prefixes
		domains = u.IngressHosts
	case namespace != structs.IntentionDefaultNamespace:
		domains = []string{fmt.Sprintf("%s.ingress.%s.*", u.DestinationName, namespace)}
	default:
		domains = []string{fmt.Sprintf("%s.ingress.*", u.DestinationName)}
	}

	for _, h := range domains {
		domainsSet[h] = true
	}

	// Host headers may contain port numbers in them, so we need to make sure
	// we match on the host with and without the port number. Well-known
	// ports like HTTP/HTTPS are stripped from Host headers, but other ports
	// will be in the header.
	for _, h := range domains {
		_, _, err := net.SplitHostPort(h)
		// Error message from Go's net/ipsock.go
		// We check to see if a port is not missing, and ignore the
		// error from SplitHostPort otherwise, since we have previously
		// validated the Host values and should trust the user's input.
		if err == nil || !strings.Contains(err.Error(), "missing port in address") {
			continue
		}

		domainWithPort := fmt.Sprintf("%s:%d", h, listenerKey.Port)

		// Do not add a duplicate domain if a hostname with port is already in the
		// set
		if !domainsSet[domainWithPort] {
			domains = append(domains, domainWithPort)
		}
	}

	return domains
}

func makeUpstreamRouteForDiscoveryChain(
	routeName string,
	chain *structs.CompiledDiscoveryChain,
	serviceDomains []string,
) (*envoy_route_v3.VirtualHost, error) {
	var routes []*envoy_route_v3.Route

	startNode := chain.Nodes[chain.StartNode]
	if startNode == nil {
		return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
	}

	switch startNode.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		routes = make([]*envoy_route_v3.Route, 0, len(startNode.Routes))

		for _, discoveryRoute := range startNode.Routes {
			routeMatch := makeRouteMatchForDiscoveryRoute(discoveryRoute)

			var (
				routeAction *envoy_route_v3.Route_Route
				err         error
			)

			nextNode := chain.Nodes[discoveryRoute.NextNode]

			var lb *structs.LoadBalancer
			if nextNode.LoadBalancer != nil {
				lb = nextNode.LoadBalancer
			}

			switch nextNode.Type {
			case structs.DiscoveryGraphNodeTypeSplitter:
				routeAction, err = makeRouteActionForSplitter(nextNode.Splits, chain)
				if err != nil {
					return nil, err
				}

			case structs.DiscoveryGraphNodeTypeResolver:
				routeAction = makeRouteActionForChainCluster(nextNode.Resolver.Target, chain)

			default:
				return nil, fmt.Errorf("unexpected graph node after route %q", nextNode.Type)
			}

			if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
				return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
			}

			// TODO(rb): Better help handle the envoy case where you need (prefix=/foo/,rewrite=/) and (exact=/foo,rewrite=/) to do a full rewrite

			destination := discoveryRoute.Definition.Destination

			route := &envoy_route_v3.Route{}

			if destination != nil {
				if destination.PrefixRewrite != "" {
					routeAction.Route.PrefixRewrite = destination.PrefixRewrite
				}

				if destination.RequestTimeout > 0 {
					routeAction.Route.Timeout = ptypes.DurationProto(destination.RequestTimeout)
				}

				if destination.HasRetryFeatures() {
					retryPolicy := &envoy_route_v3.RetryPolicy{}
					if destination.NumRetries > 0 {
						retryPolicy.NumRetries = makeUint32Value(int(destination.NumRetries))
					}

					// The RetryOn magic values come from: https://www.envoyproxy.io/docs/envoy/v1.10.0/configuration/http_filters/router_filter#config-http-filters-router-x-envoy-retry-on
					if destination.RetryOnConnectFailure {
						retryPolicy.RetryOn = "connect-failure"
					}
					if len(destination.RetryOnStatusCodes) > 0 {
						if retryPolicy.RetryOn != "" {
							retryPolicy.RetryOn = retryPolicy.RetryOn + ",retriable-status-codes"
						} else {
							retryPolicy.RetryOn = "retriable-status-codes"
						}
						retryPolicy.RetriableStatusCodes = destination.RetryOnStatusCodes
					}

					routeAction.Route.RetryPolicy = retryPolicy
				}

				if err := injectHeaderManipToRoute(destination, route); err != nil {
					return nil, fmt.Errorf("failed to apply header manipulation configuration to route: %v", err)
				}
			}

			route.Match = routeMatch
			route.Action = routeAction

			routes = append(routes, route)
		}

	case structs.DiscoveryGraphNodeTypeSplitter:
		routeAction, err := makeRouteActionForSplitter(startNode.Splits, chain)
		if err != nil {
			return nil, err
		}

		var lb *structs.LoadBalancer
		if startNode.LoadBalancer != nil {
			lb = startNode.LoadBalancer
		}
		if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
		}

		defaultRoute := &envoy_route_v3.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoy_route_v3.Route{defaultRoute}

	case structs.DiscoveryGraphNodeTypeResolver:
		routeAction := makeRouteActionForChainCluster(startNode.Resolver.Target, chain)

		var lb *structs.LoadBalancer
		if startNode.LoadBalancer != nil {
			lb = startNode.LoadBalancer
		}
		if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
		}

		defaultRoute := &envoy_route_v3.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoy_route_v3.Route{defaultRoute}

	default:
		return nil, fmt.Errorf("unknown first node in discovery chain of type: %s", startNode.Type)
	}

	host := &envoy_route_v3.VirtualHost{
		Name:    routeName,
		Domains: serviceDomains,
		Routes:  routes,
	}

	return host, nil
}

func makeRouteMatchForDiscoveryRoute(discoveryRoute *structs.DiscoveryRoute) *envoy_route_v3.RouteMatch {
	match := discoveryRoute.Definition.Match
	if match == nil || match.IsEmpty() {
		return makeDefaultRouteMatch()
	}

	em := &envoy_route_v3.RouteMatch{}

	switch {
	case match.HTTP.PathExact != "":
		em.PathSpecifier = &envoy_route_v3.RouteMatch_Path{
			Path: match.HTTP.PathExact,
		}
	case match.HTTP.PathPrefix != "":
		em.PathSpecifier = &envoy_route_v3.RouteMatch_Prefix{
			Prefix: match.HTTP.PathPrefix,
		}
	case match.HTTP.PathRegex != "":
		em.PathSpecifier = &envoy_route_v3.RouteMatch_SafeRegex{
			SafeRegex: makeEnvoyRegexMatch(match.HTTP.PathRegex),
		}
	default:
		em.PathSpecifier = &envoy_route_v3.RouteMatch_Prefix{
			Prefix: "/",
		}
	}

	if len(match.HTTP.Header) > 0 {
		em.Headers = make([]*envoy_route_v3.HeaderMatcher, 0, len(match.HTTP.Header))
		for _, hdr := range match.HTTP.Header {
			eh := &envoy_route_v3.HeaderMatcher{
				Name: hdr.Name,
			}

			switch {
			case hdr.Exact != "":
				eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_ExactMatch{
					ExactMatch: hdr.Exact,
				}
			case hdr.Regex != "":
				eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_SafeRegexMatch{
					SafeRegexMatch: makeEnvoyRegexMatch(hdr.Regex),
				}
			case hdr.Prefix != "":
				eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_PrefixMatch{
					PrefixMatch: hdr.Prefix,
				}
			case hdr.Suffix != "":
				eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_SuffixMatch{
					SuffixMatch: hdr.Suffix,
				}
			case hdr.Present:
				eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_PresentMatch{
					PresentMatch: true,
				}
			default:
				continue // skip this impossible situation
			}

			if hdr.Invert {
				eh.InvertMatch = true
			}

			em.Headers = append(em.Headers, eh)
		}
	}

	if len(match.HTTP.Methods) > 0 {
		methodHeaderRegex := strings.Join(match.HTTP.Methods, "|")

		eh := &envoy_route_v3.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: makeEnvoyRegexMatch(methodHeaderRegex),
			},
		}

		em.Headers = append(em.Headers, eh)
	}

	if len(match.HTTP.QueryParam) > 0 {
		em.QueryParameters = make([]*envoy_route_v3.QueryParameterMatcher, 0, len(match.HTTP.QueryParam))
		for _, qm := range match.HTTP.QueryParam {
			eq := &envoy_route_v3.QueryParameterMatcher{
				Name: qm.Name,
			}

			switch {
			case qm.Exact != "":
				eq.QueryParameterMatchSpecifier = &envoy_route_v3.QueryParameterMatcher_StringMatch{
					StringMatch: &envoy_matcher_v3.StringMatcher{
						MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
							Exact: qm.Exact,
						},
					},
				}
			case qm.Regex != "":
				eq.QueryParameterMatchSpecifier = &envoy_route_v3.QueryParameterMatcher_StringMatch{
					StringMatch: &envoy_matcher_v3.StringMatcher{
						MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
							SafeRegex: makeEnvoyRegexMatch(qm.Regex),
						},
					},
				}
			case qm.Present:
				eq.QueryParameterMatchSpecifier = &envoy_route_v3.QueryParameterMatcher_PresentMatch{
					PresentMatch: true,
				}
			default:
				continue // skip this impossible situation
			}

			em.QueryParameters = append(em.QueryParameters, eq)
		}
	}

	return em
}

func makeDefaultRouteMatch() *envoy_route_v3.RouteMatch {
	return &envoy_route_v3.RouteMatch{
		PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
			Prefix: "/",
		},
		// TODO(banks) Envoy supports matching only valid GRPC
		// requests which might be nice to add here for gRPC services
		// but it's not supported in our current envoy SDK version
		// although docs say it was supported by 1.8.0. Going to defer
		// that until we've updated the deps.
	}
}

func makeRouteActionForChainCluster(targetID string, chain *structs.CompiledDiscoveryChain) *envoy_route_v3.Route_Route {
	target := chain.Targets[targetID]
	return makeRouteActionFromName(CustomizeClusterName(target.Name, chain))
}

func makeRouteActionFromName(clusterName string) *envoy_route_v3.Route_Route {
	return &envoy_route_v3.Route_Route{
		Route: &envoy_route_v3.RouteAction{
			ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{
				Cluster: clusterName,
			},
		},
	}
}

func makeRouteActionForSplitter(splits []*structs.DiscoverySplit, chain *structs.CompiledDiscoveryChain) (*envoy_route_v3.Route_Route, error) {
	clusters := make([]*envoy_route_v3.WeightedCluster_ClusterWeight, 0, len(splits))
	for _, split := range splits {
		nextNode := chain.Nodes[split.NextNode]

		if nextNode.Type != structs.DiscoveryGraphNodeTypeResolver {
			return nil, fmt.Errorf("unexpected splitter destination node type: %s", nextNode.Type)
		}
		targetID := nextNode.Resolver.Target

		target := chain.Targets[targetID]

		clusterName := CustomizeClusterName(target.Name, chain)

		// The smallest representable weight is 1/10000 or .01% but envoy
		// deals with integers so scale everything up by 100x.
		cw := &envoy_route_v3.WeightedCluster_ClusterWeight{
			Weight: makeUint32Value(int(split.Weight * 100)),
			Name:   clusterName,
		}
		if err := injectHeaderManipToWeightedCluster(split.Definition, cw); err != nil {
			return nil, err
		}

		clusters = append(clusters, cw)
	}

	return &envoy_route_v3.Route_Route{
		Route: &envoy_route_v3.RouteAction{
			ClusterSpecifier: &envoy_route_v3.RouteAction_WeightedClusters{
				WeightedClusters: &envoy_route_v3.WeightedCluster{
					Clusters:    clusters,
					TotalWeight: makeUint32Value(10000), // scaled up 100%
				},
			},
		},
	}, nil
}

func injectLBToRouteAction(lb *structs.LoadBalancer, action *envoy_route_v3.RouteAction) error {
	if lb == nil || !lb.IsHashBased() {
		return nil
	}

	result := make([]*envoy_route_v3.RouteAction_HashPolicy, 0, len(lb.HashPolicies))
	for _, policy := range lb.HashPolicies {
		if policy.SourceIP {
			result = append(result, &envoy_route_v3.RouteAction_HashPolicy{
				PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
					ConnectionProperties: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties{
						SourceIp: true,
					},
				},
				Terminal: policy.Terminal,
			})

			continue
		}

		switch policy.Field {
		case structs.HashPolicyHeader:
			result = append(result, &envoy_route_v3.RouteAction_HashPolicy{
				PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Header_{
					Header: &envoy_route_v3.RouteAction_HashPolicy_Header{
						HeaderName: policy.FieldValue,
					},
				},
				Terminal: policy.Terminal,
			})
		case structs.HashPolicyCookie:
			cookie := envoy_route_v3.RouteAction_HashPolicy_Cookie{
				Name: policy.FieldValue,
			}
			if policy.CookieConfig != nil {
				cookie.Path = policy.CookieConfig.Path

				if policy.CookieConfig.TTL != 0*time.Second {
					cookie.Ttl = ptypes.DurationProto(policy.CookieConfig.TTL)
				}

				// Envoy will generate a session cookie if the ttl is present and zero.
				if policy.CookieConfig.Session {
					cookie.Ttl = ptypes.DurationProto(0 * time.Second)
				}
			}
			result = append(result, &envoy_route_v3.RouteAction_HashPolicy{
				PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
					Cookie: &cookie,
				},
				Terminal: policy.Terminal,
			})
		case structs.HashPolicyQueryParam:
			result = append(result, &envoy_route_v3.RouteAction_HashPolicy{
				PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter_{
					QueryParameter: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter{
						Name: policy.FieldValue,
					},
				},
				Terminal: policy.Terminal,
			})
		default:
			return fmt.Errorf("unsupported load balancer hash policy field: %v", policy.Field)
		}
	}
	action.HashPolicy = result
	return nil
}

func injectHeaderManipToRoute(dest *structs.ServiceRouteDestination, r *envoy_route_v3.Route) error {
	if !dest.RequestHeaders.IsZero() {
		r.RequestHeadersToAdd = append(
			r.RequestHeadersToAdd,
			makeHeadersValueOptions(dest.RequestHeaders.Add, true)...,
		)
		r.RequestHeadersToAdd = append(
			r.RequestHeadersToAdd,
			makeHeadersValueOptions(dest.RequestHeaders.Set, false)...,
		)
		r.RequestHeadersToRemove = append(
			r.RequestHeadersToRemove,
			dest.RequestHeaders.Remove...,
		)
	}
	if !dest.ResponseHeaders.IsZero() {
		r.ResponseHeadersToAdd = append(
			r.ResponseHeadersToAdd,
			makeHeadersValueOptions(dest.ResponseHeaders.Add, true)...,
		)
		r.ResponseHeadersToAdd = append(
			r.ResponseHeadersToAdd,
			makeHeadersValueOptions(dest.ResponseHeaders.Set, false)...,
		)
		r.ResponseHeadersToRemove = append(
			r.ResponseHeadersToRemove,
			dest.ResponseHeaders.Remove...,
		)
	}
	return nil
}

func injectHeaderManipToVirtualHost(dest *structs.IngressService, vh *envoy_route_v3.VirtualHost) error {
	if !dest.RequestHeaders.IsZero() {
		vh.RequestHeadersToAdd = append(
			vh.RequestHeadersToAdd,
			makeHeadersValueOptions(dest.RequestHeaders.Add, true)...,
		)
		vh.RequestHeadersToAdd = append(
			vh.RequestHeadersToAdd,
			makeHeadersValueOptions(dest.RequestHeaders.Set, false)...,
		)
		vh.RequestHeadersToRemove = append(
			vh.RequestHeadersToRemove,
			dest.RequestHeaders.Remove...,
		)
	}
	if !dest.ResponseHeaders.IsZero() {
		vh.ResponseHeadersToAdd = append(
			vh.ResponseHeadersToAdd,
			makeHeadersValueOptions(dest.ResponseHeaders.Add, true)...,
		)
		vh.ResponseHeadersToAdd = append(
			vh.ResponseHeadersToAdd,
			makeHeadersValueOptions(dest.ResponseHeaders.Set, false)...,
		)
		vh.ResponseHeadersToRemove = append(
			vh.ResponseHeadersToRemove,
			dest.ResponseHeaders.Remove...,
		)
	}
	return nil
}

func injectHeaderManipToWeightedCluster(split *structs.ServiceSplit, c *envoy_route_v3.WeightedCluster_ClusterWeight) error {
	if !split.RequestHeaders.IsZero() {
		c.RequestHeadersToAdd = append(
			c.RequestHeadersToAdd,
			makeHeadersValueOptions(split.RequestHeaders.Add, true)...,
		)
		c.RequestHeadersToAdd = append(
			c.RequestHeadersToAdd,
			makeHeadersValueOptions(split.RequestHeaders.Set, false)...,
		)
		c.RequestHeadersToRemove = append(
			c.RequestHeadersToRemove,
			split.RequestHeaders.Remove...,
		)
	}
	if !split.ResponseHeaders.IsZero() {
		c.ResponseHeadersToAdd = append(
			c.ResponseHeadersToAdd,
			makeHeadersValueOptions(split.ResponseHeaders.Add, true)...,
		)
		c.ResponseHeadersToAdd = append(
			c.ResponseHeadersToAdd,
			makeHeadersValueOptions(split.ResponseHeaders.Set, false)...,
		)
		c.ResponseHeadersToRemove = append(
			c.ResponseHeadersToRemove,
			split.ResponseHeaders.Remove...,
		)
	}
	return nil
}
