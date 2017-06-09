package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
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

	ui := cli.NewMockUi()
	c := &EventCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetClientHTTP,
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
