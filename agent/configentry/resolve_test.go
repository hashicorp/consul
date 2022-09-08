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
