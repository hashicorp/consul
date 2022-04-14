package consul

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
	"github.com/hashicorp/consul/testrpc"
)

func TestGRPCIntegration_ConnectCA_Sign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// The gRPC endpoint itself well-tested with mocks. This test checks we're
	// correctly wiring everything up in the server by:
	//
	//	* Starting a cluster with multiple servers.
	//	* Making a request to a follower's public gRPC port.
	//	* Ensuring that the request is correctly forwarded to the leader.
	//	* Ensuring we get a valid certificate back (so it went through the CAManager).
	dir1, server1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 2
	})
	defer os.RemoveAll(dir1)
	defer server1.Shutdown()

	dir2, server2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})
	defer os.RemoveAll(dir2)
	defer server2.Shutdown()

	joinLAN(t, server2, server1)

	testrpc.WaitForLeader(t, server1.RPC, "dc1")

	var follower *Server
	if server1.IsLeader() {
		follower = server2
	} else {
		follower = server1
	}

	// publicGRPCServer is bound to a listener by the wrapping agent code, so we
	// need to do it ourselves here.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		require.NoError(t, follower.publicGRPCServer.Serve(lis))
	}()
	t.Cleanup(follower.publicGRPCServer.Stop)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client := pbconnectca.NewConnectCAServiceClient(conn)

	csr, _ := connect.TestCSR(t, &connect.SpiffeIDService{
		Host:       connect.TestClusterID + ".consul",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	})

	// This would fail if it wasn't forwarded to the leader.
	rsp, err := client.Sign(context.Background(), &pbconnectca.SignRequest{
		Csr: csr,
	})
	require.NoError(t, err)

	_, err = connect.ParseCert(rsp.CertPem)
	require.NoError(t, err)
}
