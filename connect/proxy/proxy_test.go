package proxy

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent"
	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib/freeport"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestProxy_public(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ports := freeport.GetT(t, 1)

	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()
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
	require.NoError(err)

	// Start the backend service that is being proxied
	testApp := NewTestTCPServer(t)
	defer testApp.Close()

	// Start the proxy
	p, err := New(client, NewStaticConfigWatcher(&Config{
		ProxiedServiceName: "echo",
		PublicListener: PublicListenerConfig{
			BindAddress:         "127.0.0.1",
			BindPort:            ports[0],
			LocalServiceAddress: testApp.Addr().String(),
		},
	}), testLogger(t))
	require.NoError(err)
	defer p.Close()
	go p.Serve()

	// Create a test connection to the proxy. We retry here a few times
	// since this is dependent on the agent actually starting up and setting
	// up the CA.
	var conn net.Conn
	svc, err := connect.NewService("echo", client)
	require.NoError(err)
	retry.Run(t, func(r *retry.R) {
		conn, err = svc.Dial(context.Background(), &connect.StaticResolver{
			Addr:    TestLocalAddr(ports[0]),
			CertURI: agConnect.TestSpiffeIDService(t, "echo"),
		})
		if err != nil {
			r.Fatalf("err: %s", err)
		}
	})

	// Connection works, test it is the right one
	TestEchoConn(t, conn, "")
}

func testLogger(t *testing.T) *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}
