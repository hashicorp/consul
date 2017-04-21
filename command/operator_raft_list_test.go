package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func TestOperator_Raft_ListPeers_Implements(t *testing.T) {
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperator_Raft_ListPeers(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	expected := fmt.Sprintf("%s  127.0.0.1:%d  127.0.0.1:%d  leader  true   2",
		a1.config.NodeName, a1.config.Ports.Server, a1.config.Ports.Server)

	// Test the legacy mode with 'consul operator raft -list-peers'
	{
		ui, c := testOperatorRaftCommand(t)
		args := []string{"-http-addr=" + a1.httpAddr, "-list-peers"}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		output := strings.TrimSpace(ui.OutputWriter.String())
		if !strings.Contains(output, expected) {
			t.Fatalf("bad: %q, %q", output, expected)
		}
	}

	// Test the list-peers subcommand directly
	{
		ui := new(cli.MockUi)
		c := OperatorRaftListCommand{
			Command: base.Command{
				UI:    ui,
				Flags: base.FlagSetHTTP,
			},
		}
		args := []string{"-http-addr=" + a1.httpAddr}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		output := strings.TrimSpace(ui.OutputWriter.String())
		if !strings.Contains(output, expected) {
			t.Fatalf("bad: %q, %q", output, expected)
		}
	}
}
