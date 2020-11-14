package xds

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoymatcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func (s *Server) routesFromSnapshot(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return routesForConnectProxy(cInfo, cfgSnap.Proxy.Upstreams, cfgSnap.ConnectProxy.DiscoveryChain)
	case structs.ServiceKindIngressGateway:
		return routesForIngressGateway(cInfo, cfgSnap.IngressGateway.Upstreams, cfgSnap.IngressGateway.DiscoveryChain)
	case structs.ServiceKindTerminatingGateway:
		return s.routesFromSnapshotTerminatingGateway(cInfo, cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func routesForConnectProxy(
	cInfo connectionInfo,
	upstreams structs.Upstreams,
	chains map[string]*structs.CompiledDiscoveryChain,
) ([]proto.Message, error) {

	var resources []proto.Message
	for _, u := range upstreams {
		upstreamID := u.Identifier()

		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = chains[upstreamID]
		}

		if chain == nil || chain.IsDefault() {
			// TODO(rb): make this do the old school stuff too
		} else {
			virtualHost, err := makeUpstreamRouteForDiscoveryChain(cInfo, upstreamID, chain, []string{"*"})
			if err != nil {
				return nil, err
			}

			route := &envoy.RouteConfiguration{
				Name:         upstreamID,
				VirtualHosts: []*envoyroute.VirtualHost{virtualHost},
				// ValidateClusters defaults to true when defined statically and false
				// when done via RDS. Re-set the sane value of true to prevent
				// null-routing traffic.
				ValidateClusters: makeBoolValue(true),
			}
			resources = append(resources, route)
		}
	}

	// TODO(rb): make sure we don't generate an empty result
	return resources, nil
}

// routesFromSnapshotTerminatingGateway returns the xDS API representation of the "routes" in the snapshot.
// For any HTTP service we will return a default route.
func (s *Server) routesFromSnapshotTerminatingGateway(_ connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	logger := s.Logger.Named(logging.TerminatingGateway)

	var resources []proto.Message
	for _, svc := range cfgSnap.TerminatingGateway.ValidServices() {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
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
		route, err := makeNamedDefaultRouteWithLB(clusterName, lb)
		if err != nil {
			logger.Error("failed to make route", "cluster", clusterName, "error", err)
			continue
		}
		resources = append(resources, route)

		// If there is a service-resolver for this service then also setup routes for each subset
		for name := range resolver.Subsets {
			clusterName = connect.ServiceSNI(svc.Name, name, svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
			route, err := makeNamedDefaultRouteWithLB(clusterName, lb)
			if err != nil {
				logger.Error("failed to make route", "cluster", clusterName, "error", err)
				continue
			}
			resources = append(resources, route)
		}
	}

	return resources, nil
}

func makeNamedDefaultRouteWithLB(clusterName string, lb *structs.LoadBalancer) (*envoy.RouteConfiguration, error) {
	action := makeRouteActionFromName(clusterName)

	if err := injectLBToRouteAction(lb, action.Route); err != nil {
		return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
	}

	return &envoy.RouteConfiguration{
		Name: clusterName,
		VirtualHosts: []*envoyroute.VirtualHost{
			{
				Name:    clusterName,
				Domains: []string{"*"},
				Routes: []*envoyroute.Route{
					{
						Match:  makeDefaultRouteMatch(),
						Action: action,
					},
				},
			},
		},
		// ValidateClusters defaults to true when defined statically and false
		// when done via RDS. Re-set the sane value of true to prevent
		// null-routing traffic.
		ValidateClusters: makeBoolValue(true),
	}, nil
}

// routesForIngressGateway returns the xDS API representation of the
// "routes" in the snapshot.
func routesForIngressGateway(
	cInfo connectionInfo,
	upstreams map[proxycfg.IngressListenerKey]structs.Upstreams,
	chains map[string]*structs.CompiledDiscoveryChain,
) ([]proto.Message, error) {

	var result []proto.Message
	for listenerKey, upstreams := range upstreams {
		// Do not create any route configuration for TCP listeners
		if listenerKey.Protocol == "tcp" {
			continue
		}

		upstreamRoute := &envoy.RouteConfiguration{
			Name: listenerKey.RouteName(),
			// ValidateClusters defaults to true when defined statically and false
			// when done via RDS. Re-set the sane value of true to prevent
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
			virtualHost, err := makeUpstreamRouteForDiscoveryChain(cInfo, upstreamID, chain, domains)
			if err != nil {
				return nil, err
			}
			upstreamRoute.VirtualHosts = append(upstreamRoute.VirtualHosts, virtualHost)
		}

		result = append(result, upstreamRoute)
	}

	return result, nil
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
	cInfo connectionInfo,
	routeName string,
	chain *structs.CompiledDiscoveryChain,
	serviceDomains []string,
) (*envoyroute.VirtualHost, error) {
	var routes []*envoyroute.Route

	startNode := chain.Nodes[chain.StartNode]
	if startNode == nil {
		return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
	}

	switch startNode.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		routes = make([]*envoyroute.Route, 0, len(startNode.Routes))

		for _, discoveryRoute := range startNode.Routes {
			routeMatch := makeRouteMatchForDiscoveryRoute(cInfo, discoveryRoute)

			var (
				routeAction *envoyroute.Route_Route
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

				if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
					return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
				}

			case structs.DiscoveryGraphNodeTypeResolver:
				routeAction = makeRouteActionForChainCluster(nextNode.Resolver.Target, chain)

				if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
					return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
				}

			default:
				return nil, fmt.Errorf("unexpected graph node after route %q", nextNode.Type)
			}

			// TODO(rb): Better help handle the envoy case where you need (prefix=/foo/,rewrite=/) and (exact=/foo,rewrite=/) to do a full rewrite

			destination := discoveryRoute.Definition.Destination
			if destination != nil {
				if destination.PrefixRewrite != "" {
					routeAction.Route.PrefixRewrite = destination.PrefixRewrite
				}

				if destination.RequestTimeout > 0 {
					routeAction.Route.Timeout = ptypes.DurationProto(destination.RequestTimeout)
				}

				if destination.HasRetryFeatures() {
					retryPolicy := &envoyroute.RetryPolicy{}
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
			}

			routes = append(routes, &envoyroute.Route{
				Match:  routeMatch,
				Action: routeAction,
			})
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

		defaultRoute := &envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoyroute.Route{defaultRoute}

	case structs.DiscoveryGraphNodeTypeResolver:
		routeAction := makeRouteActionForChainCluster(startNode.Resolver.Target, chain)

		var lb *structs.LoadBalancer
		if startNode.LoadBalancer != nil {
			lb = startNode.LoadBalancer
		}
		if err := injectLBToRouteAction(lb, routeAction.Route); err != nil {
			return nil, fmt.Errorf("failed to apply load balancer configuration to route action: %v", err)
		}

		defaultRoute := &envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoyroute.Route{defaultRoute}

	default:
		return nil, fmt.Errorf("unknown first node in discovery chain of type: %s", startNode.Type)
	}

	host := &envoyroute.VirtualHost{
		Name:    routeName,
		Domains: serviceDomains,
		Routes:  routes,
	}

	return host, nil
}

func makeRouteMatchForDiscoveryRoute(_ connectionInfo, discoveryRoute *structs.DiscoveryRoute) *envoyroute.RouteMatch {
	match := discoveryRoute.Definition.Match
	if match == nil || match.IsEmpty() {
		return makeDefaultRouteMatch()
	}

	em := &envoyroute.RouteMatch{}

	switch {
	case match.HTTP.PathExact != "":
		em.PathSpecifier = &envoyroute.RouteMatch_Path{
			Path: match.HTTP.PathExact,
		}
	case match.HTTP.PathPrefix != "":
		em.PathSpecifier = &envoyroute.RouteMatch_Prefix{
			Prefix: match.HTTP.PathPrefix,
		}
	case match.HTTP.PathRegex != "":
		em.PathSpecifier = &envoyroute.RouteMatch_SafeRegex{
			SafeRegex: makeEnvoyRegexMatch(match.HTTP.PathRegex),
		}
	default:
		em.PathSpecifier = &envoyroute.RouteMatch_Prefix{
			Prefix: "/",
		}
	}

	if len(match.HTTP.Header) > 0 {
		em.Headers = make([]*envoyroute.HeaderMatcher, 0, len(match.HTTP.Header))
		for _, hdr := range match.HTTP.Header {
			eh := &envoyroute.HeaderMatcher{
				Name: hdr.Name,
			}

			switch {
			case hdr.Exact != "":
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_ExactMatch{
					ExactMatch: hdr.Exact,
				}
			case hdr.Regex != "":
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SafeRegexMatch{
					SafeRegexMatch: makeEnvoyRegexMatch(hdr.Regex),
				}
			case hdr.Prefix != "":
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_PrefixMatch{
					PrefixMatch: hdr.Prefix,
				}
			case hdr.Suffix != "":
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SuffixMatch{
					SuffixMatch: hdr.Suffix,
				}
			case hdr.Present:
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_PresentMatch{
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

		eh := &envoyroute.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &envoyroute.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: makeEnvoyRegexMatch(methodHeaderRegex),
			},
		}

		em.Headers = append(em.Headers, eh)
	}

	if len(match.HTTP.QueryParam) > 0 {
		em.QueryParameters = make([]*envoyroute.QueryParameterMatcher, 0, len(match.HTTP.QueryParam))
		for _, qm := range match.HTTP.QueryParam {
			eq := &envoyroute.QueryParameterMatcher{
				Name: qm.Name,
			}

			switch {
			case qm.Exact != "":
				eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_StringMatch{
					StringMatch: &envoymatcher.StringMatcher{
						MatchPattern: &envoymatcher.StringMatcher_Exact{
							Exact: qm.Exact,
						},
					},
				}
			case qm.Regex != "":
				eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_StringMatch{
					StringMatch: &envoymatcher.StringMatcher{
						MatchPattern: &envoymatcher.StringMatcher_SafeRegex{
							SafeRegex: makeEnvoyRegexMatch(qm.Regex),
						},
					},
				}
			case qm.Present:
				eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_PresentMatch{
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

func makeDefaultRouteMatch() *envoyroute.RouteMatch {
	return &envoyroute.RouteMatch{
		PathSpecifier: &envoyroute.RouteMatch_Prefix{
			Prefix: "/",
		},
		// TODO(banks) Envoy supports matching only valid GRPC
		// requests which might be nice to add here for gRPC services
		// but it's not supported in our current envoy SDK version
		// although docs say it was supported by 1.8.0. Going to defer
		// that until we've updated the deps.
	}
}

func makeRouteActionForChainCluster(targetID string, chain *structs.CompiledDiscoveryChain) *envoyroute.Route_Route {
	target := chain.Targets[targetID]
	return makeRouteActionFromName(CustomizeClusterName(target.Name, chain))
}

func makeRouteActionFromName(clusterName string) *envoyroute.Route_Route {
	return &envoyroute.Route_Route{
		Route: &envoyroute.RouteAction{
			ClusterSpecifier: &envoyroute.RouteAction_Cluster{
				Cluster: clusterName,
			},
		},
	}
}

func makeRouteActionForSplitter(splits []*structs.DiscoverySplit, chain *structs.CompiledDiscoveryChain) (*envoyroute.Route_Route, error) {
	clusters := make([]*envoyroute.WeightedCluster_ClusterWeight, 0, len(splits))
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
		cw := &envoyroute.WeightedCluster_ClusterWeight{
			Weight: makeUint32Value(int(split.Weight * 100)),
			Name:   clusterName,
		}

		clusters = append(clusters, cw)
	}

	return &envoyroute.Route_Route{
		Route: &envoyroute.RouteAction{
			ClusterSpecifier: &envoyroute.RouteAction_WeightedClusters{
				WeightedClusters: &envoyroute.WeightedCluster{
					Clusters:    clusters,
					TotalWeight: makeUint32Value(10000), // scaled up 100%
				},
			},
		},
	}, nil
}

func injectLBToRouteAction(lb *structs.LoadBalancer, action *envoyroute.RouteAction) error {
	if lb == nil || !lb.IsHashBased() {
		return nil
	}

	result := make([]*envoyroute.RouteAction_HashPolicy, 0, len(lb.HashPolicies))
	for _, policy := range lb.HashPolicies {
		if policy.SourceIP {
			result = append(result, &envoyroute.RouteAction_HashPolicy{
				PolicySpecifier: &envoyroute.RouteAction_HashPolicy_ConnectionProperties_{
					ConnectionProperties: &envoyroute.RouteAction_HashPolicy_ConnectionProperties{
						SourceIp: true,
					},
				},
				Terminal: policy.Terminal,
			})

			continue
		}

		switch policy.Field {
		case structs.HashPolicyHeader:
			result = append(result, &envoyroute.RouteAction_HashPolicy{
				PolicySpecifier: &envoyroute.RouteAction_HashPolicy_Header_{
					Header: &envoyroute.RouteAction_HashPolicy_Header{
						HeaderName: policy.FieldValue,
					},
				},
				Terminal: policy.Terminal,
			})
		case structs.HashPolicyCookie:
			cookie := envoyroute.RouteAction_HashPolicy_Cookie{
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
			result = append(result, &envoyroute.RouteAction_HashPolicy{
				PolicySpecifier: &envoyroute.RouteAction_HashPolicy_Cookie_{
					Cookie: &cookie,
				},
				Terminal: policy.Terminal,
			})
		case structs.HashPolicyQueryParam:
			result = append(result, &envoyroute.RouteAction_HashPolicy{
				PolicySpecifier: &envoyroute.RouteAction_HashPolicy_QueryParameter_{
					QueryParameter: &envoyroute.RouteAction_HashPolicy_QueryParameter{
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
