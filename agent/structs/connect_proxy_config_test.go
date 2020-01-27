package structs

import (
	"encoding/json"
	"fmt"
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.ToAPI())
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
			},
			want: `{
				"DestinationType": "service",
				"DestinationName": "foo",
				"Datacenter": "dc1",
				"LocalBindPort": 1234,
				"MeshGateway": {},
				"Config": null
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
				"MeshGateway": {},
				"Config": null
			}`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			got, err := json.Marshal(tt.in)
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)
			require.JSONEq(tt.want, string(got))
		})
	}
}

func TestUpstream_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    Upstream
		wantErr bool
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			var got Upstream
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)
			require.Equal(tt.want, got)
		})
	}
}

func TestMeshGatewayConfig_OverlayWith(t *testing.T) {
	var (
		D = MeshGatewayConfig{Mode: MeshGatewayModeDefault}
		N = MeshGatewayConfig{Mode: MeshGatewayModeNone}
		R = MeshGatewayConfig{Mode: MeshGatewayModeRemote}
		L = MeshGatewayConfig{Mode: MeshGatewayModeLocal}
	)

	type testCase struct {
		base, overlay, expect MeshGatewayConfig
	}
	cases := []testCase{
		{D, D, D},
		{D, N, N},
		{D, R, R},
		{D, L, L},
		{N, D, N},
		{N, N, N},
		{N, R, R},
		{N, L, L},
		{R, D, R},
		{R, N, N},
		{R, R, R},
		{R, L, L},
		{L, D, L},
		{L, N, N},
		{L, R, R},
		{L, L, L},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%s overlaid with %s", tc.base.Mode, tc.overlay.Mode),
			func(t *testing.T) {
				got := tc.base.OverlayWith(tc.overlay)
				require.Equal(t, tc.expect, got)
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
