package command

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/cli"
)

func TestRttCommand_Implements(t *testing.T) {
	var _ cli.Command = &RttCommand{}
}

func TestRttCommand_Run_BadArgs(t *testing.T) {
	ui := new(cli.MockUi)
	c := &RttCommand{Ui: ui}

	if code := c.Run([]string{}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"node1", "node2", "node3"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"-wan", "node1", "node2"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"-wan", "node1.dc1", "node2"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"-wan", "node1", "node2.dc1"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}
}

func TestRttCommand_Run_LAN(t *testing.T) {
	updatePeriod := 10 * time.Millisecond
	a := testAgentWithConfig(t, func(c *agent.Config) {
		c.ConsulConfig.CoordinateUpdatePeriod = updatePeriod
	})
	defer a.Shutdown()
	waitForLeader(t, a.httpAddr)

	// Inject some known coordinates.
	c1 := coordinate.NewCoordinate(coordinate.DefaultConfig())
	c2 := c1.Clone()
	c2.Vec[0] = 0.123
	dist_str := fmt.Sprintf("%.3f ms", c1.DistanceTo(c2).Seconds()*1000.0)

	req1 := structs.CoordinateUpdateRequest{
		Datacenter: a.config.Datacenter,
		Node:       a.config.NodeName,
		Coord:      c1,
	}
	var reply struct{}
	if err := a.agent.RPC("Coordinate.Update", &req1, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}

	req2 := structs.CoordinateUpdateRequest{
		Datacenter: a.config.Datacenter,
		Node:       "dogs",
		Coord:      c2,
	}
	if err := a.agent.RPC("Coordinate.Update", &req2, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Wait for the updates to get flushed to the data store.
	time.Sleep(2 * updatePeriod)

	// Try two known nodes.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-http-addr=" + a.httpAddr,
			a.config.NodeName,
			"dogs",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the proper RTT was reported in the output.
		expected := fmt.Sprintf("rtt: %s", dist_str)
		if !strings.Contains(ui.OutputWriter.String(), expected) {
			t.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	}

	// Default to the agent's node.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-http-addr=" + a.httpAddr,
			"dogs",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the proper RTT was reported in the output.
		expected := fmt.Sprintf("rtt: %s", dist_str)
		if !strings.Contains(ui.OutputWriter.String(), expected) {
			t.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	}

	// Try an unknown node.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-http-addr=" + a.httpAddr,
			a.config.NodeName,
			"nope",
		}
		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}
	}
}

func TestRttCommand_Run_WAN(t *testing.T) {
	a := testAgent(t)
	defer a.Shutdown()
	waitForLeader(t, a.httpAddr)

	node := fmt.Sprintf("%s.%s", a.config.NodeName, a.config.Datacenter)

	// We can't easily inject WAN coordinates, so we will just query the
	// node with itself.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-wan",
			"-http-addr=" + a.httpAddr,
			node,
			node,
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure there was some kind of RTT reported in the output.
		if !strings.Contains(ui.OutputWriter.String(), "rtt: ") {
			t.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	}

	// Default to the agent's node.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-wan",
			"-http-addr=" + a.httpAddr,
			node,
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure there was some kind of RTT reported in the output.
		if !strings.Contains(ui.OutputWriter.String(), "rtt: ") {
			t.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	}

	// Try an unknown node.
	{
		ui := new(cli.MockUi)
		c := &RttCommand{Ui: ui}
		args := []string{
			"-wan",
			"-http-addr=" + a.httpAddr,
			node,
			"dc1.nope",
		}
		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}
	}
}
