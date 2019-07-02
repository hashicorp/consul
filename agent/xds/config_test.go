package xds

import (
	"testing"

	"github.com/stretchr/testify/require"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProxyConfig(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseUpstreamConfig(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  UpstreamConfig
	}{
		{
			name:  "defaults - nil",
			input: nil,
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name:  "defaults - empty",
			input: map[string]interface{}{},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name: "defaults - other stuff",
			input: map[string]interface{}{
				"foo":       "bar",
				"envoy_foo": "envoy_bar",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name: "protocol override",
			input: map[string]interface{}{
				"protocol": "http",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "http",
			},
		},
		{
			name: "connect timeout override, string",
			input: map[string]interface{}{
				"connect_timeout_ms": "1000",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
		{
			name: "connect timeout override, float ",
			input: map[string]interface{}{
				"connect_timeout_ms": float64(1000.0),
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
		{
			name: "connect timeout override, int ",
			input: map[string]interface{}{
				"connect_timeout_ms": 1000,
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUpstreamConfig(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
