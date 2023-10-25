// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"regexp"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func (b *Builder) buildExposePaths(workload *pbcatalog.Workload) {
	if b.proxyCfg.GetDynamicConfig() != nil && b.proxyCfg.GetDynamicConfig().GetExposeConfig() != nil {
		for _, exposePath := range b.proxyCfg.GetDynamicConfig().GetExposeConfig().GetExposePaths() {
			clusterName := exposePathClusterName(exposePath)

			b.addExposePathsListener(workload, exposePath).
				addExposePathsRouter(exposePath).
				buildListener()

			var protocol pbcatalog.Protocol
			switch exposePath.Protocol {
			case pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP:
				protocol = pbcatalog.Protocol_PROTOCOL_HTTP
			case pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP2:
				protocol = pbcatalog.Protocol_PROTOCOL_HTTP2
			default:
				panic("unsupported expose paths protocol")
			}

			b.addExposePathsRoute(exposePath, clusterName).
				addLocalAppCluster(clusterName, nil, pbproxystate.Protocol(protocol)).
				addLocalAppStaticEndpoints(clusterName, exposePath.LocalPathPort)
		}
	}
}

func (b *Builder) addExposePathsListener(workload *pbcatalog.Workload, exposePath *pbmesh.ExposePath) *ListenerBuilder {
	listenerName := exposePathListenerName(exposePath)

	listener := &pbproxystate.Listener{
		Name:      listenerName,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
	}

	meshAddress := workload.GetFirstNonExternalMeshAddress()
	if meshAddress == nil {
		return b.NewListenerBuilder(nil)
	}

	listener.BindAddress = &pbproxystate.Listener_HostPort{
		HostPort: &pbproxystate.HostPortAddress{
			Host: meshAddress.Host,
			Port: exposePath.ListenerPort,
		},
	}

	return b.NewListenerBuilder(listener)
}

func (b *ListenerBuilder) addExposePathsRouter(exposePath *pbmesh.ExposePath) *ListenerBuilder {
	if b.listener == nil {
		return b
	}
	destinationName := exposePathRouteName(exposePath)

	var l7Protocol pbproxystate.L7Protocol

	switch exposePath.Protocol {
	case pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP:
		l7Protocol = pbproxystate.L7Protocol_L7_PROTOCOL_HTTP
	case pbmesh.ExposePathProtocol_EXPOSE_PATH_PROTOCOL_HTTP2:
		l7Protocol = pbproxystate.L7Protocol_L7_PROTOCOL_HTTP2
	default:
		panic("unsupported expose paths protocol")
	}
	routerDestination := &pbproxystate.Router_L7{
		L7: &pbproxystate.L7Destination{
			Route: &pbproxystate.L7DestinationRoute{
				Name: destinationName,
			},
			StatPrefix:  destinationName,
			StaticRoute: true,
			Protocol:    l7Protocol,
		},
	}

	router := &pbproxystate.Router{
		Destination: routerDestination,
	}

	b.listener.Routers = append(b.listener.Routers, router)

	return b
}

func (b *Builder) addExposePathsRoute(exposePath *pbmesh.ExposePath, clusterName string) *Builder {
	routeName := exposePathRouteName(exposePath)
	routeRule := &pbproxystate.RouteRule{
		Match: &pbproxystate.RouteMatch{
			PathMatch: &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Exact{
					Exact: exposePath.Path,
				},
			},
		},
		Destination: &pbproxystate.RouteDestination{
			Destination: &pbproxystate.RouteDestination_Cluster{
				Cluster: &pbproxystate.DestinationCluster{
					Name: clusterName,
				},
			},
		},
	}
	virtualHost := &pbproxystate.VirtualHost{
		Name:       routeName,
		Domains:    []string{"*"},
		RouteRules: []*pbproxystate.RouteRule{routeRule},
	}
	route := &pbproxystate.Route{
		VirtualHosts: []*pbproxystate.VirtualHost{virtualHost},
	}

	b.proxyStateTemplate.ProxyState.Routes[routeName] = route
	return b
}

func exposePathName(exposePath *pbmesh.ExposePath) string {
	r := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	// The regex removes anything not a letter or number from the path.
	path := r.ReplaceAllString(exposePath.Path, "")
	return path
}

func exposePathListenerName(exposePath *pbmesh.ExposePath) string {
	// The path could be empty, so the unique name for this exposed path is the path and listener port.
	pathPort := fmt.Sprintf("%s%d", exposePathName(exposePath), exposePath.ListenerPort)
	listenerName := fmt.Sprintf("exposed_path_%s", pathPort)
	return listenerName
}

func exposePathRouteName(exposePath *pbmesh.ExposePath) string {
	// The path could be empty, so the unique name for this exposed path is the path and listener port.
	pathPort := fmt.Sprintf("%s%d", exposePathName(exposePath), exposePath.ListenerPort)
	return fmt.Sprintf("exposed_path_route_%s", pathPort)
}

func exposePathClusterName(exposePath *pbmesh.ExposePath) string {
	return fmt.Sprintf("exposed_cluster_%d", exposePath.LocalPathPort)
}
