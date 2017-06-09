package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestOperator_Autopilot_Get_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorAutopilotGetCommand{}
}

func TestOperator_Autopilot_Get(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := OperatorAutopilotGetCommand{
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
	if !strings.Contains(output, "CleanupDeadServers = true") {
		t.Fatalf("bad: %s", output)
	}
}
