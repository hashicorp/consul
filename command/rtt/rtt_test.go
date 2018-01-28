package rtt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/cli"
)

func TestRTTCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRTTCommand_BadArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
	}{
		{args: []string{}},
		{args: []string{"node1", "node2", "node3"}},
		{args: []string{"-wan", "node1", "node2"}},
		{args: []string{"-wan", "node1.dc1", "node2"}},
		{args: []string{"-wan", "node1", "node2.dc1"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			if code := c.Run(tt.args); code != 1 {
				t.Fatalf("expected return code 1, got %d", code)
			}
		})
	}
}

func TestRTTCommand_LAN(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), `
		consul = {
			coordinate = {
				update_period = "10ms"
			}
		}
	`)
	defer a.Shutdown()

	// Inject some known coordinates.
	c1 := coordinate.NewCoordinate(coordinate.DefaultConfig())
	c2 := c1.Clone()
	c2.Vec[0] = 0.123
	distStr := fmt.Sprintf("%.3f ms", c1.DistanceTo(c2).Seconds()*1000.0)
	{
		req := structs.CoordinateUpdateRequest{
			Datacenter: a.Config.Datacenter,
			Node:       a.Config.NodeName,
			Coord:      c1,
		}
		var reply struct{}
		if err := a.RPC("Coordinate.Update", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	{
		req := structs.RegisterRequest{
			Datacenter: a.Config.Datacenter,
			Node:       "dogs",
			Address:    "127.0.0.2",
		}
		var reply struct{}
		if err := a.RPC("Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	{
		var reply struct{}
		req := structs.CoordinateUpdateRequest{
			Datacenter: a.Config.Datacenter,
			Node:       "dogs",
			Coord:      c2,
		}
		if err := a.RPC("Coordinate.Update", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Ask for the RTT of two known nodes
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		a.Config.NodeName,
		"dogs",
	}
	// Wait for the updates to get flushed to the data store.
	retry.Run(t, func(r *retry.R) {
		code := c.Run(args)
		if code != 0 {
			r.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the proper RTT was reported in the output.
		expected := fmt.Sprintf("rtt: %s", distStr)
		if !strings.Contains(ui.OutputWriter.String(), expected) {
			r.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	})

	// Default to the agent's node.
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"dogs",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the proper RTT was reported in the output.
		expected := fmt.Sprintf("rtt: %s", distStr)
		if !strings.Contains(ui.OutputWriter.String(), expected) {
			t.Fatalf("bad: %#v", ui.OutputWriter.String())
		}
	}

	// Try an unknown node.
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			a.Config.NodeName,
			"nope",
		}
		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}
	}
}

func TestRTTCommand_WAN(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	node := fmt.Sprintf("%s.%s", a.Config.NodeName, a.Config.Datacenter)

	// We can't easily inject WAN coordinates, so we will just query the
	// node with itself.
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-wan",
			"-http-addr=" + a.HTTPAddr(),
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
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-wan",
			"-http-addr=" + a.HTTPAddr(),
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
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-wan",
			"-http-addr=" + a.HTTPAddr(),
			node,
			"dc1.nope",
		}
		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d: %#v", code, ui.ErrorWriter.String())
		}
	}
}
