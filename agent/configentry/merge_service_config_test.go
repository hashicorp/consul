// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"testing"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func Test_MergeServiceConfig_TransparentProxy(t *testing.T) {
	type args struct {
		defaults *structs.ServiceConfigResponse
		service  *structs.NodeService
	}
	tests := []struct {
		name string
		args args
		want *structs.NodeService
	}{
		{
			name: "inherit transparent proxy settings + kitchen sink",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
					ProxyConfig: map[string]interface{}{
						"foo": "bar",
					},
					MutualTLSMode: structs.MutualTLSModePermissive,
					Expose: structs.ExposeConfig{
						Checks: true,
						Paths: []structs.ExposePath{
							{
								ListenerPort: 8080,
								Path:         "/",
								Protocol:     "http",
							},
						},
					},
					MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
					AccessLogs: structs.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                structs.FileLogSinkType,
						Path:                "/tmp/accesslog.txt",
						JSONFormat:          "{ \"custom_start_time\": \"%START_TIME%\" }",
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						Mode:                   structs.ProxyModeDefault,
						TransparentProxy:       structs.TransparentProxyConfig{},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					Mode:                   structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
					MutualTLSMode: structs.MutualTLSModePermissive,
					Config: map[string]interface{}{
						"foo": "bar",
					},
					Expose: structs.ExposeConfig{
						Checks: true,
						Paths: []structs.ExposePath{
							{
								ListenerPort: 8080,
								Path:         "/",
								Protocol:     "http",
							},
						},
					},
					MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
					AccessLogs: structs.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                structs.FileLogSinkType,
						Path:                "/tmp/accesslog.txt",
						JSONFormat:          "{ \"custom_start_time\": \"%START_TIME%\" }",
					},
				},
			},
		},
		{
			name: "override transparent proxy settings",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       false,
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						Mode:                   structs.ProxyModeDirect,
						TransparentProxy: structs.TransparentProxyConfig{
							OutboundListenerPort: 808,
							DialedDirectly:       true,
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					Mode:                   structs.ProxyModeDirect,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 808,
						DialedDirectly:       true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultsCopy, err := copystructure.Copy(tt.args.defaults)
			require.NoError(t, err)

			got, err := MergeServiceConfig(tt.args.defaults, tt.args.service)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// The input defaults must not be modified by the merge.
			// See PR #10647
			assert.Equal(t, tt.args.defaults, defaultsCopy)
		})
	}
}

