package consul

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestGRPCResolver_Rebalance(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, server1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server1.Shutdown()

	dir2, server2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer server2.Shutdown()

	dir3, server3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir3)
	defer server3.Shutdown()

	dir4, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir4)
	defer client.Shutdown()

	// Try to join
	joinLAN(t, server2, server1)
	joinLAN(t, server3, server1)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	joinLAN(t, client, server2)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	// Make a call to our test endpoint.
	conn, err := client.GRPCConn()
	require.NoError(err)

	grpcClient := agentpb.NewTestClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response1, err := grpcClient.Test(ctx, &agentpb.TestRequest{})
	require.NoError(err)

	// Rebalance a few times to hit a different server.
	for {
		select {
		case <-ctx.Done():
			t.Fatal("could not get a response from a different server")
		default:
		}

		// Force a shuffle and wait for the connection to be rebalanced.
		client.grpcResolverBuilder.rebalanceResolvers()
		time.Sleep(100 * time.Millisecond)

		response2, err := grpcClient.Test(ctx, &agentpb.TestRequest{})
		require.NoError(err)
		if response1.ServerName == response2.ServerName {
			break
		}
	}
}

func TestGRPCResolver_Failover_LocalDC(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, server1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server1.Shutdown()

	dir2, server2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer server2.Shutdown()

	dir3, server3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir3)
	defer server3.Shutdown()

	dir4, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir4)
	defer client.Shutdown()

	// Try to join
	joinLAN(t, server2, server1)
	joinLAN(t, server3, server1)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	joinLAN(t, client, server2)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	// Make a call to our test endpoint.
	conn, err := client.GRPCConn()
	require.NoError(err)

	grpcClient := agentpb.NewTestClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response1, err := grpcClient.Test(ctx, &agentpb.TestRequest{})
	require.NoError(err)

	// Shutdown the server that answered the request.
	var shutdown *Server
	for _, s := range []*Server{server1, server2, server3} {
		if s.config.NodeName == response1.ServerName {
			shutdown = s
			break
		}
	}
	require.NotNil(shutdown)
	require.NoError(shutdown.Shutdown())

	// Wait for the balancer to switch over to another server so we get a different response.
	retry.Run(t, func(r *retry.R) {
		response2, err := grpcClient.Test(ctx, &agentpb.TestRequest{})
		r.Check(err)
		if response1.ServerName == response2.ServerName {
			r.Fatal("responses should be from different servers")
		}
	})
}

func TestGRPCResolver_Failover_MultiDC(t *testing.T) {
	t.Parallel()

	// Create a single server in dc1.
	require := require.New(t)
	dir1, server1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer server1.Shutdown()

	// Create a client in dc1.
	cDir, client := testClientWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.NodeName = uniqueNodeName(t.Name())
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(cDir)
	defer client.Shutdown()

	// Create 3 servers in dc2.
	dir2, server2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer server2.Shutdown()

	dir3, server3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir3)
	defer server3.Shutdown()

	dir4, server4 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.GRPCEnabled = true
		c.GRPCTestServerEnabled = true
		c.GRPCTestServerEnabled = true
	})
	defer os.RemoveAll(dir4)
	defer server4.Shutdown()

	// Try to join
	joinLAN(t, server3, server2)
	joinLAN(t, server4, server2)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	testrpc.WaitForLeader(t, server2.RPC, "dc2")

	joinWAN(t, server1, server2)
	joinWAN(t, server3, server2)
	joinWAN(t, server4, server2)
	joinLAN(t, client, server1)
	testrpc.WaitForTestAgent(t, client.RPC, "dc1")

	// Make a call to our test endpoint on the client in dc1.
	conn, err := client.GRPCConn()
	require.NoError(err)

	grpcClient := agentpb.NewTestClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response1, err := grpcClient.Test(ctx, &agentpb.TestRequest{Datacenter: "dc2"})
	require.NoError(err)
	// Make sure the response didn't come from dc1.
	require.Contains([]string{
		server2.config.NodeName,
		server3.config.NodeName,
		server4.config.NodeName,
	}, response1.ServerName)

	// Shutdown the server that answered the request.
	var shutdown *Server
	for _, s := range []*Server{server2, server3, server4} {
		if s.config.NodeName == response1.ServerName {
			shutdown = s
			break
		}
	}
	require.NotNil(shutdown)
	require.NoError(shutdown.Leave())
	require.NoError(shutdown.Shutdown())

	// Wait for the balancer to switch over to another server so we get a different response.
	retry.Run(t, func(r *retry.R) {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response2, err := grpcClient.Test(ctx, &agentpb.TestRequest{Datacenter: "dc2"})
		r.Check(err)
		if response1.ServerName == response2.ServerName {
			r.Fatal("responses should be from different servers")
		}
	})
}
