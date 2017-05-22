package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func TestEventCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &EventCommand{}
}

func TestEventCommandRun(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t.Name(), nil)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &EventCommand{
		Command: base.Command{
			UI:    ui,
			Flags: base.FlagSetClientHTTP,
		},
	}
	args := []string{"-http-addr=" + a1.HTTPAddr(), "-name=cmd"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "Event ID: ") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
