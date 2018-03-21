package proxy

import (
	"crypto/tls"
	"testing"

	"github.com/hashicorp/consul/connect"
	"github.com/stretchr/testify/require"
)

func TestPublicListener(t *testing.T) {
	addrs := TestLocalBindAddrs(t, 2)

	cfg := PublicListenerConfig{
		BindAddress:           addrs[0],
		LocalServiceAddress:   addrs[1],
		HandshakeTimeoutMs:    100,
		LocalConnectTimeoutMs: 100,
		TLSConfig:             connect.TestTLSConfig(t, "ca1", "web"),
	}

	testApp, err := NewTestTCPServer(t, cfg.LocalServiceAddress)
	require.Nil(t, err)
	defer testApp.Close()

	p := NewPublicListener(cfg)

	// Run proxy
	r := NewRunner("test", p)
	go r.Listen()
	defer r.Stop()

	// Proxy and backend are running, play the part of a TLS client using same
	// cert for now.
	conn, err := tls.Dial("tcp", cfg.BindAddress, connect.TestTLSConfig(t, "ca1", "web"))
	require.Nil(t, err)
	TestEchoConn(t, conn, "")
}
