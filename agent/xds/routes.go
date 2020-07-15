package xds

import (
	"errors"
	"fmt"
	"net"
	"strings"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoymatcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func routesFromSnapshot(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return routesFromSnapshotConnectProxy(cInfo, cfgSnap)
	case structs.ServiceKindIngressGateway:
		return routesFromSnapshotIngressGateway(cInfo, cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func routesFromSnapshotConnectProxy(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	var resources []proto.Message
	for _, u := range cfgSnap.Proxy.Upstreams {
		upstreamID := u.Identifier()

		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = cfgSnap.ConnectProxy.DiscoveryChain[upstreamID]
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

// routesFromSnapshotIngressGateway returns the xDS API representation of the
// "routes" in the snapshot.
func routesFromSnapshotIngressGateway(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	var result []proto.Message
	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
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
			chain := cfgSnap.IngressGateway.DiscoveryChain[upstreamID]
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
		panic("missing first node in compiled discovery chain for: " + chain.ServiceName)
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
			switch nextNode.Type {
			case structs.DiscoveryGraphNodeTypeSplitter:
				routeAction, err = makeRouteActionForSplitter(nextNode.Splits, chain)
				if err != nil {
					return nil, err
				}

			case structs.DiscoveryGraphNodeTypeResolver:
				routeAction = makeRouteActionForSingleCluster(nextNode.Resolver.Target, chain)

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

		defaultRoute := &envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoyroute.Route{defaultRoute}

	case structs.DiscoveryGraphNodeTypeResolver:
		routeAction := makeRouteActionForSingleCluster(startNode.Resolver.Target, chain)

		defaultRoute := &envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []*envoyroute.Route{defaultRoute}

	default:
		panic("unknown first node in discovery chain of type: " + startNode.Type)
	}

	host := &envoyroute.VirtualHost{
		Name:    routeName,
		Domains: serviceDomains,
		Routes:  routes,
	}

	return host, nil
}

func makeRouteMatchForDiscoveryRoute(cInfo connectionInfo, discoveryRoute *structs.DiscoveryRoute) *envoyroute.RouteMatch {
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
		if cInfo.ProxyFeatures.RouterMatchSafeRegex {
			em.PathSpecifier = &envoyroute.RouteMatch_SafeRegex{
				SafeRegex: makeEnvoyRegexMatch(match.HTTP.PathRegex),
			}
		} else {
			em.PathSpecifier = &envoyroute.RouteMatch_Regex{
				Regex: match.HTTP.PathRegex,
			}
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
				if cInfo.ProxyFeatures.RouterMatchSafeRegex {
					eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SafeRegexMatch{
						SafeRegexMatch: makeEnvoyRegexMatch(hdr.Regex),
					}
				} else {
					eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_RegexMatch{
						RegexMatch: hdr.Regex,
					}
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
		}
		if cInfo.ProxyFeatures.RouterMatchSafeRegex {
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: makeEnvoyRegexMatch(methodHeaderRegex),
			}
		} else {
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_RegexMatch{
				RegexMatch: methodHeaderRegex,
			}
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
				if cInfo.ProxyFeatures.RouterMatchSafeRegex {
					eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_StringMatch{
						StringMatch: &envoymatcher.StringMatcher{
							MatchPattern: &envoymatcher.StringMatcher_Exact{
								Exact: qm.Exact,
							},
						},
					}
				} else {
					eq.Value = qm.Exact
				}
			case qm.Regex != "":
				if cInfo.ProxyFeatures.RouterMatchSafeRegex {
					eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_StringMatch{
						StringMatch: &envoymatcher.StringMatcher{
							MatchPattern: &envoymatcher.StringMatcher_SafeRegex{
								SafeRegex: makeEnvoyRegexMatch(qm.Regex),
							},
						},
					}
				} else {
					eq.Value = qm.Regex
					eq.Regex = makeBoolValue(true)
				}
			case qm.Present:
				if cInfo.ProxyFeatures.RouterMatchSafeRegex {
					eq.QueryParameterMatchSpecifier = &envoyroute.QueryParameterMatcher_PresentMatch{
						PresentMatch: true,
					}
				} else {
					eq.Value = ""
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

func makeRouteActionForSingleCluster(targetID string, chain *structs.CompiledDiscoveryChain) *envoyroute.Route_Route {
	target := chain.Targets[targetID]

	clusterName := CustomizeClusterName(target.Name, chain)

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

func makeEnvoyRegexMatch(patt string) *envoymatcher.RegexMatcher {
	return &envoymatcher.RegexMatcher{
		EngineType: &envoymatcher.RegexMatcher_GoogleRe2{
			GoogleRe2: &envoymatcher.RegexMatcher_GoogleRE2{},
		},
		Regex: patt,
	}
}