func Test_MergeServiceConfig_Extensions(t *testing.T) {
	type args struct {
		defaults *structs.ServiceConfigResponse
		service  *structs.NodeService
	}
	tests := []struct {
		name string
		args args
		want *structs.NodeService
	}{
		{
			name: "inherit extensions",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					EnvoyExtensions: []structs.EnvoyExtension{
						{
							Name:     "ext1",
							Required: true,
							Arguments: map[string]interface{}{
								"arg1": "val1",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					EnvoyExtensions: []structs.EnvoyExtension{
						{
							Name:     "ext1",
							Required: true,
							Arguments: map[string]interface{}{
								"arg1": "val1",
							},
						},
					},
				},
			},
		},
		{
			name: "replaces existing extensions",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					EnvoyExtensions: []structs.EnvoyExtension{
						{
							Name:     "ext1",
							Required: true,
							Arguments: map[string]interface{}{
								"arg1": "val1",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						EnvoyExtensions: []structs.EnvoyExtension{
							{
								Name:     "existing-ext",
								Required: true,
								Arguments: map[string]interface{}{
									"arg1": "val1",
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					EnvoyExtensions: []structs.EnvoyExtension{
						{
							Name:     "ext1",
							Required: true,
							Arguments: map[string]interface{}{
								"arg1": "val1",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultsCopy, err := copystructure.Copy(tt.args.defaults)
			require.NoError(t, err)

			got, err := MergeServiceConfig(tt.args.defaults, tt.args.service)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// The input defaults must not be modified by the merge.
			// See PR #10647
			assert.Equal(t, tt.args.defaults, defaultsCopy)
		})
	}
}

func isEnterprise() bool {
	return acl.PartitionOrDefault("") == "default"
}

func Test_MergeServiceConfig_peeredCentralDefaultsMerging(t *testing.T) {
	partitions := []string{"default"}
	if isEnterprise() {
		partitions = append(partitions, "part1")
	}

	const peerName = "my-peer"

	newDefaults := func(partition string) *structs.ServiceConfigResponse {
		// client agents
		return &structs.ServiceConfigResponse{
			ProxyConfig: map[string]any{
				"protocol": "http",
			},
			UpstreamConfigs: []structs.OpaqueUpstreamConfig{
				{
					Upstream: structs.PeeredServiceName{
						ServiceName: structs.ServiceName{
							Name:           "*",
							EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "*"),
						},
					},
					Config: map[string]any{
						"mesh_gateway": map[string]any{
							"Mode": "local",
						},
						"protocol": "http",
					},
				},
				{
					Upstream: structs.PeeredServiceName{
						ServiceName: structs.ServiceName{
							Name:           "static-server",
							EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "default"),
						},
						Peer: peerName,
					},
					Config: map[string]any{
						"mesh_gateway": map[string]any{
							"Mode": "local",
						},
						"protocol": "http",
					},
				},
			},
			MeshGateway: structs.MeshGatewayConfig{
				Mode: "local",
			},
		}
	}

	for _, partition := range partitions {
		t.Run("partition="+partition, func(t *testing.T) {
			t.Run("clients", func(t *testing.T) {
				defaults := newDefaults(partition)

				service := &structs.NodeService{
					Kind:    "connect-proxy",
					ID:      "static-client-sidecar-proxy",
					Service: "static-client-sidecar-proxy",
					Address: "",
					Port:    21000,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "static-client",
						DestinationServiceID:   "static-client",
						LocalServiceAddress:    "127.0.0.1",
						LocalServicePort:       8080,
						Upstreams: []structs.Upstream{
							{
								DestinationType:      "service",
								DestinationNamespace: "default",
								DestinationPartition: partition,
								DestinationPeer:      peerName,
								DestinationName:      "static-server",
								LocalBindAddress:     "0.0.0.0",
								LocalBindPort:        5000,
							},
						},
					},
					EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "default"),
				}

				expect := &structs.NodeService{
					Kind:    "connect-proxy",
					ID:      "static-client-sidecar-proxy",
					Service: "static-client-sidecar-proxy",
					Address: "",
					Port:    21000,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "static-client",
						DestinationServiceID:   "static-client",
						LocalServiceAddress:    "127.0.0.1",
						LocalServicePort:       8080,
						Config: map[string]any{
							"protocol": "http",
						},
						Upstreams: []structs.Upstream{
							{
								DestinationType:      "service",
								DestinationNamespace: "default",
								DestinationPartition: partition,
								DestinationPeer:      peerName,
								DestinationName:      "static-server",
								LocalBindAddress:     "0.0.0.0",
								LocalBindPort:        5000,
								MeshGateway: structs.MeshGatewayConfig{
									Mode: "local",
								},
								Config: map[string]any{},
							},
						},
						MeshGateway: structs.MeshGatewayConfig{
							Mode: "local",
						},
					},
					EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "default"),
				}

				got, err := MergeServiceConfig(defaults, service)
				require.NoError(t, err)
				require.Equal(t, expect, got)
			})

			t.Run("dataplanes", func(t *testing.T) {
				defaults := newDefaults(partition)

				service := &structs.NodeService{
					Kind:    "connect-proxy",
					ID:      "static-client-sidecar-proxy",
					Service: "static-client-sidecar-proxy",
					Address: "10.61.57.9",
					TaggedAddresses: map[string]structs.ServiceAddress{
						"consul-virtual": {
							Address: "240.0.0.2",
							Port:    20000,
						},
					},
					Port: 20000,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "static-client",
						DestinationServiceID:   "static-client",
						LocalServicePort:       8080,
						Upstreams: []structs.Upstream{
							{
								DestinationType:      "",
								DestinationNamespace: "default",
								DestinationPeer:      peerName,
								DestinationName:      "static-server",
								LocalBindAddress:     "0.0.0.0",
								LocalBindPort:        5000,
							},
						},
					},
					EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "default"),
				}

				expect := &structs.NodeService{
					Kind:    "connect-proxy",
					ID:      "static-client-sidecar-proxy",
					Service: "static-client-sidecar-proxy",
					Address: "10.61.57.9",
					TaggedAddresses: map[string]structs.ServiceAddress{
						"consul-virtual": {
							Address: "240.0.0.2",
							Port:    20000,
						},
					},
					Port: 20000,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "static-client",
						DestinationServiceID:   "static-client",
						LocalServicePort:       8080,
						Config: map[string]any{
							"protocol": "http",
						},
						Upstreams: []structs.Upstream{
							{
								DestinationType:      "",
								DestinationNamespace: "default",
								DestinationPeer:      peerName,
								DestinationName:      "static-server",
								LocalBindAddress:     "0.0.0.0",
								LocalBindPort:        5000,
								MeshGateway: structs.MeshGatewayConfig{
									Mode: "local", // This field vanishes if the merging does not work for dataplanes.
								},
								Config: map[string]any{},
							},
						},
						MeshGateway: structs.MeshGatewayConfig{
							Mode: "local",
						},
					},
					EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(partition, "default"),
				}

				got, err := MergeServiceConfig(defaults, service)
				require.NoError(t, err)
				require.Equal(t, expect, got)
			})
		})
	}
}

