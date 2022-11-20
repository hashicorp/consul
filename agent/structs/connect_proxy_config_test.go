package structs

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestConnectProxyConfig_ToAPI(t *testing.T) {
	tests := []struct {
		name string
		in   ConnectProxyConfig
		want *api.AgentServiceConnectProxyConfig
	}{
		{
			name: "service",
			in: ConnectProxyConfig{
				DestinationServiceName: "web",
				DestinationServiceID:   "web1",
				LocalServiceAddress:    "127.0.0.2",
				LocalServicePort:       5555,
				Config: map[string]interface{}{
					"foo": "bar",
				},
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeLocal,
				},
				Upstreams: Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc1",
						LocalBindPort:   1234,
						MeshGateway: MeshGatewayConfig{
							Mode: MeshGatewayModeLocal,
						},
					},
					{
						DestinationType:  UpstreamDestTypePreparedQuery,
						DestinationName:  "foo",
						Datacenter:       "dc1",
						LocalBindPort:    2345,
						LocalBindAddress: "127.10.10.10",
					},
				},
				Mode: ProxyModeTransparent,
				TransparentProxy: TransparentProxyConfig{
					OutboundListenerPort: 808,
				},
			},
			want: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: "web",
				DestinationServiceID:   "web1",
				LocalServiceAddress:    "127.0.0.2",
				LocalServicePort:       5555,
				Config: map[string]interface{}{
					"foo": "bar",
				},
				MeshGateway: api.MeshGatewayConfig{
					Mode: api.MeshGatewayModeLocal,
				},
				Upstreams: []api.Upstream{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc1",
						LocalBindPort:   1234,
						MeshGateway: api.MeshGatewayConfig{
							Mode: api.MeshGatewayModeLocal,
						},
					},
					{
						DestinationType:  UpstreamDestTypePreparedQuery,
						DestinationName:  "foo",
						Datacenter:       "dc1",
						LocalBindPort:    2345,
						LocalBindAddress: "127.10.10.10",
					},
				},
				Mode: api.ProxyModeTransparent,
				TransparentProxy: &api.TransparentProxyConfig{
					OutboundListenerPort: 808,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.ToAPI())
		})
	}
}

