package proxy

import (
	"net"
	"testing"

	"github.com/hashicorp/consul/connect"
	"github.com/stretchr/testify/require"
)

func TestUpstream(t *testing.T) {
	tests := []struct {
		name string
		cfg  UpstreamConfig
	}{
		{
			name: "service",
			cfg: UpstreamConfig{
				DestinationType:      "service",
				DestinationNamespace: "default",
				DestinationName:      "db",
				ConnectTimeoutMs:     100,
			},
		},
		{
			name: "prepared_query",
			cfg: UpstreamConfig{
				DestinationType:      "prepared_query",
				DestinationNamespace: "default",
				DestinationName:      "geo-db",
				ConnectTimeoutMs:     100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := TestLocalBindAddrs(t, 2)

			testApp, err := NewTestTCPServer(t, addrs[0])
			require.Nil(t, err)
			defer testApp.Close()

			// Create mock client that will "discover" our test tcp server as a target and
			// skip TLS altogether.
			client := &TestConnectClient{
				Server:    testApp,
				TLSConfig: connect.TestTLSConfig(t, "ca1", "web"),
			}

			// Override cfg params
			tt.cfg.LocalBindAddress = addrs[1]
			tt.cfg.Client = client

			u := NewUpstream(tt.cfg)

			// Run proxy
			r := NewRunner("test", u)
			go r.Listen()
			defer r.Stop()

			// Proxy and fake remote service are running, play the part of the app
			// connecting to a remote connect service over TCP.
			conn, err := net.Dial("tcp", tt.cfg.LocalBindAddress)
			require.Nil(t, err)
			TestEchoConn(t, conn, "")

			// Validate that discovery actually was called as we expected
			require.Len(t, client.Calls, 1)
			require.Equal(t, tt.cfg.DestinationType, client.Calls[0].typ)
			require.Equal(t, tt.cfg.DestinationNamespace, client.Calls[0].ns)
			require.Equal(t, tt.cfg.DestinationName, client.Calls[0].name)
		})
	}
}