func Test_MergeServiceConfig_UpstreamOverrides(t *testing.T) {
	type args struct {
		defaults *structs.ServiceConfigResponse
		service  *structs.NodeService
	}
	zapUpstreamId := structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("zap", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}
	zapPeeredUpstreamId := structs.PeeredServiceName{
		Peer:        "some-peer",
		ServiceName: structs.NewServiceName("zap", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}
	tests := []struct {
		name string
		args args
		want *structs.NodeService
	}{
		{
			name: "new config fields",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"passive_health_check": map[string]interface{}{
									"Interval":    int64(10),
									"MaxFailures": int64(2),
								},
								"mesh_gateway": map[string]interface{}{
									"Mode": "local",
								},
								"protocol": "grpc",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationNamespace: "default",
								DestinationPartition: "default",
								DestinationName:      "zap",
								Config: map[string]interface{}{
									"passive_health_check": map[string]interface{}{
										"Interval":    int64(20),
										"MaxFailures": int64(4),
									},
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zap",
							Config: map[string]interface{}{
								"passive_health_check": map[string]interface{}{
									"Interval":    int64(20),
									"MaxFailures": int64(4),
								},
								"protocol": "grpc",
							},
							MeshGateway: structs.MeshGatewayConfig{
								Mode: structs.MeshGatewayModeLocal,
							},
						},
					},
				},
			},
		},
		{
			name: "remote upstream config expands local upstream list in transparent mode",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"protocol": "grpc",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						Mode:                   structs.ProxyModeTransparent,
						TransparentProxy: structs.TransparentProxyConfig{
							OutboundListenerPort: 10101,
							DialedDirectly:       true,
						},
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationNamespace: "default",
								DestinationPartition: "default",
								DestinationName:      "zip",
								LocalBindPort:        8080,
								Config: map[string]interface{}{
									"protocol": "http",
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					Mode:                   structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zip",
							LocalBindPort:        8080,
							Config: map[string]interface{}{
								"protocol": "http",
							},
						},
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zap",
							Config: map[string]interface{}{
								"protocol": "grpc",
							},
							CentrallyConfigured: true,
						},
					},
				},
			},
		},
		{
			name: "remote upstream config not added to local upstream list outside of transparent mode",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"protocol": "grpc",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						Mode:                   structs.ProxyModeDirect,
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationNamespace: "default",
								DestinationPartition: "default",
								DestinationName:      "zip",
								LocalBindPort:        8080,
								Config: map[string]interface{}{
									"protocol": "http",
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					Mode:                   structs.ProxyModeDirect,
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zip",
							LocalBindPort:        8080,
							Config: map[string]interface{}{
								"protocol": "http",
							},
						},
					},
				},
			},
		},
		{
			name: "upstream mode from remote defaults overrides local default",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"mesh_gateway": map[string]interface{}{
									"Mode": "local",
								},
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						MeshGateway: structs.MeshGatewayConfig{
							Mode: structs.MeshGatewayModeRemote,
						},
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationNamespace: "default",
								DestinationPartition: "default",
								DestinationName:      "zap",
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zap",
							Config:               map[string]interface{}{},
							MeshGateway: structs.MeshGatewayConfig{
								Mode: structs.MeshGatewayModeLocal,
							},
						},
					},
				},
			},
		},
		{
			name: "mode in local upstream config overrides all",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"mesh_gateway": map[string]interface{}{
									"Mode": "local",
								},
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo",
						MeshGateway: structs.MeshGatewayConfig{
							Mode: structs.MeshGatewayModeRemote,
						},
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationNamespace: "default",
								DestinationPartition: "default",
								DestinationName:      "zap",
								MeshGateway: structs.MeshGatewayConfig{
									Mode: structs.MeshGatewayModeNone,
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationNamespace: "default",
							DestinationPartition: "default",
							DestinationName:      "zap",
							Config:               map[string]interface{}{},
							MeshGateway: structs.MeshGatewayConfig{
								Mode: structs.MeshGatewayModeNone,
							},
						},
					},
				},
			},
		},
		{
			name: "peering upstreams are distinct from local-cluster upstreams",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapUpstreamId,
							Config: map[string]interface{}{
								"connect_timeout_ms": 2222,
							},
						},
						{
							Upstream: zapPeeredUpstreamId,
							Config: map[string]interface{}{
								"connect_timeout_ms": 3333,
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "zap",
							},
							structs.Upstream{
								DestinationPeer: "some-peer",
								DestinationName: "zap",
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationName: "zap",
							Config: map[string]interface{}{
								"connect_timeout_ms": 2222,
							},
						},
						structs.Upstream{
							DestinationPeer: "some-peer",
							DestinationName: "zap",
							Config: map[string]interface{}{
								"connect_timeout_ms": 3333,
							},
						},
					},
				},
			},
		},
		{
			name: "peering upstreams ignore protocol overrides",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapPeeredUpstreamId,
							Config: map[string]interface{}{
								"protocol": "http",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationPeer: "some-peer",
								DestinationName: "zap",
								Config: map[string]interface{}{
									"protocol": "tcp",
								},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationPeer: "some-peer",
							DestinationName: "zap",
							Config: map[string]interface{}{
								"protocol": "tcp",
							},
						},
					},
				},
			},
		},
		{
			name: "peering upstreams ignore protocol overrides with unset value",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: zapPeeredUpstreamId,
							Config: map[string]interface{}{
								"protocol": "http",
							},
						},
					},
				},
				service: &structs.NodeService{
					ID:      "foo-proxy",
					Service: "foo-proxy",
					Proxy: structs.ConnectProxyConfig{
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationPeer: "some-peer",
								DestinationName: "zap",
								Config:          map[string]interface{}{},
							},
						},
					},
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					Upstreams: structs.Upstreams{
						structs.Upstream{
							DestinationPeer: "some-peer",
							DestinationName: "zap",
							Config:          map[string]interface{}{},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultsCopy, err := copystructure.Copy(tt.args.defaults)
			require.NoError(t, err)

			got, err := MergeServiceConfig(tt.args.defaults, tt.args.service)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// The input defaults must not be modified by the merge.
			// See PR #10647
			assert.Equal(t, tt.args.defaults, defaultsCopy)
		})
	}
}

