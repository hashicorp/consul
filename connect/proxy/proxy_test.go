// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestProxy_public(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ports := freeport.GetN(t, 2)

	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	client := a.Client()

	// Register the service so we can get a leaf cert
	_, err := client.Catalog().Register(&api.CatalogRegistration{
		Datacenter: "dc1",
		Node:       "local",
		Address:    "127.0.0.1",
		Service: &api.AgentService{
			Service: "echo",
		},
	}, nil)
	require.NoError(t, err)

	// Start the backend service that is being proxied
	testApp := NewTestTCPServer(t)
	defer testApp.Close()

	upstreams := []UpstreamConfig{
		{
			DestinationName: "just-a-port",
			LocalBindPort:   ports[1],
		},
	}

	var unixSocket string
	if runtime.GOOS != "windows" {
		tempDir := testutil.TempDir(t, "consul")
		unixSocket = filepath.Join(tempDir, "test.sock")

		upstreams = append(upstreams, UpstreamConfig{
			DestinationName:     "just-a-unix-domain-socket",
			LocalBindSocketPath: unixSocket,
			LocalBindSocketMode: "0600",
		})
	}

	// Start the proxy
	p, err := New(client, NewStaticConfigWatcher(&Config{
		ProxiedServiceName: "echo",
		PublicListener: PublicListenerConfig{
			BindAddress:         "127.0.0.1",
			BindPort:            ports[0],
			LocalServiceAddress: testApp.Addr().String(),
		},
		Upstreams: upstreams,
	}), testutil.Logger(t))
	require.NoError(t, err)
	defer p.Close()
	go p.Serve()

	// We create this client with an explicit ServerNextProtos here which will use `h2`
	// if the proxy supports it. This is so we can verify below that the proxy _doesn't_
	// advertise `h2` support as it's only a L4 proxy.
	svc, err := connect.NewServiceWithConfig("echo", connect.Config{Client: client, ServerNextProtos: []string{"h2"}})
	require.NoError(t, err)

	// Create a test connection to the proxy. We retry here a few times
	// since this is dependent on the agent actually starting up and setting
	// up the CA.
	var conn net.Conn
	retry.Run(t, func(r *retry.R) {
		conn, err = svc.Dial(context.Background(), &connect.StaticResolver{
			Addr:    TestLocalAddr(ports[0]),
			CertURI: agConnect.TestSpiffeIDService(t, "echo"),
		})
		if err != nil {
			r.Fatalf("err: %s", err)
		}
	})

	// Verify that we did not select h2 via ALPN since the proxy is layer 4 only
	tlsConn := conn.(*tls.Conn)
	require.Equal(t, "", tlsConn.ConnectionState().NegotiatedProtocol)

	// Connection works, test it is the right one
	TestEchoConn(t, conn, "")

	t.Run("verify port upstream is configured", func(t *testing.T) {
		// Verify that it is listening by doing a simple TCP dial.
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(ports[1]))
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		require.NoError(t, err)
		_ = conn.Close()
	})

	t.Run("verify unix domain socket upstream will never work", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.SkipNow()
		}

		// Ensure the socket was not created
		require.NoFileExists(t, unixSocket)
	})
}
