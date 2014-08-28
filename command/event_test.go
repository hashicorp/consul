package command

import (
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestEventCommand_implements(t *testing.T) {
	var _ cli.Command = &WatchCommand{}
}

func TestEventCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &EventCommand{Ui: ui}
	args := []string{"-http-addr=" + a1.httpAddr, "-name=cmd"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "Event ID: ") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
