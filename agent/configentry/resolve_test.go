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
		scReq       *structs.ServiceConfigRequest
		upstreamIDs []structs.ServiceID
		entries     *ResolvedServiceConfigSet
	}

	sid := structs.ServiceID{
		ID:             "sid",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	uid := structs.ServiceID{
		ID:             "upstream1",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	uids := []structs.ServiceID{uid}
	wildcard := structs.NewServiceID(structs.WildcardSpecifier, acl.WildcardEnterpriseMeta())

	localMeshGW := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal}
	remoteMeshGW := structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote}

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
					Name:        "sid",
					UpstreamIDs: uids,
				},
				upstreamIDs: uids,
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
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
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
			name: "proxy upstream mesh-gateway inherits service-defaults defaults",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:        "sid",
					UpstreamIDs: uids,
				},
				upstreamIDs: uids,
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW, // applied 1st
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: remoteMeshGW, // applied 2nd
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: localMeshGW, // This is not affected by the UpstreamConfigs.
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
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
			name: "proxy upstream mesh-gateway inherits value from node-service",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:        "sid",
					UpstreamIDs: uids,
					MeshGateway: remoteMeshGW, // applied 3rd
				},
				upstreamIDs: uids,
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW, // applied 1st
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: localMeshGW, // applied 2nd
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: localMeshGW, // This is not affected by the UpstreamConfigs.
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
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
			name: "proxy upstream mesh-gateway inherits value from service-defaults override",
			args: args{
				scReq: &structs.ServiceConfigRequest{
					Name:        "sid",
					UpstreamIDs: uids,
					MeshGateway: localMeshGW, // applied 3rd
				},
				upstreamIDs: uids,
				entries: &ResolvedServiceConfigSet{
					ProxyDefaults: map[string]*structs.ProxyConfigEntry{
						acl.DefaultEnterpriseMeta().PartitionOrDefault(): {
							MeshGateway: localMeshGW, // applied 1st
						},
					},
					ServiceDefaults: map[structs.ServiceID]*structs.ServiceConfigEntry{
						sid: {
							UpstreamConfig: &structs.UpstreamConfiguration{
								Defaults: &structs.UpstreamConfig{
									MeshGateway: localMeshGW, // applied 2nd
								},
								Overrides: []*structs.UpstreamConfig{
									{
										Name:        uid.ID,
										MeshGateway: remoteMeshGW, // applied 4th
									},
								},
							},
						},
					},
				},
			},
			want: &structs.ServiceConfigResponse{
				MeshGateway: localMeshGW, // This is not affected by the UpstreamConfigs.
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": localMeshGW, // wildcard not affected, since it wasn't in the overrides.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeResolvedServiceConfig(tt.args.scReq, tt.args.upstreamIDs,
				false, tt.args.entries, nil)
			require.NoError(t, err)
			// This is needed because map iteration is random and determines the order of some outputs.
			sort.Slice(got.UpstreamIDConfigs, func(i, j int) bool {
				return got.UpstreamIDConfigs[i].Upstream.ID < got.UpstreamIDConfigs[j].Upstream.ID
			})
			require.Equal(t, tt.want, got)
		})
	}
}
