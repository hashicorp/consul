// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strings"

	"github.com/hashicorp/consul/api"
)

// EnvoyExtension has configuration for an extension that patches Envoy resources.
type EnvoyExtension struct {
	Name          string
	Required      bool
	Arguments     map[string]interface{} `bexpr:"-"`
	ConsulVersion string
	EnvoyVersion  string
}

func (c *EnvoyExtension) getHash() uint64 {
	return hashValue(c)
}

func (c *EnvoyExtension) appendHash(h *customHasher) {
	h.addString(c.Name).
		addBool(c.Required).
		addString(c.ConsulVersion).
		addString(c.EnvoyVersion).
		addJSONValue(c.Arguments)
}

type EnvoyExtensions []EnvoyExtension

// codeExecutingExtensions is the set of built-in extension names that cause
// Envoy to compile and execute caller-supplied code (Lua scripts, Wasm modules)
// on every proxied request. Attaching any of these to a config entry requires
// mesh:write in addition to the normal service:write check so that the
// code-execution capability is gated on a separate, auditable permission.
var codeExecutingExtensions = map[string]bool{
	api.BuiltinLuaExtension:  true,
	api.BuiltinWasmExtension: true,
}

// HasCodeExecutingExtension reports whether any extension in the slice is a
// code-executing type (builtin/lua or builtin/wasm).
func (es EnvoyExtensions) HasCodeExecutingExtension() bool {
	for _, e := range es {
		if codeExecutingExtensions[e.Name] {
			return true
		}
	}
	return false
}

// envoyEscapeHatchKeys is the set of Proxy.Config map keys that embed
// arbitrary Envoy JSON into the sidecar's bootstrap configuration or xDS
// listener/cluster resources. Any of these keys can introduce code-executing
// HTTP filters and therefore require mesh:write in addition to service:write.
//
// It is unexported and accessed only through IsEnvoyEscapeHatchKey,
// HasEnvoyEscapeHatchKey, and EnvoyEscapeHatchKeyNames so that no importing
// package (or test) can mutate the set at runtime and silently weaken the
// security gate.
//
// Bootstrap-time keys (command/connect/envoy/bootstrap_config.go):
//
//	envoy_bootstrap_json_tpl, envoy_extra_static_listeners_json,
//	envoy_extra_static_clusters_json, envoy_extra_stats_sinks_json,
//	envoy_stats_config_json, envoy_tracing_json
//
// xDS runtime keys (agent/xds/config/config.go):
//
//	envoy_public_listener_json, envoy_listener_tracing_json,
//	envoy_local_cluster_json
//
// Per-upstream keys (agent/structs/config_entry.go UpstreamConfig):
//
//	envoy_listener_json, envoy_cluster_json
var envoyEscapeHatchKeys = map[string]bool{
	"envoy_bootstrap_json_tpl":          true,
	"envoy_extra_static_listeners_json": true,
	"envoy_public_listener_json":        true,
	"envoy_listener_json":               true,
	"envoy_cluster_json":                true,
	"envoy_local_cluster_json":          true,
	"envoy_extra_static_clusters_json":  true,
	"envoy_extra_stats_sinks_json":      true,
	"envoy_tracing_json":                true,
	"envoy_stats_config_json":           true,
	"envoy_listener_tracing_json":       true,
}

// IsEnvoyEscapeHatchKey reports whether the given config map key is an Envoy
// escape-hatch key. Comparison is case-insensitive: the mapstructure decoders
// that ultimately consume these maps (command/connect/envoy/bootstrap_config.go,
// agent/xds/config/config.go) match struct tags against map keys
// case-insensitively by default, so an exact-case comparison here could be
// bypassed by a caller-supplied key that only differs in case.
func IsEnvoyEscapeHatchKey(key string) bool {
	return envoyEscapeHatchKeys[strings.ToLower(key)]
}

// EnvoyEscapeHatchKeyNames returns a copy of the Envoy escape-hatch key names.
// A fresh slice is returned on each call so callers cannot mutate the
// underlying set.
func EnvoyEscapeHatchKeyNames() []string {
	names := make([]string, 0, len(envoyEscapeHatchKeys))
	for key := range envoyEscapeHatchKeys {
		names = append(names, key)
	}
	return names
}

// HasEnvoyEscapeHatchKey reports whether the given opaque config map (e.g. a
// ConnectProxyConfig.Config or Upstream.Config value) sets any Envoy
// escape-hatch key.
func HasEnvoyEscapeHatchKey(cfg map[string]interface{}) bool {
	for key := range cfg {
		if IsEnvoyEscapeHatchKey(key) {
			return true
		}
	}
	return false
}

func (es EnvoyExtensions) ToAPI() []api.EnvoyExtension {
	extensions := make([]api.EnvoyExtension, len(es))
	for i, e := range es {
		extensions[i] = api.EnvoyExtension{
			Name:          e.Name,
			Required:      e.Required,
			Arguments:     e.Arguments,
			EnvoyVersion:  e.EnvoyVersion,
			ConsulVersion: e.ConsulVersion,
		}
	}
	return extensions
}
