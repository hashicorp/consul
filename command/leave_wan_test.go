package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func testLeaveWANCommand(t *testing.T) (*cli.MockUi, *LeaveWANCommand) {
	ui := cli.NewMockUi()
	return ui, &LeaveWANCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetClientHTTP,
		},
	}
}

func TestLeaveWANCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &LeaveWANCommand{}
}

func TestLeaveWANCommandRun(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLeaveWANCommand(t)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "leave complete") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestLeaveWANCommandFailOnNonFlagArgs(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	_, c := testLeaveWANCommand(t)
	args := []string{"-http-addr=" + a.HTTPAddr(), "appserver1"}

	code := c.Run(args)
	if code == 0 {
		t.Fatalf("bad: failed to check for unexpected args")
	}
}
