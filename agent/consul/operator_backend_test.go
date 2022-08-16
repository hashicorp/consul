package consul

import (
	"context"
	"github.com/hashicorp/consul/proto/pboperator"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestOperatorBackend_ForwardToLeader(t *testing.T) {
	t.Parallel()

	conf := testClusterConfig{
		Datacenter: "dc1",
		Servers:    3,
		ServerConf: func(config *Config) {
			config.RaftConfig.HeartbeatTimeout = 2 * time.Second
			config.RaftConfig.ElectionTimeout = 2 * time.Second
			config.RaftConfig.LeaderLeaseTimeout = 1 * time.Second
		},
	}

	nodes := newTestCluster(t, &conf)
	s1 := nodes.Servers[0]
	// Make sure a leader is elected
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Make a write call to server2 and make sure it gets forwarded to server1
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Dial server2 directly
	conn, err := gogrpc.DialContext(ctx, s1.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(s1.config.RPCAddr.String())),
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	operatorClient := pboperator.NewOperatorServiceClient(conn)

	testutil.RunStep(t, "forward a write", func(t *testing.T) {
		beforeLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, beforeLeader)
		// Do the grpc Write call to server2
		req := pboperator.TransferLeaderRequest{
			ID: "",
		}
		_, err := operatorClient.TransferLeader(ctx, &req)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		retry.Run(t, func(r *retry.R) {
			afterLeader, _ := s1.raft.LeaderWithID()
			require.NotEmpty(r, afterLeader)
		})
		afterLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, afterLeader)
		if afterLeader == beforeLeader {
			t.Fatalf("leader should have changed %s == %s", afterLeader, beforeLeader)
		}
	})
}
