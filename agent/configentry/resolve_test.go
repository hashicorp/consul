package configentry

import (
	"testing"

	"github.com/stretchr/testify/require"

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeResolvedServiceConfig(tt.args.scReq, tt.args.upstreamIDs,
				false, tt.args.entries, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
