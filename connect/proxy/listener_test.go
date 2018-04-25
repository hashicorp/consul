package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib/freeport"
	"github.com/stretchr/testify/require"
)

func TestPublicListener(t *testing.T) {
	ca := agConnect.TestCA(t, nil)
	ports := freeport.GetT(t, 2)

	cfg := PublicListenerConfig{
		BindAddress:           "127.0.0.1",
		BindPort:              ports[0],
		LocalServiceAddress:   TestLocalAddr(ports[1]),
		HandshakeTimeoutMs:    100,
		LocalConnectTimeoutMs: 100,
	}

	testApp, err := NewTestTCPServer(t, cfg.LocalServiceAddress)
	require.NoError(t, err)
	defer testApp.Close()

	svc := connect.TestService(t, "db", ca)

	l := NewPublicListener(svc, cfg, log.New(os.Stderr, "", log.LstdFlags))

	// Run proxy
	go func() {
		err := l.Serve()
		require.NoError(t, err)
	}()
	defer l.Close()
	l.Wait()

	// Proxy and backend are running, play the part of a TLS client using same
	// cert for now.
	conn, err := svc.Dial(context.Background(), &connect.StaticResolver{
		Addr:    TestLocalAddr(ports[0]),
		CertURI: agConnect.TestSpiffeIDService(t, "db"),
	})
	require.NoError(t, err)
	TestEchoConn(t, conn, "")
}

func TestUpstreamListener(t *testing.T) {
	ca := agConnect.TestCA(t, nil)
	ports := freeport.GetT(t, 1)

	// Run a test server that we can dial.
	testSvr := connect.NewTestServer(t, "db", ca)
	go func() {
		err := testSvr.Serve()
		require.NoError(t, err)
	}()
	defer testSvr.Close()
	<-testSvr.Listening

	cfg := UpstreamConfig{
		DestinationType:      "service",
		DestinationNamespace: "default",
		DestinationName:      "db",
		ConnectTimeoutMs:     100,
		LocalBindAddress:     "localhost",
		LocalBindPort:        ports[0],
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
		require.NoError(t, err)
	}()
	defer l.Close()
	l.Wait()

	// Proxy and fake remote service are running, play the part of the app
	// connecting to a remote connect service over TCP.
	conn, err := net.Dial("tcp",
		fmt.Sprintf("%s:%d", cfg.LocalBindAddress, cfg.LocalBindPort))
	require.NoError(t, err)
	TestEchoConn(t, conn, "")
}