func TestConnectProxyConfig_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      ConnectProxyConfig
		want    string
		wantErr bool
	}{
		{
			name: "direct proxy",
			in: ConnectProxyConfig{
				DestinationServiceName: "api",
				DestinationServiceID:   "api-1",
				LocalServiceAddress:    "127.0.0.1",
				LocalServicePort:       8080,
				Mode:                   ProxyModeDirect,
				Config: map[string]interface{}{
					"connect_timeout_ms": 5000,
				},
				Upstreams: Upstreams{
					Upstream{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "db",
						Datacenter:      "dc1",
						LocalBindPort:   1234,
					},
				},
				MeshGateway: MeshGatewayConfig{Mode: MeshGatewayModeLocal},
				Expose:      ExposeConfig{Checks: true},

				// No transparent proxy config, since proxy is set to "direct" mode.
				// Field should be omitted from json output.
				// TransparentProxy: TransparentProxyConfig{},
			},
			want: `{
				"DestinationServiceName": "api",
				"DestinationServiceID": "api-1",
				"LocalServiceAddress": "127.0.0.1",
				"LocalServicePort": 8080,
				"Mode": "direct",
				"Config": {
					"connect_timeout_ms": 5000
				},
				"Upstreams": [
					{
						"DestinationType": "service",
						"DestinationName": "db",
						"Datacenter": "dc1",
						"LocalBindPort": 1234,
						"MeshGateway": {}
					}
				],
				"MeshGateway": {
					"Mode": "local"
				},
				"Expose": {
					"Checks": true
				}
			}`,
			wantErr: false,
		},
		{
			name: "transparent proxy",
			in: ConnectProxyConfig{
				DestinationServiceName: "billing",
				DestinationServiceID:   "billing-1",
				LocalServiceAddress:    "127.0.0.1",
				LocalServicePort:       8080,
				Mode:                   ProxyModeTransparent,
				Config: map[string]interface{}{
					"connect_timeout_ms": 5000,
				},
				MeshGateway: MeshGatewayConfig{Mode: MeshGatewayModeLocal},
				Expose:      ExposeConfig{Checks: true},
				TransparentProxy: TransparentProxyConfig{
					DialedDirectly:       true,
					OutboundListenerPort: 16001,
				},
			},
			want: `{
				"DestinationServiceName": "billing",
				"DestinationServiceID": "billing-1",
				"LocalServiceAddress": "127.0.0.1",
				"LocalServicePort": 8080,
				"Mode": "transparent",
				"Config": {
					"connect_timeout_ms": 5000
				},
				"TransparentProxy": {
					"DialedDirectly": true,
					"OutboundListenerPort": 16001
				},
				"MeshGateway": {
					"Mode": "local"
				},
				"Expose": {
					"Checks": true
				}
			}`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.MarshalJSON()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestUpstream_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      Upstream
		want    string
		wantErr bool
	}{
		{
			name: "service",
			in: Upstream{
				DestinationType: UpstreamDestTypeService,
				DestinationName: "foo",
				Datacenter:      "dc1",
				LocalBindPort:   1234,
				// Test IngressHosts does not marshal
				IngressHosts: []string{"test.example.com"},
			},
			want: `{
				"DestinationType": "service",
				"DestinationName": "foo",
				"Datacenter": "dc1",
				"LocalBindPort": 1234,
				"MeshGateway": {}
			}`,
			wantErr: false,
		},
		{
			name: "pq",
			in: Upstream{
				DestinationType: UpstreamDestTypePreparedQuery,
				DestinationName: "foo",
				Datacenter:      "dc1",
				LocalBindPort:   1234,
			},
			want: `{
				"DestinationType": "prepared_query",
				"DestinationName": "foo",
				"Datacenter": "dc1",
				"LocalBindPort": 1234,
				"MeshGateway": {}
			}`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestUpstream_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		jsonSnake string
		want      Upstream
		wantErr   bool
	}{
		{
			name: "service",
			json: `{
				"DestinationType": "service",
				"DestinationName": "foo",
				"Datacenter": "dc1"
			}`,
			want: Upstream{
				DestinationType: UpstreamDestTypeService,
				DestinationName: "foo",
				Datacenter:      "dc1",
			},
			wantErr: false,
		},
		{
			name: "pq",
			json: `{
				"DestinationType": "prepared_query",
				"DestinationName": "foo",
				"Datacenter": "dc1"
			}`,
			want: Upstream{
				DestinationType: UpstreamDestTypePreparedQuery,
				DestinationName: "foo",
				Datacenter:      "dc1",
			},
			wantErr: false,
		},
		{
			name: "ingress-hosts-do-not-unmarshal",
			json: `{
				"DestinationType": "service",
				"DestinationName": "foo",
				"Datacenter": "dc1",
				"IngressHosts": ["asdf"]
			}`,
			want: Upstream{
				DestinationType: UpstreamDestTypeService,
				DestinationName: "foo",
				Datacenter:      "dc1",
				IngressHosts:    nil, // Make sure this doesn't get parsed
			},
			wantErr: true,
		},
		{
			name: "kitchen sink",
			json: `
				{
				  "DestinationType": "service",
				  "DestinationNamespace": "default",
				  "DestinationName": "bar1",
				  "Datacenter": "dc1",
				  "LocalBindAddress": "127.0.0.2",
				  "LocalBindPort": 6060,
				  "Config": {
					"x": "y",
					"z": -2
				  },
				  "MeshGateway": {
					"Mode": "local"
				  }
				}
			`,
			jsonSnake: `
				{
				  "destination_type": "service",
				  "destination_namespace": "default",
				  "destination_name": "bar1",
				  "datacenter": "dc1",
				  "local_bind_address": "127.0.0.2",
				  "local_bind_port": 6060,
				  "config": {
					"x": "y",
					"z": -2
				  },
				  "mesh_gateway": {
					"mode": "local"
				  }
				}
			`,
			want: Upstream{
				DestinationType:      UpstreamDestTypeService,
				DestinationNamespace: "default",
				DestinationName:      "bar1",
				Datacenter:           "dc1",
				LocalBindAddress:     "127.0.0.2",
				LocalBindPort:        6060,
				Config: map[string]interface{}{
					"x": "y",
					"z": float64(-2),
				},
				MeshGateway: MeshGatewayConfig{
					Mode: "local",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("camel", func(t *testing.T) {
				var got Upstream
				err := json.Unmarshal([]byte(tt.json), &got)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, tt.want, got, "%+v", got)
				}
			})

			if tt.jsonSnake != "" {
				t.Run("snake", func(t *testing.T) {
					var got Upstream
					err := json.Unmarshal([]byte(tt.jsonSnake), &got)
					if tt.wantErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.Equal(t, tt.want, got)
					}
				})
			}
		})
	}
}

