package command

import (
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestInfoCommand_implements(t *testing.T) {
	var _ cli.Command = &InfoCommand{}
}

func TestInfoCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &InfoCommand{Ui: ui}
	args := []string{"-rpc-addr=" + a1.addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "agent") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
