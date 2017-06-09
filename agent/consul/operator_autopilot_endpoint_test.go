package consul

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/raft"
)

func TestOperator_Autopilot_GetConfiguration(t *testing.T) {
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
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("err: %v", err)
	}

	// Create an ACL with operator read permissions.
	var token string
	{
		var rules = `
                    operator = "read"
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

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
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %#v", config)
	}
}

func TestOperator_Autopilot_SetConfiguration_ACLDeny(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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
	if err == nil || !strings.Contains(err.Error(), permissionDenied) {
		t.Fatalf("err: %v", err)
	}

	// Create an ACL with operator write permissions.
	var token string
	{
		var rules = `
                    operator = "write"
                `

		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &token); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

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
		var reply structs.OperatorHealthReply
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

func TestOperator_ServerHealth_UnsupportedRaftVersion(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.RaftConfig.ProtocolVersion = 2
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.OperatorHealthReply
	err := msgpackrpc.CallWithCodec(codec, "Operator.ServerHealth", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "raft_protocol set to 3 or higher") {
		t.Fatalf("bad: %v", err)
	}
}
