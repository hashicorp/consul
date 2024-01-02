// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lua

import (
	"errors"
	"fmt"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
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
	extensioncommon.BasicExtensionAdapter

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
	if l.ProxyType == "" {
		l.ProxyType = string(api.ServiceKindConnectProxy)
	}
	return l.validate()
}

func (l *lua) validate() error {
	var resultErr error
	if l.Script == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("missing Script value"))
	}
	if l.ProxyType != string(api.ServiceKindConnectProxy) {
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

func (l *lua) matchesListenerDirection(p extensioncommon.FilterPayload) bool {
	isInboundListener := p.IsInbound()
	return (!isInboundListener && l.Listener == "outbound") || (isInboundListener && l.Listener == "inbound")
}

// PatchFilter inserts a lua filter directly prior to envoy.filters.http.router.
func (l *lua) PatchFilter(p extensioncommon.FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
	filter := p.Message
	// Make sure filter matches extension config.
	if !l.matchesListenerDirection(p) {
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
	luaHttpFilter, err := extensioncommon.MakeEnvoyHTTPFilter(
		"envoy.filters.http.lua",
		&envoy_lua_v3.Lua{
			DefaultSourceCode: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: l.Script,
				},
			},
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

	newFilter, err := extensioncommon.MakeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}
