package proxy

import (
	"context"
	"net"
	"testing"

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

	require := require.New(t)

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

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
	}), testutil.Logger(t))
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