// Tests that RateLimit config is a no-op in non-enterprise.
// In practice, the ratelimit config would have been validated
// on write.
func Test_MergeServiceConfig_RateLimit(t *testing.T) {
	rl := structs.RateLimits{
		InstanceLevel: structs.InstanceLevelRateLimits{
			RequestsPerSecond: 1234,
			RequestsMaxBurst:  2345,
			Routes: []structs.InstanceLevelRouteRateLimits{
				{
					PathExact:         "/admin",
					RequestsPerSecond: 3333,
					RequestsMaxBurst:  4444,
				},
			},
		},
	}
	tests := []struct {
		name     string
		defaults *structs.ServiceConfigResponse
		service  *structs.NodeService
		want     *structs.NodeService
	}{
		{
			name: "injects ratelimit extension",
			defaults: &structs.ServiceConfigResponse{
				RateLimits: rl,
			},
			service: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
				},
			},
			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					EnvoyExtensions: func() []structs.EnvoyExtension {
						if ext := rl.ToEnvoyExtension(); ext != nil {
							return []structs.EnvoyExtension{*ext}
						}
						return nil
					}(),
				},
			},
		},
		{
			name: "injects ratelimit extension at the end",
			defaults: &structs.ServiceConfigResponse{
				RateLimits: rl,
				EnvoyExtensions: []structs.EnvoyExtension{
					{
						Name:     "existing-ext",
						Required: true,
						Arguments: map[string]interface{}{
							"arg1": "val1",
						},
					},
				},
			},
			service: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
				},
			},

			want: &structs.NodeService{
				ID:      "foo-proxy",
				Service: "foo-proxy",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "foo",
					DestinationServiceID:   "foo",
					EnvoyExtensions: func() []structs.EnvoyExtension {
						existing := []structs.EnvoyExtension{
							{
								Name:     "existing-ext",
								Required: true,
								Arguments: map[string]interface{}{
									"arg1": "val1",
								},
							},
						}
						if ext := rl.ToEnvoyExtension(); ext != nil {
							existing = append(existing, *ext)
						}
						return existing
					}(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeServiceConfig(tt.defaults, tt.service)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
