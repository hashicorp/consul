package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func TestInfoCommand_implements(t *testing.T) {
	var _ cli.Command = &InfoCommand{}
}

func TestInfoCommandRun(t *testing.T) {
	a1 := agent.NewTestAgent(t.Name(), nil)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &InfoCommand{
		Command: base.Command{
			UI:    ui,
			Flags: base.FlagSetClientHTTP,
		},
	}
	args := []string{"-http-addr=" + a1.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "agent") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
