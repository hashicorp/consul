package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func testMembersCommand(t *testing.T) (*cli.MockUi, *MembersCommand) {
	ui := new(cli.MockUi)
	return ui, &MembersCommand{
		Command: base.Command{
			UI:    ui,
			Flags: base.FlagSetClientHTTP,
		},
	}
}

func TestMembersCommand_implements(t *testing.T) {
	var _ cli.Command = &MembersCommand{}
}

func TestMembersCommandRun(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testMembersCommand(t)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Name
	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	// Agent type
	if !strings.Contains(ui.OutputWriter.String(), "server") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	// Datacenter
	if !strings.Contains(ui.OutputWriter.String(), "dc1") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_WAN(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testMembersCommand(t)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-wan"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), fmt.Sprintf("%d", a.Config.Ports.SerfWan)) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_statusFilter(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testMembersCommand(t)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-status=a.*e",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_statusFilter_failed(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testMembersCommand(t)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-status=(fail|left)",
	}

	code := c.Run(args)
	if code == 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	if code != 2 {
		t.Fatalf("bad: %d", code)
	}
}
