package consul

import (
	"testing"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ComputeResolvedServiceConfig(t *testing.T) {
	type args struct {
		scReq       *structs.ServiceConfigRequest
		upstreamIDs []structs.ServiceID
		entries     *configentry.ResolvedServiceConfigSet
	}

	sid := structs.ServiceID{
		ID:             "sid",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	tests := []struct {
		name string
		args args
		want *structs.ServiceConfigResponse
	}{
		{
			name: "proxy with maxinboundsconnections",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &configentry.ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							MaxInboundConnections: 20,
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"max_inbound_connections": 20,
				},
			},
		},
		{
			name: "proxy with local_connect_timeout_ms and local_request_timeout_ms",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &configentry.ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							MaxInboundConnections: 20,
							LocalConnectTimeoutMs: 20000,
							LocalRequestTimeoutMs: 30000,
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"max_inbound_connections":  20,
					"local_connect_timeout_ms": 20000,
					"local_request_timeout_ms": 30000,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := computeResolvedServiceConfig(tt.args.scReq, tt.args.upstreamIDs,
				false, tt.args.entries, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
			name: "inherit transparent proxy settings",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
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

func Test_MergeServiceConfig_UpstreamOverrides(t *testing.T) {
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
			name: "new config fields",
			args: args{
				defaults: &structs.ServiceConfigResponse{
					UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: structs.ServiceID{
								ID:             "zap",
								EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							},
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
					UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: structs.ServiceID{
								ID:             "zap",
								EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							},
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
					UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: structs.ServiceID{
								ID:             "zap",
								EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							},
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
					UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: structs.ServiceID{
								ID:             "zap",
								EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							},
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
					UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
						{
							Upstream: structs.ServiceID{
								ID:             "zap",
								EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
							},
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
