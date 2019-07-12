package xds

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// routesFromSnapshot returns the xDS API representation of the "routes" in the
// snapshot.
func routesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return routesFromSnapshotConnectProxy(cfgSnap, token)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// routesFromSnapshotConnectProxy returns the xDS API representation of the
// "routes" in the snapshot.
func routesFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
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
			upstreamRoute, err := makeUpstreamRouteForDiscoveryChain(&u, chain, cfgSnap)
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
	cfgSnap *proxycfg.ConfigSnapshot,
) (*envoy.RouteConfiguration, error) {
	upstreamID := u.Identifier()
	routeName := upstreamID

	var routes []envoyroute.Route

	switch chain.Node.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		routes = make([]envoyroute.Route, 0, len(chain.Node.Routes))

		for _, discoveryRoute := range chain.Node.Routes {
			routeMatch := makeRouteMatchForDiscoveryRoute(discoveryRoute, chain.Protocol)

			var (
				routeAction *envoyroute.Route_Route
				err         error
			)

			next := discoveryRoute.DestinationNode
			if next.Type == structs.DiscoveryGraphNodeTypeSplitter {
				routeAction, err = makeRouteActionForSplitter(next.Splits, cfgSnap)
				if err != nil {
					return nil, err
				}

			} else if next.Type == structs.DiscoveryGraphNodeTypeGroupResolver {
				groupResolver := next.GroupResolver
				routeAction = makeRouteActionForSingleCluster(groupResolver.Target, cfgSnap)

			} else {
				return nil, fmt.Errorf("unexpected graph node after route %q", next.Type)
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
							retryPolicy.RetryOn = ",retriable-status-codes"
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
		routeAction, err := makeRouteActionForSplitter(chain.Node.Splits, cfgSnap)
		if err != nil {
			return nil, err
		}

		defaultRoute := envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []envoyroute.Route{defaultRoute}

	case structs.DiscoveryGraphNodeTypeGroupResolver:
		groupResolver := chain.Node.GroupResolver

		routeAction := makeRouteActionForSingleCluster(groupResolver.Target, cfgSnap)

		defaultRoute := envoyroute.Route{
			Match:  makeDefaultRouteMatch(),
			Action: routeAction,
		}

		routes = []envoyroute.Route{defaultRoute}

	default:
		panic("unknown top node in discovery chain of type: " + chain.Node.Type)
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
	switch protocol {
	case "http", "http2":
		// The only match stanza is HTTP.
	default:
		return makeDefaultRouteMatch()
	}

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
			case hdr.Invert: // THIS HAS TO BE LAST
				eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_PresentMatch{
					// We set this to the misleading value of 'true' here
					// because we'll generically invert it next.
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

	if len(match.HTTP.QueryParam) > 0 {
		em.QueryParameters = make([]*envoyroute.QueryParameterMatcher, 0, len(match.HTTP.QueryParam))
		for _, qm := range match.HTTP.QueryParam {
			eq := &envoyroute.QueryParameterMatcher{
				Name:  qm.Name,
				Value: qm.Value,
				Regex: makeBoolValue(qm.Regex),
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

func makeRouteActionForSingleCluster(target structs.DiscoveryTarget, cfgSnap *proxycfg.ConfigSnapshot) *envoyroute.Route_Route {
	clusterName := TargetSNI(target, cfgSnap)

	return &envoyroute.Route_Route{
		Route: &envoyroute.RouteAction{
			ClusterSpecifier: &envoyroute.RouteAction_Cluster{
				Cluster: clusterName,
			},
		},
	}
}

func makeRouteActionForSplitter(splits []*structs.DiscoverySplit, cfgSnap *proxycfg.ConfigSnapshot) (*envoyroute.Route_Route, error) {
	clusters := make([]*envoyroute.WeightedCluster_ClusterWeight, 0, len(splits))
	for _, split := range splits {
		if split.Node.Type != structs.DiscoveryGraphNodeTypeGroupResolver {
			return nil, fmt.Errorf("unexpected splitter destination node type: %s", split.Node.Type)
		}
		groupResolver := split.Node.GroupResolver
		target := groupResolver.Target
		clusterName := TargetSNI(target, cfgSnap)

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
