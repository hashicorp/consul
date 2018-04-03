package proxy

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/connect"
	"github.com/stretchr/testify/require"
)

func TestPublicListener(t *testing.T) {
	ca := agConnect.TestCA(t, nil)
	addrs := TestLocalBindAddrs(t, 2)

	cfg := PublicListenerConfig{
		BindAddress:           addrs[0],
		LocalServiceAddress:   addrs[1],
		HandshakeTimeoutMs:    100,
		LocalConnectTimeoutMs: 100,
	}

	testApp, err := NewTestTCPServer(t, cfg.LocalServiceAddress)
	require.Nil(t, err)
	defer testApp.Close()

	svc := connect.TestService(t, "db", ca)

	l := NewPublicListener(svc, cfg, log.New(os.Stderr, "", log.LstdFlags))

	// Run proxy
	go func() {
		err := l.Serve()
		require.Nil(t, err)
	}()
	defer l.Close()

	// Proxy and backend are running, play the part of a TLS client using same
	// cert for now.
	conn, err := svc.Dial(context.Background(), &connect.StaticResolver{
		Addr:    addrs[0],
		CertURI: agConnect.TestSpiffeIDService(t, "db"),
	})
	require.Nilf(t, err, "unexpected err: %s", err)
	TestEchoConn(t, conn, "")
}

func TestUpstreamListener(t *testing.T) {
	ca := agConnect.TestCA(t, nil)
	addrs := TestLocalBindAddrs(t, 1)

	// Run a test server that we can dial.
	testSvr := connect.NewTestServer(t, "db", ca)
	go func() {
		err := testSvr.Serve()
		require.Nil(t, err)
	}()
	defer testSvr.Close()

	cfg := UpstreamConfig{
		DestinationType:      "service",
		DestinationNamespace: "default",
		DestinationName:      "db",
		ConnectTimeoutMs:     100,
		LocalBindAddress:     addrs[0],
		resolver: &connect.StaticResolver{
			Addr:    testSvr.Addr,
			CertURI: agConnect.TestSpiffeIDService(t, "db"),
		},
	}

	svc := connect.TestService(t, "web", ca)

	l := NewUpstreamListener(svc, cfg, log.New(os.Stderr, "", log.LstdFlags))

	// Run proxy
	go func() {
		err := l.Serve()
		require.Nil(t, err)
	}()
	defer l.Close()

	// Proxy and fake remote service are running, play the part of the app
	// connecting to a remote connect service over TCP.
	conn, err := net.Dial("tcp", cfg.LocalBindAddress)
	require.Nilf(t, err, "unexpected err: %s", err)
	TestEchoConn(t, conn, "")
}
