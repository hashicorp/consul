package extensioncommon

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"

	"strings"
)

// GetListenerEnvoyID returns the Envoy ID string parsed from the name of the given Listener. If none is found, it
// returns the empty string.
func GetListenerEnvoyID(l *envoy_listener_v3.Listener) string {
	if id, _, found := strings.Cut(l.Name, ":"); found {
		return id
	}
	return ""
}

// IsLocalAppCluster returns true if the given Cluster represents the local Cluster, which receives inbound traffic to
// the local proxy.
func IsLocalAppCluster(c *envoy_cluster_v3.Cluster) bool {
	return c.Name == xdscommon.LocalAppClusterName
}

// IsRouteToLocalAppCluster takes a RouteConfiguration and returns true if all routes within it target the local
// Cluster. Note that because we currently target RouteConfiguration in PatchRoute, we have to check multiple individual
// Route resources.
func IsRouteToLocalAppCluster(r *envoy_route_v3.RouteConfiguration) bool {
	clusterNames := RouteClusterNames(r)
	_, match := clusterNames[xdscommon.LocalAppClusterName]

	return match && len(clusterNames) == 1
}

// IsInboundPublicListener returns true if the given Listener represents the inbound public Listener for the local
// service.
func IsInboundPublicListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.PublicListenerName
}

// IsOutboundTProxyListener returns true if the given Listener represents the outbound TProxy Listener for the local
// service.
func IsOutboundTProxyListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.OutboundListenerName
}
