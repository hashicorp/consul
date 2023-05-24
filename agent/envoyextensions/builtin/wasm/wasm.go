// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wasm

import (
	"errors"
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// wasm is a built-in Envoy extension that can patch filter chains to insert Wasm plugins.
type wasm struct {
	name       string
	wasmConfig *wasmConfig
}

var supportedRuntimes = []string{"v8", "wamr", "wavm", "wasmtime"}

var _ extensioncommon.BasicExtension = (*wasm)(nil)

func Constructor(ext api.EnvoyExtension) (extensioncommon.EnvoyExtender, error) {
	w, err := construct(ext)
	if err != nil {
		return nil, err
	}
	return &extensioncommon.BasicEnvoyExtender{
		Extension: &w,
	}, nil
}

func construct(ext api.EnvoyExtension) (wasm, error) {
	w := wasm{name: ext.Name}

	if w.name != api.BuiltinWasmExtension {
		return w, fmt.Errorf("expected extension name %q but got %q", api.BuiltinWasmExtension, w.name)
	}

	if err := w.fromArguments(ext.Arguments); err != nil {
		return w, err
	}

	// Configure the failure behavior for the filter. If the plugin is required,
	// then filter runtime errors result in a failed request (fail "closed").
	// Otherwise, runtime errors result in the filter being skipped (fail "open").
	w.wasmConfig.PluginConfig.failOpen = !ext.Required

	return w, nil
}

func (w *wasm) fromArguments(args map[string]any) error {
	var err error
	w.wasmConfig, err = newWasmConfig(args)
	if err != nil {
		return fmt.Errorf("error decoding extension arguments: %w", err)
	}
	return w.wasmConfig.validate()
}

// CanApply indicates if the WASM extension can be applied to the given extension configuration.
// Currently the Wasm extension can be applied if the extension configuration is for an inbound
// listener (checked below) on a local connect-proxy.
func (w wasm) CanApply(config *extensioncommon.RuntimeConfig) bool {
	return config.Kind == w.wasmConfig.ProxyType
}

func (w wasm) matchesConfigDirection(isInboundListener bool) bool {
	return isInboundListener && w.wasmConfig.ListenerType == "inbound"
}

// PatchRoute does nothing for the WASM extension.
func (w wasm) PatchRoute(_ *extensioncommon.RuntimeConfig, r *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return r, false, nil
}

// PatchCluster does nothing for the WASM extension.
func (w wasm) PatchCluster(_ *extensioncommon.RuntimeConfig, c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchFilter adds a Wasm filter to the HTTP filter chain.
// TODO (wasm/tcp): Add support for TCP filters.
func (w wasm) PatchFilter(cfg *extensioncommon.RuntimeConfig, filter *envoy_listener_v3.Filter, isInboundListener bool) (*envoy_listener_v3.Filter, bool, error) {
	if !w.matchesConfigDirection(isInboundListener) {
		return filter, false, nil
	}

	if filter.Name != "envoy.filters.network.http_connection_manager" {
		return filter, false, nil
	}
	if typedConfig := filter.GetTypedConfig(); typedConfig == nil {
		return filter, false, errors.New("failed to get typed config for http filter")
	}

	httpConnMgr := envoy_resource_v3.GetHTTPConnectionManager(filter)
	if httpConnMgr == nil {
		return filter, false, errors.New("failed to get HTTP connection manager")
	}

	wasmPluginConfig, err := w.wasmConfig.PluginConfig.envoyPluginConfig(cfg)
	if err != nil {
		return filter, false, fmt.Errorf("failed to encode Envoy Wasm configuration: %w", err)
	}

	extHttpFilter, err := extensioncommon.MakeEnvoyHTTPFilter(
		"envoy.filters.http.wasm",
		&envoy_http_wasm_v3.Wasm{Config: wasmPluginConfig},
	)
	if err != nil {
		return filter, false, err
	}

	var (
		changedFilters = make([]*envoy_http_v3.HttpFilter, 0, len(httpConnMgr.HttpFilters)+1)
		changed        bool
	)

	// We need to be careful about overwriting http filters completely because
	// http filters validates intentions with the RBAC filter. This inserts the
	// filter before `envoy.filters.http.router` while keeping everything
	// else intact.
	for _, httpFilter := range httpConnMgr.HttpFilters {
		if httpFilter.Name == "envoy.filters.http.router" {
			changedFilters = append(changedFilters, extHttpFilter)
			changed = true
		}
		changedFilters = append(changedFilters, httpFilter)
	}
	if changed {
		httpConnMgr.HttpFilters = changedFilters
	}

	newFilter, err := extensioncommon.MakeFilter("envoy.filters.network.http_connection_manager", httpConnMgr)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}