func TestConnectProxyConfig_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		jsonSnake string
		want      ConnectProxyConfig
		wantErr   bool
	}{
		{
			name: "kitchen sink",
			json: `
				{
				  "DestinationServiceName": "foo-name",
				  "DestinationServiceID": "foo-id",
				  "LocalServiceAddress": "127.0.0.1",
				  "LocalServicePort": 5050,
				  "Config": {
					"a": "b",
					"v": 42
				  },
				  "Upstreams": [
					{
					  "DestinationType": "service",
					  "DestinationNamespace": "default",
					  "DestinationName": "bar1",
					  "Datacenter": "dc1",
					  "LocalBindAddress": "127.0.0.2",
					  "LocalBindPort": 6060,
					  "Config": {
						"x": "y",
						"z": -2
					  },
					  "MeshGateway": {
						"Mode": "local"
					  }
					},
					{
					  "DestinationType": "service",
					  "DestinationNamespace": "default",
					  "DestinationName": "bar2",
					  "Datacenter": "dc2",
					  "LocalBindAddress": "127.0.0.2",
					  "LocalBindPort": 6161
					}
				  ],
				  "MeshGateway": {
					"Mode": "remote"
				  },
				  "Expose": {
					"Checks": true,
					"Paths": [
					  {
						"ListenerPort": 8080,
						"Path": "/foo",
						"LocalPathPort": 7070,
						"Protocol": "http2",
						"ParsedFromCheck": true
					  },
					  {
						"ListenerPort": 8181,
						"Path": "/foo2",
						"LocalPathPort": 7171,
						"Protocol": "http",
						"ParsedFromCheck": false
					  }
					]
				  }
				}
			`,
			jsonSnake: `
				{
				  "destination_service_name": "foo-name",
				  "destination_service_id": "foo-id",
				  "local_service_address": "127.0.0.1",
				  "local_service_port": 5050,
				  "config": {
					"a": "b",
					"v": 42
				  },
				  "upstreams": [
					{
					  "destination_type": "service",
					  "destination_namespace": "default",
					  "destination_name": "bar1",
					  "datacenter": "dc1",
					  "local_bind_address": "127.0.0.2",
					  "local_bind_port": 6060,
					  "config": {
						"x": "y",
						"z": -2
					  },
					  "mesh_gateway": {
						"mode": "local"
					  }
					},
					{
					  "destination_type": "service",
					  "destination_namespace": "default",
					  "destination_name": "bar2",
					  "datacenter": "dc2",
					  "local_bind_address": "127.0.0.2",
					  "local_bind_port": 6161
					}
				  ],
				  "mesh_gateway": {
					"mode": "remote"
				  },
				  "expose": {
					"checks": true,
					"paths": [
					  {
						"listener_port": 8080,
						"path": "/foo",
						"local_path_port": 7070,
						"protocol": "http2",
						"parsed_from_check": true
					  },
					  {
						"listener_port": 8181,
						"path": "/foo2",
						"local_path_port": 7171,
						"protocol": "http",
						"parsed_from_check": false
					  }
					]
				  }
				}
			`,
			want: ConnectProxyConfig{
				DestinationServiceName: "foo-name",
				DestinationServiceID:   "foo-id",
				LocalServiceAddress:    "127.0.0.1",
				LocalServicePort:       5050,
				Config: map[string]interface{}{
					"a": "b",
					"v": float64(42),
				},
				Upstreams: []Upstream{
					{
						DestinationType:      UpstreamDestTypeService,
						DestinationNamespace: "default",
						DestinationName:      "bar1",
						Datacenter:           "dc1",
						LocalBindAddress:     "127.0.0.2",
						LocalBindPort:        6060,
						Config: map[string]interface{}{
							"x": "y",
							"z": float64(-2),
						},
						MeshGateway: MeshGatewayConfig{
							Mode: "local",
						},
					},

					{
						DestinationType:      UpstreamDestTypeService,
						DestinationNamespace: "default",
						DestinationName:      "bar2",
						Datacenter:           "dc2",
						LocalBindAddress:     "127.0.0.2",
						LocalBindPort:        6161,
					},
				},

				MeshGateway: MeshGatewayConfig{
					Mode: "remote",
				},
				Expose: ExposeConfig{
					Checks: true,
					Paths: []ExposePath{
						{
							ListenerPort:    8080,
							Path:            "/foo",
							LocalPathPort:   7070,
							Protocol:        "http2",
							ParsedFromCheck: true,
						},
						{
							ListenerPort:    8181,
							Path:            "/foo2",
							LocalPathPort:   7171,
							Protocol:        "http",
							ParsedFromCheck: false,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("camel", func(t *testing.T) {
				//
				var got ConnectProxyConfig
				err := json.Unmarshal([]byte(tt.json), &got)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, tt.want, got)
				}
			})
			if tt.jsonSnake != "" {
				t.Run("snake", func(t *testing.T) {
					//
					var got ConnectProxyConfig
					err := json.Unmarshal([]byte(tt.json), &got)
					if tt.wantErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.Equal(t, tt.want, got)
					}
				})
			}
		})
	}
}

