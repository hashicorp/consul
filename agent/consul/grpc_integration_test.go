package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/grpc/public"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
)

func TestGRPCIntegration_ConnectCA_Sign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// The gRPC endpoint itself well-tested with mocks. This test checks we're
	// correctly wiring everything up in the server by:
	//
	//	* Starting a cluster with multiple servers.
	//	* Making a request to a follower's public gRPC port.
	//	* Ensuring that the request is correctly forwarded to the leader.
	//	* Ensuring we get a valid certificate back (so it went through the CAManager).
	server1, conn1 := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 2
	})

	server2, conn2 := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = false
	})

	joinLAN(t, server2, server1)

	waitForLeaderEstablishment(t, server1, server2)

	conn := conn2
	if server2.IsLeader() {
		conn = conn1
	}

	client := pbconnectca.NewConnectCAServiceClient(conn)

	csr, _ := connect.TestCSR(t, &connect.SpiffeIDService{
		Host:       connect.TestClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	ctx = public.ContextWithToken(ctx, TestDefaultInitialManagementToken)

	// This would fail if it wasn't forwarded to the leader.
	rsp, err := client.Sign(ctx, &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.NoError(t, err)

	_, err = connect.ParseCert(rsp.CertPem)
	require.NoError(t, err)
}

func TestGRPCIntegration_ServerDiscovery_WatchServers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// The gRPC endpoint itself well-tested with mocks. This test checks we're
	// correctly wiring everything up in the server by:
	//
	//	* Starting a server
	// * Initiating the gRPC stream
	// * Validating the snapshot
	// * Adding another server
	// * Validating another message is sent.

	server1, conn := testGRPCIntegrationServer(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
	})
	waitForLeaderEstablishment(t, server1)

	client := pbserverdiscovery.NewServerDiscoveryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	ctx = public.ContextWithToken(ctx, TestDefaultInitialManagementToken)

	serverStream, err := client.WatchServers(ctx, &pbserverdiscovery.WatchServersRequest{Wan: false})
	require.NoError(t, err)

	rsp, err := serverStream.Recv()
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.Len(t, rsp.Servers, 1)

	_, server2, _ := testACLServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	}, false)

	// join the new server to the leader
	joinLAN(t, server2, server1)

	// now receive the event containing 2 servers
	rsp, err = serverStream.Recv()
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.Len(t, rsp.Servers, 2)
}
