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
				Upstreams: Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc1",
						LocalBindPort:   1234,
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
				Upstreams: []api.Upstream{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc1",
						LocalBindPort:   1234,
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
