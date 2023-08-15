// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestParseProxyConfig(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  ProxyConfig
	}{
		{
			name:  "defaults - nil",
			input: nil,
			want: ProxyConfig{
				Protocol:              "tcp",
				LocalConnectTimeoutMs: 5000,
			},
		},
		{
			name:  "defaults - empty",
			input: map[string]interface{}{},
			want: ProxyConfig{
				Protocol:              "tcp",
				LocalConnectTimeoutMs: 5000,
			},
		},
		{
			name: "defaults - other stuff",
			input: map[string]interface{}{
				"foo":       "bar",
				"envoy_foo": "envoy_bar",
			},
			want: ProxyConfig{
				Protocol:              "tcp",
				LocalConnectTimeoutMs: 5000,
			},
		},
		{
			name: "protocol override",
			input: map[string]interface{}{
				"protocol": "http",
			},
			want: ProxyConfig{
				Protocol:              "http",
				LocalConnectTimeoutMs: 5000,
			},
		},
		{
			name: "protocol uppercase override",
			input: map[string]interface{}{
				"protocol": "HTTP",
			},
			want: ProxyConfig{
				Protocol:              "http",
				LocalConnectTimeoutMs: 5000,
			},
		},
		{
			name: "bind address override, string",
			input: map[string]interface{}{
				"bind_address": "127.0.0.2",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				Protocol:              "tcp",
				BindAddress:           "127.0.0.2",
			},
		},
		{
			name: "bind port override, string",
			input: map[string]interface{}{
				"bind_port": "8888",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				Protocol:              "tcp",
				BindPort:              8888,
			},
		},
		{
			name: "bind port override, int",
			input: map[string]interface{}{
				"bind_port": 8889,
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				Protocol:              "tcp",
				BindPort:              8889,
			},
		},
		{
			name: "local connect timeout override, string",
			input: map[string]interface{}{
				"local_connect_timeout_ms": "1000",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 1000,
				Protocol:              "tcp",
			},
		},
		{
			name: "local connect timeout override, float ",
			input: map[string]interface{}{
				"local_connect_timeout_ms": float64(1000.0),
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 1000,
				Protocol:              "tcp",
			},
		},
		{
			name: "local connect timeout override, int ",
			input: map[string]interface{}{
				"local_connect_timeout_ms": 1000,
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 1000,
				Protocol:              "tcp",
			},
		},
		{
			name: "local request timeout override, string",
			input: map[string]interface{}{
				"local_request_timeout_ms": "1000",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalRequestTimeoutMs: intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "local request timeout override, float ",
			input: map[string]interface{}{
				"local_request_timeout_ms": float64(1000.0),
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalRequestTimeoutMs: intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "local request timeout override, int ",
			input: map[string]interface{}{
				"local_request_timeout_ms": 1000,
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalRequestTimeoutMs: intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "local idle timeout override, float ",
			input: map[string]interface{}{
				"local_idle_timeout_ms": float64(1000.0),
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalIdleTimeoutMs:    intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "local idle timeout override, int ",
			input: map[string]interface{}{
				"local_idle_timeout_ms": 1000,
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalIdleTimeoutMs:    intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "local idle timeout override, string",
			input: map[string]interface{}{
				"local_idle_timeout_ms": "1000",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs: 5000,
				LocalIdleTimeoutMs:    intPointer(1000),
				Protocol:              "tcp",
			},
		},
		{
			name: "balance inbound connections override, string",
			input: map[string]interface{}{
				"balance_inbound_connections": "exact_balance",
			},
			want: ProxyConfig{
				LocalConnectTimeoutMs:     5000,
				Protocol:                  "tcp",
				BalanceInboundConnections: "exact_balance",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProxyConfig(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseGatewayConfig(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  GatewayConfig
	}{
		{
			name:  "defaults - nil",
			input: nil,
			want: GatewayConfig{
				ConnectTimeoutMs: 5000,
			},
		},
		{
			name:  "defaults - empty",
			input: map[string]interface{}{},
			want: GatewayConfig{
				ConnectTimeoutMs: 5000,
			},
		},
		{
			name: "defaults - other stuff",
			input: map[string]interface{}{
				"foo":       "bar",
				"envoy_foo": "envoy_bar",
			},
			want: GatewayConfig{
				ConnectTimeoutMs: 5000,
			},
		},
		{
			name: "kitchen sink",
			input: map[string]interface{}{
				"envoy_gateway_bind_tagged_addresses": true,
				"envoy_gateway_bind_addresses":        map[string]structs.ServiceAddress{"foo": {Address: "127.0.0.1", Port: 80}},
				"envoy_gateway_no_default_bind":       true,
				"envoy_dns_discovery_type":            "StRiCt_DnS",
				"connect_timeout_ms":                  10,
			},
			want: GatewayConfig{
				ConnectTimeoutMs:    10,
				BindTaggedAddresses: true,
				NoDefaultBind:       true,
				BindAddresses:       map[string]structs.ServiceAddress{"foo": {Address: "127.0.0.1", Port: 80}},
				DNSDiscoveryType:    "strict_dns",
			},
		},
		{
			name: "deprecated kitchen sink",
			input: map[string]interface{}{
				"envoy_mesh_gateway_bind_tagged_addresses": true,
				"envoy_mesh_gateway_bind_addresses":        map[string]structs.ServiceAddress{"foo": {Address: "127.0.0.1", Port: 80}},
				"envoy_mesh_gateway_no_default_bind":       true,
				"connect_timeout_ms":                       10,
			},
			want: GatewayConfig{
				ConnectTimeoutMs:    10,
				BindTaggedAddresses: true,
				NoDefaultBind:       true,
				BindAddresses:       map[string]structs.ServiceAddress{"foo": {Address: "127.0.0.1", Port: 80}},
			},
		},
		{
			name: "new fields override deprecated ones",
			input: map[string]interface{}{
				// Deprecated
				"envoy_mesh_gateway_bind_tagged_addresses": true,
				"envoy_mesh_gateway_bind_addresses":        map[string]structs.ServiceAddress{"foo": {Address: "127.0.0.1", Port: 80}},
				"envoy_mesh_gateway_no_default_bind":       true,

				// New
				"envoy_gateway_bind_tagged_addresses": false,
				"envoy_gateway_bind_addresses":        map[string]structs.ServiceAddress{"bar": {Address: "127.0.0.1", Port: 8080}},
				"envoy_gateway_no_default_bind":       false,
			},
			want: GatewayConfig{
				ConnectTimeoutMs:    5000,
				BindTaggedAddresses: false,
				NoDefaultBind:       false,
				BindAddresses:       map[string]structs.ServiceAddress{"bar": {Address: "127.0.0.1", Port: 8080}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGatewayConfig(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func intPointer(i int) *int {
	return &i
}
