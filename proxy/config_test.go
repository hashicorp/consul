package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfigFile(t *testing.T) {
	cfg, err := ParseConfigFile("testdata/config-kitchensink.hcl")
	require.Nil(t, err)

	expect := &Config{
		ProxyID:                 "foo",
		Token:                   "11111111-2222-3333-4444-555555555555",
		ProxiedServiceName:      "web",
		ProxiedServiceNamespace: "default",
		PublicListener: PublicListenerConfig{
			BindAddress:           ":9999",
			LocalServiceAddress:   "127.0.0.1:5000",
			LocalConnectTimeoutMs: 1000,
			HandshakeTimeoutMs:    5000,
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
