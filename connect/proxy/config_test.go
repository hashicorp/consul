package proxy

import (
	"testing"

	"github.com/hashicorp/consul/connect"
	"github.com/stretchr/testify/require"
)

func TestParseConfigFile(t *testing.T) {
	cfg, err := ParseConfigFile("testdata/config-kitchensink.hcl")
	require.Nil(t, err)

	expect := &Config{
		ProxyID:                 "foo",
		Token:                   "11111111-2222-3333-4444-555555555555",
		ProxiedServiceID:        "web",
		ProxiedServiceNamespace: "default",
		PublicListener: PublicListenerConfig{
			BindAddress:           ":9999",
			LocalServiceAddress:   "127.0.0.1:5000",
			LocalConnectTimeoutMs: 1000,
			HandshakeTimeoutMs:    10000, // From defaults
		},
		Upstreams: []UpstreamConfig{
			{
				LocalBindAddress:     "127.0.0.1:6000",
				DestinationName:      "db",
				DestinationNamespace: "default",
				DestinationType:      "service",
				ConnectTimeoutMs:     10000,
			},
			{
				LocalBindAddress:     "127.0.0.1:6001",
				DestinationName:      "geo-cache",
				DestinationNamespace: "default",
				DestinationType:      "prepared_query",
				ConnectTimeoutMs:     10000,
			},
		},
		DevCAFile:          "connect/testdata/ca1-ca-consul-internal.cert.pem",
		DevServiceCertFile: "connect/testdata/ca1-svc-web.cert.pem",
		DevServiceKeyFile:  "connect/testdata/ca1-svc-web.key.pem",
	}

	require.Equal(t, expect, cfg)
}

func TestUpstreamResolverFromClient(t *testing.T) {
	tests := []struct {
		name string
		cfg  UpstreamConfig
		want *connect.ConsulResolver
	}{
		{
			name: "service",
			cfg: UpstreamConfig{
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "service",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypeService,
			},
		},
		{
			name: "prepared_query",
			cfg: UpstreamConfig{
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "prepared_query",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypePreparedQuery,
			},
		},
		{
			name: "unknown behaves like service",
			cfg: UpstreamConfig{
				DestinationNamespace:  "foo",
				DestinationName:       "web",
				DestinationDatacenter: "ny1",
				DestinationType:       "junk",
			},
			want: &connect.ConsulResolver{
				Namespace:  "foo",
				Name:       "web",
				Datacenter: "ny1",
				Type:       connect.ConsulResolverTypeService,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Client doesn't really matter as long as it's passed through.
			got := UpstreamResolverFromClient(nil, tt.cfg)
			require.Equal(t, tt.want, got)
		})
	}
}
