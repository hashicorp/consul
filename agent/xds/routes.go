package xds

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func routesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, _ string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return routesFromSnapshotConnectProxy(cfgSnap)
	case structs.ServiceKindIngressGateway:
		return routesFromSnapshotIngressGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func routesFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	return routesFromUpstreams(cfgSnap.ConnectProxy.ConfigSnapshotUpstreams, cfgSnap.Proxy.Upstreams)
}

// routesFromSnapshotIngressGateway returns the xDS API representation of the
// "routes" in the snapshot.
func routesFromSnapshotIngressGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	return routesFromUpstreams(cfgSnap.IngressGateway.ConfigSnapshotUpstreams, cfgSnap.IngressGateway.Upstreams)
}

func routesFromUpstreams(snap proxycfg.ConfigSnapshotUpstreams, upstreams structs.Upstreams) ([]proto.Message, error) {
	var resources []proto.Message

	for _, u := range upstreams {
		upstreamID := u.Identifier()

		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = snap.DiscoveryChain[upstreamID]
		}

		if chain == nil || chain.IsDefault() {
			// TODO(rb): make this do the old school stuff too
		} else {
			upstreamRoute, err := makeUpstreamRouteForDiscoveryChain(&u, chain)
			if err != nil {
				return nil, err
			}
			if upstreamRoute != nil {
				resources = append(resources, upstreamRoute)
			}
		}
	}

	// TODO(rb): make sure we don't generate an empty result

	return resources, nil
}

func makeUpstreamRouteForDiscoveryChain(
	u *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
) (*envoy.RouteConfiguration, error) {
	upstreamID := u.Identifier()
	routeName := upstreamID

	var routes []envoyroute.Route

	startNode := chain.Nodes[chain.StartNode]
	if startNode == nil {
		panic("missing first node in compiled discovery chain for: " + chain.ServiceName)
	}

	switch startNode.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		routes = make([]envoyroute.Route, 0, len(startNode.Routes))

		for _, discoveryRoute := range startNode.Routes {
			routeMatch := makeRouteMatchForDiscoveryRoute(discoveryRoute, chain.Protocol)

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
					routeAction.Route.Timeout = &destination.RequestTimeout
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

			routes = append(routes, envoyroute.Route{
				Match:  routeMatch,
				Action: routeAction,
			})
		}

	case structs.DiscoveryGraphNodeTypeSplitter:
		routeAction, err := makeRouteActionForSplitter(startNode.Splits, chain)
		if err != nil {
			return nil, err
		}

		defaultRoute := envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []envoyroute.Route{defaultRoute}

	case structs.DiscoveryGraphNodeTypeResolver:
		routeAction := makeRouteActionForSingleCluster(startNode.Resolver.Target, chain)

		defaultRoute := envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []envoyroute.Route{defaultRoute}

	default:
		panic("unknown first node in discovery chain of type: " + startNode.Type)
	}

	return &envoy.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []envoyroute.VirtualHost{
			envoyroute.VirtualHost{
				Name:    routeName,
				Domains: []string{"*"},
				Routes:  routes,
			},
		},
		// ValidateClusters defaults to true when defined statically and false
		// when done via RDS. Re-set the sane value of true to prevent
		// null-routing traffic.
		ValidateClusters: makeBoolValue(true),
	}, nil
}

func makeRouteMatchForDiscoveryRoute(discoveryRoute *structs.DiscoveryRoute, protocol string) envoyroute.RouteMatch {
	match := discoveryRoute.Definition.Match
	if match == nil || match.IsEmpty() {
		return makeDefaultRouteMatch()
	}

	em := envoyroute.RouteMatch{}

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
		em.PathSpecifier = &envoyroute.RouteMatch_Regex{
			Regex: match.HTTP.PathRegex,
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
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_RegexMatch{
					RegexMatch: hdr.Regex,
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
			HeaderMatchSpecifier: &envoyroute.HeaderMatcher_RegexMatch{
				RegexMatch: methodHeaderRegex,
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
				eq.Value = qm.Exact
			case qm.Regex != "":
				eq.Value = qm.Regex
				eq.Regex = makeBoolValue(true)
			case qm.Present:
				eq.Value = ""
			default:
				continue // skip this impossible situation
			}

			em.QueryParameters = append(em.QueryParameters, eq)
		}
	}

	return em
}

func makeDefaultRouteMatch() envoyroute.RouteMatch {
	return envoyroute.RouteMatch{
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
