//go:build !consulent
// +build !consulent

package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/testrpc"
)

func TestPeeringBackend_RejectsPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// make a grpc client to dial s1 directly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s1.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		Partition: "test",
	}
	_, err = peeringClient.GenerateToken(ctx, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Partitions are a Consul Enterprise feature")
}

func TestPeeringBackend_IgnoresDefaultPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// make a grpc client to dial s1 directly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s1.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	req := pbpeering.GenerateTokenRequest{
		PeerName:  "my-peer",
		Partition: "DeFaUlT",
	}
	_, err = peeringClient.GenerateToken(ctx, &req)
	require.NoError(t, err)
}
