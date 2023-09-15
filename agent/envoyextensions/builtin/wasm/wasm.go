// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package wasm

import (
	"fmt"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	envoy_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/wasm/v3"

	"github.com/hashicorp/consul/api"
	cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// wasm is a built-in Envoy extension that can patch filter chains to insert Wasm plugins.
type wasm struct {
	cmn.BasicExtensionAdapter

	name       string
	wasmConfig *wasmConfig
}

var supportedRuntimes = []string{"v8", "wamr", "wavm", "wasmtime"}

var _ cmn.BasicExtension = (*wasm)(nil)

func Constructor(ext api.EnvoyExtension) (cmn.EnvoyExtender, error) {
	w, err := construct(ext)
	if err != nil {
		return nil, err
	}
	return &cmn.BasicEnvoyExtender{
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
func (w wasm) CanApply(cfg *cmn.RuntimeConfig) bool {
	return cfg.Kind == w.wasmConfig.ProxyType &&
		cfg.Protocol == w.wasmConfig.Protocol
}

func (w wasm) matchesConfigDirection(isInboundListener bool) bool {
	return (isInboundListener && w.wasmConfig.ListenerType == "inbound") ||
		(!isInboundListener && w.wasmConfig.ListenerType == "outbound")
}

// PatchFilters adds a Wasm HTTP or TCP filter to the filter chain.
func (w wasm) PatchFilters(cfg *cmn.RuntimeConfig, filters []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error) {
	if !w.matchesConfigDirection(isInboundListener) {
		return filters, nil
	}

	// Check that the Wasm plugin protocol matches the service protocol.
	// It is a runtime error if the extension is configured to apply a Wasm plugin
	// that doesn't match the service's protocol.
	// This shouldn't happen because the caller should check CanApply first, but just in case.
	if cfg.Protocol != w.wasmConfig.Protocol {
		return filters, fmt.Errorf("failed to apply Wasm filter: service protocol for %q is %q but protocol for the Wasm extension is %q. Please ensure the protocols match",
			cfg.ServiceName.Name, cfg.Protocol, w.wasmConfig.Protocol)
	}

	// Generate the Wasm plugin configuration. It is the same config for HTTP and network filters.
	wasmPluginConfig, err := w.wasmConfig.PluginConfig.envoyPluginConfig(cfg)
	if err != nil {
		return filters, fmt.Errorf("failed to encode Envoy Wasm plugin configuration: %w", err)
	}

	// Insert the filter immediately before the terminal filter.
	insertOptions := cmn.InsertOptions{Location: cmn.InsertBeforeFirstMatch}

	switch cfg.Protocol {
	case "grpc", "http2", "http":
		insertOptions.FilterName = "envoy.filters.http.router"
		filter, err := cmn.MakeEnvoyHTTPFilter("envoy.filters.http.wasm", &envoy_http_wasm_v3.Wasm{Config: wasmPluginConfig})
		if err != nil {
			return filters, fmt.Errorf("failed to make Wasm HTTP filter: %w", err)
		}
		return cmn.InsertHTTPFilter(filters, filter, insertOptions)
	case "tcp":
		fallthrough
	default:
		insertOptions.FilterName = "envoy.filters.network.tcp_proxy"
		filter, err := cmn.MakeFilter("envoy.filters.network.wasm", &envoy_wasm_v3.Wasm{Config: wasmPluginConfig})
		if err != nil {
			return filters, fmt.Errorf("failed to make Wasm network filter: %w", err)
		}
		return cmn.InsertNetworkFilter(filters, filter, insertOptions)
	}
}
