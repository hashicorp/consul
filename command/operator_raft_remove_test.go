package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestOperator_Raft_RemovePeer_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftRemoveCommand{}
}

func TestOperator_Raft_RemovePeer(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Test the legacy mode with 'consul operator raft -remove-peer'
	{
		ui, c := testOperatorRaftCommand(t)
		args := []string{"-http-addr=" + a.HTTPAddr(), "-remove-peer", "-address=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// If we get this error, it proves we sent the address all they through.
		output := strings.TrimSpace(ui.ErrorWriter.String())
		if !strings.Contains(output, "address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("bad: %s", output)
		}
	}

	// Test the remove-peer subcommand directly
	{
		ui := cli.NewMockUi()
		c := OperatorRaftRemoveCommand{
			BaseCommand: BaseCommand{
				UI:    ui,
				Flags: FlagSetHTTP,
			},
		}
		args := []string{"-http-addr=" + a.HTTPAddr(), "-address=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// If we get this error, it proves we sent the address all they through.
		output := strings.TrimSpace(ui.ErrorWriter.String())
		if !strings.Contains(output, "address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("bad: %s", output)
		}
	}

	// Test the remove-peer subcommand with -id
	{
		ui := cli.NewMockUi()
		c := OperatorRaftRemoveCommand{
			BaseCommand: BaseCommand{
				UI:    ui,
				Flags: FlagSetHTTP,
			},
		}
		args := []string{"-http-addr=" + a.HTTPAddr(), "-id=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// If we get this error, it proves we sent the address all they through.
		output := strings.TrimSpace(ui.ErrorWriter.String())
		if !strings.Contains(output, "id \"nope\" was not found in the Raft configuration") {
			t.Fatalf("bad: %s", output)
		}
	}
}
