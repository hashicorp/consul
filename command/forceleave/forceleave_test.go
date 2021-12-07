package forceleave

import (
	"strings"
	"testing"

	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestForceLeaveCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestForceLeaveCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := agent.NewTestAgent(t, ``)
	a2 := agent.NewTestAgent(t, ``)
	defer a1.Shutdown()
	defer a2.Shutdown()

	_, err := a2.JoinLAN([]string{a1.Config.SerfBindAddrLAN.String()}, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Forcibly shutdown a2 so that it appears "failed" in a1
	a2.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		a2.Config.NodeName,
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	m := a1.LANMembersInAgentPartition()
	if len(m) != 2 {
		t.Fatalf("should have 2 members: %#v", m)
	}
	retry.Run(t, func(r *retry.R) {
		m = a1.LANMembersInAgentPartition()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestForceLeaveCommand_NoNodeWithName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := agent.NewTestAgent(t, ``)
	defer a1.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		"garbage-name",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestForceLeaveCommand_prune(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := agent.StartTestAgent(t, agent.TestAgent{Name: "Agent1"})
	defer a1.Shutdown()
	a2 := agent.StartTestAgent(t, agent.TestAgent{Name: "Agent2"})
	defer a2.Shutdown()

	_, err := a2.JoinLAN([]string{a1.Config.SerfBindAddrLAN.String()}, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Forcibly shutdown a2 so that it appears "failed" in a1
	a2.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		"-prune",
		a2.Config.NodeName,
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	retry.Run(t, func(r *retry.R) {
		m := len(a1.LANMembersInAgentPartition())
		if m != 1 {
			r.Fatalf("should have 1 members, got %#v", m)
		}
	})

}

func TestForceLeaveCommand_noAddrs(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=foo"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "node name") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
