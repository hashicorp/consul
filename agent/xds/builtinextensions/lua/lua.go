package lua

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_lua_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

type lua struct {
	ProxyType string
	Listener  string
	Script    string
}

var _ builtinextensiontemplate.Plugin = (*lua)(nil)

// MakeLuaExtension is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeLuaExtension(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin lua

	if name := ext.EnvoyExtension.Name; name != api.BuiltinLuaExtension {
		return nil, fmt.Errorf("expected extension name 'lua' but got %q", name)
	}

	if err := mapstructure.Decode(ext.EnvoyExtension.Arguments, &plugin); err != nil {
		return nil, fmt.Errorf("error decoding extension arguments: %v", err)
	}

	if plugin.Script == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("Script is required"))
	}

	if err := validateProxyType(plugin.ProxyType); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	if err := validateListener(plugin.Listener); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return plugin, resultErr
}

func validateProxyType(t string) error {
	if t != "connect-proxy" {
		return fmt.Errorf("unexpected ProxyType %q", t)
	}

	return nil
}

func validateListener(t string) error {
	if t != "inbound" && t != "outbound" {
		return fmt.Errorf("unexpected Listener %q", t)
	}

	return nil
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p lua) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return string(config.Kind) == p.ProxyType && p.matchesListenerDirection(config)
}

func (p lua) matchesListenerDirection(config xdscommon.ExtensionConfiguration) bool {
	return (config.IsUpstream() && p.Listener == "outbound") || (!config.IsUpstream() && p.Listener == "inbound")
}

// PatchRoute does nothing.
func (p lua) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return route, false, nil
}

// PatchCluster does nothing.
func (p lua) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchFilter inserts a lua filter directly prior to envoy.filters.http.router.
func (p lua) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	if filter.Name != "envoy.filters.network.http_connection_manager" {
		return filter, false, nil
	}
	if typedConfig := filter.GetTypedConfig(); typedConfig == nil {
		return filter, false, errors.New("error getting typed config for http filter")
	}

	config := envoy_resource_v3.GetHTTPConnectionManager(filter)
	if config == nil {
		return filter, false, errors.New("error unmarshalling filter")
	}
	luaHttpFilter, err := makeEnvoyHTTPFilter(
		"envoy.filters.http.lua",
		&envoy_lua_v3.Lua{
			InlineCode: p.Script,
		},
	)
	if err != nil {
		return filter, false, err
	}

	var (
		changedFilters = make([]*envoy_http_v3.HttpFilter, 0, len(config.HttpFilters)+1)
		changed        bool
	)

	// We need to be careful about overwriting http filters completely because
	// http filters validates intentions with the RBAC filter. This inserts the
	// lua filter before envoy.filters.http.router while keeping everything
	// else intact.
	for _, httpFilter := range config.HttpFilters {
		if httpFilter.Name == "envoy.filters.http.router" {
			changedFilters = append(changedFilters, luaHttpFilter)
			changed = true
		}
		changedFilters = append(changedFilters, httpFilter)
	}
	if changed {
		config.HttpFilters = changedFilters
	}

	newFilter, err := makeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}
