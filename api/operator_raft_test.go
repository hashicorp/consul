// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAPI_OperatorRaftGetConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	operator := c.Operator()
	out, err := operator.RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Servers) != 1 ||
		!out.Servers[0].Leader ||
		!out.Servers[0].Voter {
		t.Fatalf("bad: %v", out)
	}
}

func TestAPI_OperatorRaftRemovePeerByAddress(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s1.Stop()

	_, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = true
		conf.Bootstrap = false
		conf.RetryJoin = []string{s1.LANAddr}
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s2.Stop()
	s2.WaitForVoting(t)

	operator := c1.Operator()
	err := operator.RaftRemovePeerByAddress(s2.ServerAddr, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	cfg, err := c1.Operator().RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("more than 1 server left: %+v", cfg.Servers)
	}
}

func TestAPI_OperatorRaftRemovePeerByID(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s1.Stop()

	_, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = true
		conf.Bootstrap = false
		conf.RetryJoin = []string{s1.LANAddr}
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s2.Stop()
	s2.WaitForVoting(t)

	operator := c1.Operator()
	err := operator.RaftRemovePeerByID(s2.Config.NodeID, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	cfg, err := c1.Operator().RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("more than 1 server left: %+v", cfg.Servers)
	}
}

func TestAPI_OperatorRaftLeaderTransfer(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s1.Stop()

	_, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = true
		conf.Bootstrap = false
		conf.RetryJoin = []string{s1.LANAddr}
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s2.Stop()
	s2.WaitForVoting(t)

	cfg, err := c1.Operator().RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("not 2 servers: %#v", cfg.Servers)
	}
	var leaderID string
	for _, srv := range cfg.Servers {
		if srv.Leader {
			leaderID = srv.ID
		}
	}
	if leaderID == "" {
		t.Fatalf("no leader: %+v", cfg.Servers)
	}

	transfer, err := c1.Operator().RaftLeaderTransfer("", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !transfer.Success {
		t.Fatal("unsuccessful transfer")
	}

	s2.WaitForLeader(t)

	cfg, err = c1.Operator().RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var newLeaderID string
	for _, srv := range cfg.Servers {
		if srv.Leader {
			newLeaderID = srv.ID
		}
	}
	if newLeaderID == "" {
		t.Fatalf("no leader: %#v", cfg.Servers)
	}
	if newLeaderID == leaderID {
		t.Fatalf("leader did not change: %v == %v", newLeaderID, leaderID)
	}

	_, s3 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = true
		conf.Bootstrap = false
		conf.RetryJoin = []string{s1.LANAddr, s2.LANAddr}
		if conf.Autopilot == nil {
			conf.Autopilot = &testutil.TestAutopilotConfig{}
		}
		conf.Autopilot.ServerStabilizationTime = "1ms"
	})
	defer s3.Stop()
	s3.WaitForVoting(t)

	// Transfer it to another member
	transfer, err = c1.Operator().RaftLeaderTransfer(s3.Config.NodeID, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !transfer.Success {
		t.Fatal("unsuccessful transfer")
	}

	s3.WaitForLeader(t)

	cfg, err = c1.Operator().RaftGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	newLeaderID = ""
	for _, srv := range cfg.Servers {
		if srv.Leader {
			newLeaderID = srv.ID
		}
	}
	if newLeaderID == "" {
		t.Fatalf("no leader: %#v", cfg.Servers)
	}
	if newLeaderID != s3.Config.NodeID {
		t.Fatalf("leader is not s3: %v != %v", newLeaderID, s3.Config.NodeID)
	}
}