func TestValidateMeshGatewayMode(t *testing.T) {
	for _, tc := range []struct {
		modeConstant string
		modeExplicit string
		expect       MeshGatewayMode
		ok           bool
	}{
		{string(MeshGatewayModeNone), "none", MeshGatewayModeNone, true},
		{string(MeshGatewayModeDefault), "", MeshGatewayModeDefault, true},
		{string(MeshGatewayModeLocal), "local", MeshGatewayModeLocal, true},
		{string(MeshGatewayModeRemote), "remote", MeshGatewayModeRemote, true},
	} {
		tc := tc

		t.Run(tc.modeConstant+" (constant)", func(t *testing.T) {
			got, err := ValidateMeshGatewayMode(tc.modeConstant)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			} else {
				require.Error(t, err)
			}
		})
		t.Run(tc.modeExplicit+" (explicit)", func(t *testing.T) {
			got, err := ValidateMeshGatewayMode(tc.modeExplicit)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateProxyMode(t *testing.T) {
	for _, tc := range []struct {
		modeConstant string
		modeExplicit string
		expect       ProxyMode
		ok           bool
	}{
		{string(ProxyModeDefault), "", ProxyModeDefault, true},
		{string(ProxyModeDirect), "direct", ProxyModeDirect, true},
		{string(ProxyModeTransparent), "transparent", ProxyModeTransparent, true},
	} {
		tc := tc

		t.Run(tc.modeConstant+" (constant)", func(t *testing.T) {
			got, err := ValidateProxyMode(tc.modeConstant)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			} else {
				require.Error(t, err)
			}
		})
		t.Run(tc.modeExplicit+" (explicit)", func(t *testing.T) {
			got, err := ValidateProxyMode(tc.modeExplicit)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}
