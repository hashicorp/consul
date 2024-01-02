// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestAPI_OperatorAutopilotGetSetConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	operator := c.Operator()
	config, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %v", config)
	}

	// Change a config setting
	newConf := &AutopilotConfiguration{CleanupDeadServers: false}
	if err := operator.AutopilotSetConfiguration(newConf, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	config, err = operator.AutopilotGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if config.CleanupDeadServers {
		t.Fatalf("bad: %v", config)
	}
}

func TestAPI_OperatorAutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	retry.Run(t, func(r *retry.R) {
		operator := c.Operator()
		config, err := operator.AutopilotGetConfiguration(nil)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if !config.CleanupDeadServers {
			r.Fatalf("bad: %v", config)
		}

		// Pass an invalid ModifyIndex
		{
			newConf := &AutopilotConfiguration{
				CleanupDeadServers: false,
				ModifyIndex:        config.ModifyIndex - 1,
			}
			resp, err := operator.AutopilotCASConfiguration(newConf, nil)
			if err != nil {
				r.Fatalf("err: %v", err)
			}
			if resp {
				r.Fatalf("bad: %v", resp)
			}
		}

		// Pass a valid ModifyIndex
		{
			newConf := &AutopilotConfiguration{
				CleanupDeadServers: false,
				ModifyIndex:        config.ModifyIndex,
			}
			resp, err := operator.AutopilotCASConfiguration(newConf, nil)
			if err != nil {
				r.Fatalf("err: %v", err)
			}
			if !resp {
				r.Fatalf("bad: %v", resp)
			}
		}
	})
}

func TestAPI_OperatorAutopilotServerHealth(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.RaftProtocol = 3
	})
	defer s.Stop()

	operator := c.Operator()
	retry.Run(t, func(r *retry.R) {
		out, err := operator.AutopilotServerHealth(nil)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		if len(out.Servers) != 1 ||
			!out.Servers[0].Healthy ||
			out.Servers[0].Name != s.Config.NodeName {
			r.Fatalf("bad: %v", out)
		}
	})
}

func TestAPI_OperatorAutopilotState(t *testing.T) {
	c, s := makeClient(t)
	defer s.Stop()

	operator := c.Operator()
	retry.Run(t, func(r *retry.R) {
		out, err := operator.AutopilotState(nil)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		srv, ok := out.Servers[s.Config.NodeID]
		if !ok || !srv.Healthy || srv.Name != s.Config.NodeName {
			r.Fatalf("bad: %v", out)
		}
	})
}

func TestAPI_OperatorAutopilotServerHealth_429(t *testing.T) {
	mapi, client := setupMockAPI(t)

	reply := OperatorHealthReply{
		Healthy:          false,
		FailureTolerance: 0,
		Servers: []ServerHealth{
			{
				ID:          "d9fdded2-27ae-4db2-9232-9d8d0114ac98",
				Name:        "foo",
				Address:     "198.18.0.1:8300",
				SerfStatus:  "alive",
				Version:     "1.8.3",
				Leader:      true,
				LastContact: NewReadableDuration(0),
				LastTerm:    4,
				LastIndex:   99,
				Healthy:     true,
				Voter:       true,
				StableSince: time.Date(2020, 9, 2, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:          "1bcdda01-b896-41bc-a763-1a62b4260777",
				Name:        "bar",
				Address:     "198.18.0.2:8300",
				SerfStatus:  "alive",
				Version:     "1.8.3",
				Leader:      false,
				LastContact: NewReadableDuration(10 * time.Millisecond),
				LastTerm:    4,
				LastIndex:   99,
				Healthy:     true,
				Voter:       true,
				StableSince: time.Date(2020, 9, 2, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:          "661d1eac-81be-436b-bfe1-d51ffd665b9d",
				Name:        "baz",
				Address:     "198.18.0.3:8300",
				SerfStatus:  "failed",
				Version:     "1.8.3",
				Leader:      false,
				LastContact: NewReadableDuration(10 * time.Millisecond),
				LastTerm:    4,
				LastIndex:   99,
				Healthy:     false,
				Voter:       true,
			},
		},
	}
	mapi.withReply("GET", "/v1/operator/autopilot/health", nil, 429, reply).Once()

	out, err := client.Operator().AutopilotServerHealth(nil)
	require.NoError(t, err)
	require.Equal(t, &reply, out)

	mapi.withReply("GET", "/v1/operator/autopilot/health", nil, 500, nil).Once()
	_, err = client.Operator().AutopilotServerHealth(nil)

	var statusE StatusError
	if errors.As(err, &statusE) {
		require.Equal(t, 500, statusE.Code)
	} else {
		t.Error("Failed to unwrap error as StatusError")
	}

}
