// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
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
