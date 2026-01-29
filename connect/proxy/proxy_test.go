// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
			CertURI: agConnect.TestSpiffeIDService(r, "echo"),
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

func TestPublicListenerConfig_Defaults(t *testing.T) {
	testCases := []struct {
		name     string
		config   PublicListenerConfig
		expected PublicListenerConfig
	}{
		{
			name:   "empty config gets defaults",
			config: PublicListenerConfig{},
			expected: PublicListenerConfig{
				LocalConnectTimeoutMs: 1000,
				HandshakeTimeoutMs:    10000,
				BindAddress:           "0.0.0.0",
			},
		},
		{
			name: "partial config preserves values",
			config: PublicListenerConfig{
				LocalConnectTimeoutMs: 5000,
				BindAddress:           "127.0.0.1",
			},
			expected: PublicListenerConfig{
				LocalConnectTimeoutMs: 5000,
				HandshakeTimeoutMs:    10000,
				BindAddress:           "127.0.0.1",
			},
		},
		{
			name: "full config unchanged",
			config: PublicListenerConfig{
				LocalConnectTimeoutMs: 2000,
				HandshakeTimeoutMs:    15000,
				BindAddress:           "192.168.1.100",
			},
			expected: PublicListenerConfig{
				LocalConnectTimeoutMs: 2000,
				HandshakeTimeoutMs:    15000,
				BindAddress:           "192.168.1.100",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.applyDefaults()
			require.Equal(t, tc.expected, tc.config)
		})
	}
}

func TestUpstreamConfig_ConnectTimeout(t *testing.T) {
	testCases := []struct {
		name     string
		config   map[string]interface{}
		expected time.Duration
	}{
		{
			name:     "no config uses default",
			config:   map[string]interface{}{},
			expected: 10000 * time.Millisecond, // 10s default
		},
		{
			name:     "nil config uses default",
			config:   nil,
			expected: 10000 * time.Millisecond,
		},
		{
			name: "custom timeout",
			config: map[string]interface{}{
				"connect_timeout_ms": 5000,
			},
			expected: 5000 * time.Millisecond,
		},
		{
			name: "zero timeout",
			config: map[string]interface{}{
				"connect_timeout_ms": 0,
			},
			expected: 0,
		},
		{
			name: "very large timeout",
			config: map[string]interface{}{
				"connect_timeout_ms": 300000, // 5 minutes
			},
			expected: 300000 * time.Millisecond,
		},
		{
			name: "invalid type falls back to default",
			config: map[string]interface{}{
				"connect_timeout_ms": "invalid",
			},
			expected: 10000 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &UpstreamConfig{
				Config: tc.config,
			}
			result := upstream.ConnectTimeout()
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestUpstreamConfig_Defaults(t *testing.T) {
	testCases := []struct {
		name     string
		config   UpstreamConfig
		expected UpstreamConfig
	}{
		{
			name:   "empty config gets defaults",
			config: UpstreamConfig{},
			expected: UpstreamConfig{
				DestinationType:      "service",
				DestinationNamespace: "default",
				DestinationPartition: "default",
				LocalBindAddress:     "127.0.0.1",
			},
		},
		{
			name: "partial config preserves values",
			config: UpstreamConfig{
				DestinationName:      "web",
				DestinationNamespace: "prod",
				LocalBindPort:        8080,
			},
			expected: UpstreamConfig{
				DestinationName:      "web",
				DestinationType:      "service",
				DestinationNamespace: "prod",
				DestinationPartition: "default",
				LocalBindAddress:     "127.0.0.1",
				LocalBindPort:        8080,
			},
		},
		{
			name: "socket path overrides bind address default",
			config: UpstreamConfig{
				LocalBindSocketPath: "/var/run/socket.sock",
			},
			expected: UpstreamConfig{
				DestinationType:      "service",
				DestinationNamespace: "default",
				DestinationPartition: "default",
				LocalBindSocketPath:  "/var/run/socket.sock",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.applyDefaults()
			require.Equal(t, tc.expected, tc.config)
		})
	}
}
