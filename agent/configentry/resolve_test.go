package configentry

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func Test_ComputeResolvedServiceConfig(t *testing.T) {
	type args struct {
		scReq   *structs.ServiceConfigRequest
		entries *ResolvedServiceConfigSet
	}

	sid := structs.ServiceID{
		ID:             "sid",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	uid := structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("upstream1", acl.DefaultEnterpriseMeta()),
	}

	uids := []structs.PeeredServiceName{uid}

	wildcard := structs.PeeredServiceName{
		ServiceName: structs.NewServiceName(structs.WildcardSpecifier, acl.WildcardEnterpriseMeta()),
	}

	localMeshGW := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal}
	remoteMeshGW := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote}
	noneMeshGW := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeNone}

	tests := []struct {
		name string
		args args
		want *structs.ServiceConfigResponse
	}{
		{
			name: "proxy with balanceinboundconnections",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							BalanceInboundConnections: "exact_balance",
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"balance_inbound_connections": "exact_balance",
				},
			},
		},
		{
			name: "proxy with maxinboundsconnections",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &ResolvedServiceConfigSet{
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
				entries: &ResolvedServiceConfigSet{
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
		{
			name: "proxy upstream mesh-gateway inherits proxy-defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: remoteMeshGW, // applied 1st
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: remoteMeshGW,
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": structs.MeshGatewayConfig{
								Mode: structs.MeshGatewayModeRemote,
							},
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							"mesh_gateway": remoteMeshGW,
						},
					},
				},
			},
		},
		{
			name: "proxy inherits kitchen sink from proxy-defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							Config: map[string]interface{}{
								"foo": "bar",
							},
							Expose: structs.ExposeConfig{
								Checks: true,
								Paths:  []structs.ExposePath{},
							},
							Mode:        structs.ProxyModeTransparent,
							MeshGateway: remoteMeshGW,
							TransparentProxy: structs.TransparentProxyConfig{
								OutboundListenerPort: 6666,
								DialedDirectly:       true,
							},
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
			},
			want: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"foo": "bar",
				},
				Expose: structs.ExposeConfig{
					Checks: true,
					Paths:  []structs.ExposePath{},
				},
				Mode:        structs.ProxyModeTransparent,
				MeshGateway: remoteMeshGW,
				TransparentProxy: structs.TransparentProxyConfig{
					OutboundListenerPort: 6666,
					DialedDirectly:       true,
				},
				AccessLogs: structs.AccessLogsConfig{
					Enabled:             true,
					DisableListenerLogs: true,
					Type:                structs.FileLogSinkType,
					Path:                "/tmp/accesslog.txt",
					JSONFormat:          "{ \"custom_start_time\": \"%START_TIME%\" }",
				},
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": remoteMeshGW,
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							"mesh_gateway": remoteMeshGW,
						},
					},
				},
			},
		},
		{
			name: "proxy upstream mesh-gateway inherits service-defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW, // applied 1st
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							MeshGateway: noneMeshGW, // applied 2nd
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: noneMeshGW, // service-defaults has a higher precedence.
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": noneMeshGW,
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							"mesh_gateway": noneMeshGW,
						},
					},
				},
			},
		},
		{
			name: "proxy wildcard upstream mesh-gateway inherits proxy-defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
					Mode: structs.ProxyModeTransparent,
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW,
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									Protocol: "http",
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: localMeshGW,
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": localMeshGW, // From proxy-defaults
							"protocol":     "http",
						},
					},
				},
			},
		},
		{
			name: "proxy upstream mesh-gateway inherits upstream defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW,
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							MeshGateway: noneMeshGW,
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: remoteMeshGW,
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: noneMeshGW, // Merged from proxy-defaults + service-defaults
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							// Wildcard stores the values from UpstreamConfig.Defaults directly
							"mesh_gateway": remoteMeshGW,
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							// Upstream-specific config comes from UpstreamConfig.Defaults
							"mesh_gateway": remoteMeshGW,
						},
					},
				},
			},
		},
		{
			name: "proxy upstream mesh-gateway inherits value from node-service",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,

					// MeshGateway from NodeService is received in the request
					MeshGateway: remoteMeshGW,
				},
				entries: &ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: noneMeshGW,
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							// NodeService.Proxy.MeshGateway has a higher precedence than centralized
							// UpstreamConfig.Defaults, since it's specific to a service instance.
							"mesh_gateway": remoteMeshGW,
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							"mesh_gateway": remoteMeshGW,
						},
					},
				},
			},
		},
		{
			name: "proxy upstream mesh-gateway inherits value from service-defaults override",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:                 "sid",
					UpstreamServiceNames: uids,
					MeshGateway:          localMeshGW, // applied 2nd
				},
				entries: &ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: localMeshGW, // applied 1st
								},
								Overrides: []*structs.UpstreamConfig{
									{
										Name:        uid.ServiceName.Name,
										MeshGateway: remoteMeshGW, // applied 3rd
									},
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				UpstreamConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							// Wildcard stores the values from UpstreamConfig.Defaults directly
							"mesh_gateway": localMeshGW,
						},
					},
					{
						Upstream: uid,
						Config: map[string]interface{}{
							// UpstreamConfig.Overrides has a higher precedence than UpstreamConfig.Defaults
							"mesh_gateway": remoteMeshGW,
						},
					},
				},
			},
		},
		{
			name: "servicedefaults envoy extension",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &ResolvedServiceConfigSet{
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							EnvoyExtensions: []structs.EnvoyExtension{
								{
									Name:      "sd-ext",
									Required:  false,
									Arguments: map[string]interface{}{"arg": "val"},
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				EnvoyExtensions: []structs.EnvoyExtension{
					{
						Name:      "sd-ext",
						Required:  false,
						Arguments: map[string]interface{}{"arg": "val"},
					},
				},
			},
		},
		{
			name: "proxydefaults envoy extension appended to servicedefaults extension",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name: "sid",
				},
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							EnvoyExtensions: []structs.EnvoyExtension{
								{
									Name:      "pd-ext",
									Required:  false,
									Arguments: map[string]interface{}{"arg": "val"},
								},
							},
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							EnvoyExtensions: []structs.EnvoyExtension{
								{
									Name:      "sd-ext",
									Required:  false,
									Arguments: map[string]interface{}{"arg": "val"},
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				EnvoyExtensions: []structs.EnvoyExtension{
					{
						Name:      "pd-ext",
						Required:  false,
						Arguments: map[string]interface{}{"arg": "val"},
					},
					{
						Name:      "sd-ext",
						Required:  false,
						Arguments: map[string]interface{}{"arg": "val"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeResolvedServiceConfig(tt.args.scReq, tt.args.entries, nil)
			require.NoError(t, err)
			// This is needed because map iteration is random and determines the order of some outputs.
			sort.Slice(got.UpstreamConfigs, func(i, j int) bool {
				return got.UpstreamConfigs[i].Upstream.ServiceName.Name < got.UpstreamConfigs[j].Upstream.ServiceName.Name
			})
			require.Equal(t, tt.want, got)
		})
	}
}
