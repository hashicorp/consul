// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lua

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_lua_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestConstructor(t *testing.T) {
	makeArguments := func(overrides map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{
			"ProxyType": "connect-proxy",
			"Listener":  "inbound",
			"Script":    "function envoy_on_request(request_handle) request_handle:headers():add('test', 'test') end",
		}
		for k, v := range overrides {
			m[k] = v
		}
		return m
	}

	tests := map[string]struct {
		extensionName string
		arguments     map[string]interface{}
		expected      lua
		ok            bool
	}{
		"with no arguments": {
			arguments: nil,
			ok:        false,
		},
		"with an invalid name": {
			arguments:     makeArguments(map[string]interface{}{}),
			extensionName: "bad",
			ok:            false,
		},
		"empty script": {
			arguments: makeArguments(map[string]interface{}{"Script": ""}),
			ok:        false,
		},
		"invalid proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": "terminating-gateway"}),
			ok:        false,
		},
		"invalid listener": {
			arguments: makeArguments(map[string]interface{}{"Listener": "invalid"}),
			ok:        false,
		},
		"default proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": ""}),
			expected: lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) request_handle:headers():add('test', 'test') end",
			},
			ok: true,
		},
		"api gateway proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": "api-gateway"}),
			expected: lua{
				ProxyType: "api-gateway",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) request_handle:headers():add('test', 'test') end",
			},
			ok: true,
		},
		"valid everything": {
			arguments: makeArguments(map[string]interface{}{}),
			expected: lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) request_handle:headers():add('test', 'test') end",
			},
			ok: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			extensionName := api.BuiltinLuaExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}

			ext := extensioncommon.RuntimeConfig{
				ServiceName: api.CompoundServiceName{Name: "svc"},
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
				},
			}

			e, err := Constructor(ext.EnvoyExtension)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, &extensioncommon.BasicEnvoyExtender{Extension: &tc.expected}, e)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLuaExtension_PatchFilter(t *testing.T) {
	makeFilter := func(filters []*envoy_http_v3.HttpFilter) *envoy_listener_v3.Filter {
		hcm := &envoy_http_v3.HttpConnectionManager{
			HttpFilters: filters,
		}
		any, err := anypb.New(hcm)
		require.NoError(t, err)
		return &envoy_listener_v3.Filter{
			Name: "envoy.filters.network.http_connection_manager",
			ConfigType: &envoy_listener_v3.Filter_TypedConfig{
				TypedConfig: any,
			},
		}
	}

	makeLuaFilter := func(script string) *envoy_http_v3.HttpFilter {
		luaConfig := &envoy_lua_v3.Lua{
			DefaultSourceCode: &envoy_core_v3.DataSource{
				Specifier: &envoy_core_v3.DataSource_InlineString{
					InlineString: script,
				},
			},
		}
		return &envoy_http_v3.HttpFilter{
			Name: "envoy.filters.http.lua",
			ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{
				TypedConfig: mustMarshalAny(luaConfig),
			},
		}
	}

	tests := map[string]struct {
		extension      *lua
		filter         *envoy_listener_v3.Filter
		isInbound      bool
		expectedFilter *envoy_listener_v3.Filter
		expectPatched  bool
		expectError    string
	}{
		"non-http filter is ignored": {
			extension: &lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) end",
			},
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.tcp_proxy",
			},
			expectedFilter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.tcp_proxy",
			},
			expectPatched: false,
		},
		"listener direction mismatch": {
			extension: &lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) end",
			},
			filter: makeFilter([]*envoy_http_v3.HttpFilter{
				{Name: "envoy.filters.http.router"},
			}),
			isInbound:      false,
			expectedFilter: makeFilter([]*envoy_http_v3.HttpFilter{{Name: "envoy.filters.http.router"}}),
			expectPatched:  false,
		},
		"successful patch with router filter": {
			extension: &lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) end",
			},
			filter: makeFilter([]*envoy_http_v3.HttpFilter{
				{Name: "envoy.filters.http.router"},
			}),
			isInbound: true,
			expectedFilter: makeFilter([]*envoy_http_v3.HttpFilter{
				makeLuaFilter("function envoy_on_request(request_handle) end"),
				{Name: "envoy.filters.http.router"},
			}),
			expectPatched: true,
		},
		"successful patch with multiple filters": {
			extension: &lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) end",
			},
			filter: makeFilter([]*envoy_http_v3.HttpFilter{
				{Name: "envoy.filters.http.other1"},
				{Name: "envoy.filters.http.router"},
				{Name: "envoy.filters.http.other2"},
			}),
			isInbound: true,
			expectedFilter: makeFilter([]*envoy_http_v3.HttpFilter{
				{Name: "envoy.filters.http.other1"},
				makeLuaFilter("function envoy_on_request(request_handle) end"),
				{Name: "envoy.filters.http.router"},
				{Name: "envoy.filters.http.other2"},
			}),
			expectPatched: true,
		},
		"invalid filter config": {
			extension: &lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "function envoy_on_request(request_handle) end",
			},
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: &anypb.Any{},
				},
			},
			isInbound:      true,
			expectedFilter: nil,
			expectPatched:  false,
			expectError:    "error unmarshalling filter",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			direction := extensioncommon.TrafficDirectionOutbound
			if tc.isInbound {
				direction = extensioncommon.TrafficDirectionInbound
			}

			payload := extensioncommon.FilterPayload{
				Message:          tc.filter,
				TrafficDirection: direction,
			}

			filter, patched, err := tc.extension.PatchFilter(payload)

			if tc.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectPatched, patched)
			if tc.expectedFilter != nil {
				require.Equal(t, tc.expectedFilter, filter)
			}
		})
	}
}

func mustMarshalAny(m proto.Message) *anypb.Any {
	a, err := anypb.New(m)
	if err != nil {
		panic(err)
	}
	return a
}
