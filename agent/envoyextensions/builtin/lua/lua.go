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
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
)

var _ extensioncommon.BasicExtension = (*lua)(nil)

type lua struct {
	ProxyType string
	Listener  string
	Script    string
}

// Constructor follows a specific function signature required for the extension registration.
func Constructor(ext api.EnvoyExtension) (extensioncommon.EnvoyExtender, error) {
	var l lua
	if name := ext.Name; name != api.BuiltinLuaExtension {
		return nil, fmt.Errorf("expected extension name 'lua' but got %q", name)
	}
	if err := l.fromArguments(ext.Arguments); err != nil {
		return nil, err
	}
	return &extensioncommon.BasicEnvoyExtender{
		Extension: &l,
	}, nil
}

func (l *lua) fromArguments(args map[string]interface{}) error {
	if err := mapstructure.Decode(args, l); err != nil {
		return fmt.Errorf("error decoding extension arguments: %v", err)
	}
	return l.validate()
}

func (l *lua) validate() error {
	var resultErr error
	if l.Script == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("missing Script value"))
	}
	if l.ProxyType != "connect-proxy" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unexpected ProxyType %q", l.ProxyType))
	}
	if l.Listener != "inbound" && l.Listener != "outbound" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unexpected Listener %q", l.Listener))
	}
	return resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (l *lua) CanApply(config *extensioncommon.RuntimeConfig) bool {
	return string(config.Kind) == l.ProxyType
}

func (l *lua) matchesListenerDirection(isInboundListener bool) bool {
	return (!isInboundListener && l.Listener == "outbound") || (isInboundListener && l.Listener == "inbound")
}

// PatchRoute does nothing.
func (l *lua) PatchRoute(_ *extensioncommon.RuntimeConfig, route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return route, false, nil
}

// PatchCluster does nothing.
func (l *lua) PatchCluster(_ *extensioncommon.RuntimeConfig, c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchFilter inserts a lua filter directly prior to envoy.filters.http.router.
func (l *lua) PatchFilter(_ *extensioncommon.RuntimeConfig, filter *envoy_listener_v3.Filter, isInboundListener bool) (*envoy_listener_v3.Filter, bool, error) {
	// Make sure filter matches extension config.
	if !l.matchesListenerDirection(isInboundListener) {
		return filter, false, nil
	}

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
			InlineCode: l.Script,
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
