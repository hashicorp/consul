package api

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestOperator_AutopilotGetSetConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

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

func TestOperator_AutopilotCASConfiguration(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	operator := c.Operator()
	config, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %v", config)
	}

	// Pass an invalid ModifyIndex
	{
		newConf := &AutopilotConfiguration{
			CleanupDeadServers: false,
			ModifyIndex:        config.ModifyIndex - 1,
		}
		resp, err := operator.AutopilotCASConfiguration(newConf, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp {
			t.Fatalf("bad: %v", resp)
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
			t.Fatalf("err: %v", err)
		}
		if !resp {
			t.Fatalf("bad: %v", resp)
		}
	}
}

func TestOperator_AutopilotServerHealth(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.RaftProtocol = 3
	})
	defer s.Stop()

	operator := c.Operator()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		out, err := operator.AutopilotServerHealth(nil)
		if err != nil {
			t.Logf("err: %v", err)
			continue
		}
		if got, want := len(out.Servers), 1; got != want {
			t.Logf("got %d servers want %d", got, want)
			continue
		}
		if got, want := out.Servers[0].Healthy, true; got != want {
			t.Logf("got healthy %s want %s", got, want)
			continue
		}
		if got, want := out.Servers[0].Name, s.Config.NodeName; got != want {
			t.Logf("got name %q want %q", got, want)
			continue
		}
		break
	}
}
