// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package extensionruntime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestGetRuntimeConfigurations_APIGateway(t *testing.T) {
	tests := []struct {
		name           string
		cfgSnap        *proxycfg.ConfigSnapshot
		expectedConfig map[api.CompoundServiceName][]extensioncommon.RuntimeConfig
	}{
		{
			name: "API Gateway with no extensions",
			cfgSnap: &proxycfg.ConfigSnapshot{
				Kind:    structs.ServiceKindAPIGateway,
				Proxy:   structs.ConnectProxyConfig{},
				Service: "api-gateway",
				ProxyID: proxycfg.ProxyID{
					ServiceID: structs.ServiceID{
						ID: "api-gateway",
					},
				},
			},
			expectedConfig: map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
				{
					Name:      "api-gateway",
					Namespace: "default",
				}: {},
			},
		},
		{
			name: "API Gateway with extensions",
			cfgSnap: &proxycfg.ConfigSnapshot{
				Kind: structs.ServiceKindAPIGateway,
				Proxy: structs.ConnectProxyConfig{
					EnvoyExtensions: []structs.EnvoyExtension{
						{
							Name: "builtin/lua",
							Arguments: map[string]interface{}{
								"Script": "function envoy_on_response(response_handle) response_handle:headers():add('x-test', 'test') end",
							},
						},
					},
				},
				Service: "api-gateway",
				ProxyID: proxycfg.ProxyID{
					ServiceID: structs.ServiceID{
						ID: "api-gateway",
					},
				},
			},
			expectedConfig: map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
				{
					Name:      "api-gateway",
					Namespace: "default",
				}: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: "builtin/lua",
							Arguments: map[string]interface{}{
								"Script": "function envoy_on_response(response_handle) response_handle:headers():add('x-test', 'test') end",
							},
						},
						ServiceName: api.CompoundServiceName{
							Name:      "api-gateway",
							Namespace: "default",
						},
						Upstreams:             map[api.CompoundServiceName]*extensioncommon.UpstreamData{},
						IsSourcedFromUpstream: false,
						Kind:                  api.ServiceKindAPIGateway,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetRuntimeConfigurations(tt.cfgSnap)
			require.Equal(t, tt.expectedConfig, config)
		})
	}
}
