package command

import (
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestReloadCommand_implements(t *testing.T) {
	var _ cli.Command = &ReloadCommand{}
}

func TestReloadCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &ReloadCommand{Ui: ui}
	args := []string{"-rpc-addr=" + a1.addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "reload triggered") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
