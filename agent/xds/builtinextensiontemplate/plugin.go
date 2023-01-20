package builtinextensiontemplate

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

// Plugin is the interface that each extension must implement. It
// is responsible for modifying the xDS structures based on only the state of
// the extension.
type Plugin interface {
	// CanApply determines if the extension can mutate resources for the given xdscommon.ExtensionConfiguration.
	CanApply(xdscommon.ExtensionConfiguration) bool

	// PatchRoute patches a route to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	PatchRoute(*envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error)

	// PatchCluster patches a cluster to include the custom Envoy configuration
	// required to integrate with the built in extension template.
	PatchCluster(*envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error)

	// PatchFilter patches an Envoy filter to include the custom Envoy
	// configuration required to integrate with the built in extension template.
	PatchFilter(*envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error)
}

// PluginConstructor is used to construct a plugin based on
// xdscommon.ExtensionConfiguration. This function should contain all the logic around
// turning an extension's arguments into a plugin. The PluginConstructor will be used
// as the Constructor field on an EnvoyExtension.
type PluginConstructor func(extension xdscommon.ExtensionConfiguration) (Plugin, error)
