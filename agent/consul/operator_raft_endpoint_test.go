package consul

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.RaftConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	future := s1.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(future.Configuration().Servers) != 1 {
		t.Fatalf("bad: %v", future.Configuration().Servers)
	}
	me := future.Configuration().Servers[0]
	expected := structs.RaftConfigurationResponse{
		Servers: []*structs.RaftServer{
			{
				ID:              me.ID,
				Node:            s1.config.NodeName,
				Address:         me.Address,
				Leader:          true,
				Voter:           true,
				ProtocolVersion: "3",
			},
		},
		Index: future.Index(),
	}
	require.Equal(t, expected, reply)
}

func TestOperator_RaftGetConfiguration_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Make a request with no token to make sure it gets denied.
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.RaftConfigurationResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Create an ACL with operator read permissions.
	rules := `operator = "read"`
	token := createToken(t, codec, rules)

	// Now it should go through.
	arg.Token = token
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	future := s1.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(future.Configuration().Servers) != 1 {
		t.Fatalf("bad: %v", future.Configuration().Servers)
	}
	me := future.Configuration().Servers[0]
	expected := structs.RaftConfigurationResponse{
		Servers: []*structs.RaftServer{
			{
				ID:              me.ID,
				Node:            s1.config.NodeName,
				Address:         me.Address,
				Leader:          true,
				Voter:           true,
				ProtocolVersion: "3",
			},
		},
		Index: future.Index(),
	}
	require.Equal(t, expected, reply)
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to remove a peer that's not there.
	arg := structs.RaftRemovePeerRequest{
		Datacenter: "dc1",
		Address:    raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", freeport.GetOne(t))),
	}
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	// Add it manually to Raft.
	{
		id := raft.ServerID("fake-node-id")
		future := s1.raft.AddVoter(id, arg.Address, 0, time.Second)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByAddress_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Make a request with no token to make sure it gets denied.
	arg := structs.RaftRemovePeerRequest{
		Datacenter: "dc1",
		Address:    raft.ServerAddress(s1.config.RPCAddr.String()),
	}
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	token := createToken(t, codec, `operator = "write" `)

	// Now it should kick back for being an invalid config, which means it
	// tried to do the operation.
	arg.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "at least one voter") {
		t.Fatalf("err: %v", err)
	}
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to remove a peer that's not there.
	arg := structs.RaftRemovePeerRequest{
		Datacenter: "dc1",
		ID:         raft.ServerID("e35bde83-4e9c-434f-a6ef-453f44ee21ea"),
	}
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	// Add it manually to Raft.
	{
		future := s1.raft.AddVoter(arg.ID, raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", freeport.GetOne(t))), 0, 0)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByID_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Make a request with no token to make sure it gets denied.
	arg := structs.RaftRemovePeerRequest{
		Datacenter: "dc1",
		ID:         raft.ServerID(s1.config.NodeID),
	}
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	token := createToken(t, codec, `operator = "write"`)

	// Now it should kick back for being an invalid config, which means it
	// tried to do the operation.
	arg.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "at least one voter") {
		t.Fatalf("err: %v", err)
	}
}
