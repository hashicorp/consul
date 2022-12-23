package consul

import (
	"context"
	"github.com/hashicorp/consul/acl"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pboperator"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestOperatorBackend_TransferLeader(t *testing.T) {
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
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	operatorClient := pboperator.NewOperatorServiceClient(conn)

	testutil.RunStep(t, "transfer leader", func(t *testing.T) {
		beforeLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, beforeLeader)
		// Do the grpc Write call to server2
		req := pboperator.TransferLeaderRequest{
			ID: "",
		}
		reply, err := operatorClient.TransferLeader(ctx, &req)
		require.NoError(t, err)
		require.True(t, reply.Success)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		retry.Run(t, func(r *retry.R) {
			time.Sleep(1 * time.Second)
			afterLeader, _ := s1.raft.LeaderWithID()
			require.NotEmpty(r, afterLeader)
			require.NotEqual(r, afterLeader, beforeLeader)
		})

	})
}

func TestOperatorBackend_TransferLeaderWithACL(t *testing.T) {
	t.Parallel()

	conf := testClusterConfig{
		Datacenter: "dc1",
		Servers:    3,
		ServerConf: func(config *Config) {
			config.RaftConfig.HeartbeatTimeout = 2 * time.Second
			config.RaftConfig.ElectionTimeout = 2 * time.Second
			config.RaftConfig.LeaderLeaseTimeout = 1 * time.Second
			config.ACLsEnabled = true
			config.ACLInitialManagementToken = "root"
			config.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	operatorClient := pboperator.NewOperatorServiceClient(conn)

	codec := rpcClient(t, s1)
	rules := `operator = "write"`
	tokenWrite := createTokenWithPolicyNameFull(t, codec, "the-policy-write", rules, "root")
	rules = `operator = "read"`
	tokenRead := createToken(t, codec, rules)
	require.NoError(t, err)

	testutil.RunStep(t, "transfer leader no token", func(t *testing.T) {
		beforeLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, beforeLeader)
		// Do the grpc Write call to server2
		req := pboperator.TransferLeaderRequest{
			ID: "",
		}
		reply, err := operatorClient.TransferLeader(ctx, &req)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Nil(t, reply)
		time.Sleep(1 * time.Second)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		retry.Run(t, func(r *retry.R) {
			time.Sleep(1 * time.Second)
			afterLeader, _ := s1.raft.LeaderWithID()
			require.NotEmpty(r, afterLeader)
			if afterLeader != beforeLeader {
				r.Fatalf("leader should have changed %s == %s", afterLeader, beforeLeader)
			}
		})

	})

	testutil.RunStep(t, "transfer leader operator read token", func(t *testing.T) {

		beforeLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, beforeLeader)
		// Do the grpc Write call to server2
		req := pboperator.TransferLeaderRequest{
			ID: "",
		}

		ctxToken, err := external.ContextWithQueryOptions(ctx, structs.QueryOptions{Token: tokenRead})
		require.NoError(t, err)

		reply, err := operatorClient.TransferLeader(ctxToken, &req)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Nil(t, reply)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		retry.Run(t, func(r *retry.R) {
			time.Sleep(1 * time.Second)
			afterLeader, _ := s1.raft.LeaderWithID()
			require.NotEmpty(r, afterLeader)
			if afterLeader != beforeLeader {
				r.Fatalf("leader should have changed %s == %s", afterLeader, beforeLeader)
			}
		})
	})

	testutil.RunStep(t, "transfer leader operator write token", func(t *testing.T) {

		beforeLeader, _ := s1.raft.LeaderWithID()
		require.NotEmpty(t, beforeLeader)
		// Do the grpc Write call to server2
		req := pboperator.TransferLeaderRequest{
			ID: "",
		}
		ctxToken, err := external.ContextWithQueryOptions(ctx, structs.QueryOptions{Token: tokenWrite.SecretID})
		require.NoError(t, err)
		reply, err := operatorClient.TransferLeader(ctxToken, &req)
		require.NoError(t, err)
		require.True(t, reply.Success)
		time.Sleep(1 * time.Second)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		retry.Run(t, func(r *retry.R) {
			time.Sleep(1 * time.Second)
			afterLeader, _ := s1.raft.LeaderWithID()
			require.NotEmpty(r, afterLeader)
			if afterLeader == beforeLeader {
				r.Fatalf("leader should have changed %s == %s", afterLeader, beforeLeader)
			}
		})
	})
}
