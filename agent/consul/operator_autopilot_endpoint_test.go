// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestOperator_Autopilot_GetConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.AutopilotConfig.CleanupDeadServers = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.AutopilotConfig
	err := msgpackrpc.CallWithCodec(codec, "Operator.AutopilotGetConfiguration", &arg, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
}

func TestOperator_Autopilot_GetConfiguration_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.AutopilotConfig.CleanupDeadServers = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to get config without permissions
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.AutopilotConfig
	err := msgpackrpc.CallWithCodec(codec, "Operator.AutopilotGetConfiguration", &arg, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	token := createToken(t, codec, `operator = "read"`)

	// Now we can read and verify the config
	arg.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Operator.AutopilotGetConfiguration", &arg, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
}

func TestOperator_Autopilot_SetConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.AutopilotConfig.CleanupDeadServers = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Change the autopilot config from the default
	arg := structs.AutopilotSetConfigRequest{
		Datacenter: "dc1",
		Config: structs.AutopilotConfig{
			CleanupDeadServers: true,
			MinQuorum:          3,
		},
	}
	var reply *bool
	err := msgpackrpc.CallWithCodec(codec, "Operator.AutopilotSetConfiguration", &arg, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's changed
	state := s1.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !config.CleanupDeadServers && config.MinQuorum != 3 {
		t.Fatalf("bad: %#v", config)
	}
}

func TestOperator_Autopilot_SetConfiguration_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.AutopilotConfig.CleanupDeadServers = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try to set config without permissions
	arg := structs.AutopilotSetConfigRequest{
		Datacenter: "dc1",
		Config: structs.AutopilotConfig{
			CleanupDeadServers: true,
		},
	}
	var reply *bool
	err := msgpackrpc.CallWithCodec(codec, "Operator.AutopilotSetConfiguration", &arg, &reply)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	token := createToken(t, codec, `operator = "write"`)

	// Now we can update the config
	arg.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Operator.AutopilotSetConfiguration", &arg, &reply)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's changed
	state := s1.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %#v", config)
	}
}

func TestOperator_ServerHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
		c.ServerHealthInterval = 100 * time.Millisecond
		c.AutopilotInterval = 100 * time.Millisecond
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	joinLAN(t, s2, s1)

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()
	joinLAN(t, s3, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	retry.Run(t, func(r *retry.R) {
		arg := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.AutopilotHealthReply
		err := msgpackrpc.CallWithCodec(codec, "Operator.ServerHealth", &arg, &reply)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if !reply.Healthy {
			r.Fatalf("bad: %v", reply)
		}
		if reply.FailureTolerance != 1 {
			r.Fatalf("bad: %v", reply)
		}
		if len(reply.Servers) != 3 {
			r.Fatalf("bad: %v", reply)
		}
		// Leader should have LastContact == 0, others should be positive
		for _, s := range reply.Servers {
			isLeader := s1.raft.Leader() == raft.ServerAddress(s.Address)
			if isLeader && s.LastContact != 0 {
				r.Fatalf("bad: %v", reply)
			}
			if !isLeader && s.LastContact <= 0 {
				r.Fatalf("bad: %v", reply)
			}
		}
	})
}

func TestOperator_AutopilotState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
		c.ServerHealthInterval = 100 * time.Millisecond
		c.AutopilotInterval = 100 * time.Millisecond
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	joinLAN(t, s2, s1)

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()
	joinLAN(t, s3, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	retry.Run(t, func(r *retry.R) {
		arg := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply autopilot.State
		err := msgpackrpc.CallWithCodec(codec, "Operator.AutopilotState", &arg, &reply)
		require.NoError(r, err)
		require.True(r, reply.Healthy)
		require.Equal(r, 1, reply.FailureTolerance)
		require.Len(r, reply.Servers, 3)

		// Leader should have LastContact == 0, others should be positive
		for _, s := range reply.Servers {
			isLeader := s1.raft.Leader() == s.Server.Address
			if isLeader {
				require.Zero(r, s.Stats.LastContact)
			} else {
				require.NotEqual(r, time.Duration(0), s.Stats.LastContact)
			}
		}
	})
}
