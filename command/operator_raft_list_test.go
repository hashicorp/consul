package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestOperator_Raft_ListPeers_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperator_Raft_ListPeers(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	expected := fmt.Sprintf("%s  %s  127.0.0.1:%d  leader  true   3",
		a.Config.NodeName, a.Config.NodeID, a.Config.ServerPort)

	// Test the list-peers subcommand directly
	{
		ui := cli.NewMockUi()
		c := OperatorRaftListCommand{
			BaseCommand: BaseCommand{
				UI:    ui,
				Flags: FlagSetHTTP,
			},
		}
		args := []string{"-http-addr=" + a.HTTPAddr()}

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
