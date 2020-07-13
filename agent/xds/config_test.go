package xds

import (
	"testing"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/consul/agent/structs"
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
		{
			name: "connect limits map",
			input: map[string]interface{}{
				"limits": map[string]interface{}{
					"max_connections":         50,
					"max_pending_requests":    60,
					"max_concurrent_requests": 70,
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
				Limits: UpstreamLimits{
					MaxConnections:        intPointer(50),
					MaxPendingRequests:    intPointer(60),
					MaxConcurrentRequests: intPointer(70),
				},
			},
		},
		{
			name: "connect limits map zero",
			input: map[string]interface{}{
				"limits": map[string]interface{}{
					"max_connections":         0,
					"max_pending_requests":    0,
					"max_concurrent_requests": 0,
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
				Limits: UpstreamLimits{
					MaxConnections:        intPointer(0),
					MaxPendingRequests:    intPointer(0),
					MaxConcurrentRequests: intPointer(0),
				},
			},
		},
		{
			name: "passive health check map",
			input: map[string]interface{}{
				"passive_health_check": map[string]interface{}{
					"interval":     "22s",
					"max_failures": 7,
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
				PassiveHealthCheck: PassiveHealthCheck{
					Interval:    22 * time.Second,
					MaxFailures: 7,
				},
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

type raw map[string]interface{}

func TestParseUpstreamConfigNoDefaults_LoadBalancer(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  UpstreamConfig
	}{
		{
			name: "random",
			input: raw{
				"load_balancer": raw{
					"policy": "random",
				},
			},
			want: UpstreamConfig{
				LoadBalancer: LoadBalancer{
					Policy: "random",
				},
			},
		},
		{
			name: "ring hash",
			input: raw{
				"load_balancer": raw{
					"policy": "ring_hash",
					"ring_hash": raw{
						"minimum_ring_size": 3,
						"maximum_ring_size": 7,
					},
					"hash_policy": []raw{
						{"field": "header", "match_value": "x-route-key", "terminal": "true"},
						{"field": "connection_property_source_ip", "terminal": "true"},
					},
				},
			},
			want: UpstreamConfig{
				LoadBalancer: LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: RingHashConfig{
						MinimumRingSize: 3,
						MaximumRingSize: 7,
					},
					HashPolicy: []HashPolicy{
						{Field: "header", MatchValue: "x-route-key", Terminal: true},
						{Field: "connection_property_source_ip", Terminal: true},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUpstreamConfigNoDefaults(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
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
func TestLoadBalancer_ApplyToCluster(t *testing.T) {
	var tests = []struct {
		name     string
		lb       LoadBalancer
		expected envoy.Cluster
	}{
		{
			name: "random",
			lb: LoadBalancer{
				Policy: "random",
			},
			expected: envoy.Cluster{LbPolicy: envoy.Cluster_RANDOM},
		},
		{
			name: "ring_hash",
			lb: LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
			},
			expected: envoy.Cluster{
				LbPolicy: envoy.Cluster_RING_HASH,
				LbConfig: &envoy.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy.Cluster_RingHashLbConfig{
						MinimumRingSize: &wrappers.UInt64Value{Value: 3},
						MaximumRingSize: &wrappers.UInt64Value{Value: 7},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &envoy.Cluster{}
			err := tc.lb.ApplyToCluster(c)
			require.NoError(t, err)

			require.Equal(t, &tc.expected, c)
		})
	}
}

func TestLoadBalancer_ApplyHashPolicyToRouteAction(t *testing.T) {
	var tests = []struct {
		name     string
		lb       LoadBalancer
		expected envoyroute.RouteAction
	}{
		{
			name: "random",
			lb: LoadBalancer{
				Policy: "random",
			},
			expected: envoyroute.RouteAction{},
		},
		{
			name: "ring_hash",
			lb: LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
				HashPolicy: []HashPolicy{
					{Field: "header", MatchValue: "x-route-key", Terminal: true},
					{Field: "connection_property_source_ip", MatchValue: "true"},
				},
			},
			expected: envoyroute.RouteAction{
				HashPolicy: []*envoyroute.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoyroute.RouteAction_HashPolicy_Header_{
							Header: &envoyroute.RouteAction_HashPolicy_Header{
								HeaderName: "x-route-key",
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoyroute.RouteAction_HashPolicy_ConnectionProperties_{
							ConnectionProperties: &envoyroute.RouteAction_HashPolicy_ConnectionProperties{
								SourceIp: true,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ra := &envoyroute.RouteAction{}
			err := tc.lb.ApplyHashPolicyToRouteAction(ra)
			require.NoError(t, err)

			require.Equal(t, &tc.expected, ra)
		})
	}
}
