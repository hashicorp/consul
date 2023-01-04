package validate

import (
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

type validate struct {
}

var _ builtinextensiontemplate.Plugin = (*validate)(nil)

// MakeValidate is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeValidate(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin validate

	return plugin, resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p validate) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return true
}

// PatchRoute does nothing.
func (p validate) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return route, false, nil
}

// PatchCluster does nothing.
func (p validate) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, fmt.Errorf("bad cluster")
}

// PatchFilter inserts a lua filter directly prior to BeforeFilterWithName.
func (p validate) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	return filter, true, nil
}
