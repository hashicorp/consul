package command

import (
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestReachabilityCommand_implements(t *testing.T) {
	var _ cli.Command = &ReachabilityCommand{}
}

func TestReachabilityCommand_Run(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &ReachabilityCommand{Ui: ui}
	args := []string{"-rpc-addr=" + a1.addr, "-verbose"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a1.config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
	if !strings.Contains(ui.OutputWriter.String(), "Successfully") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
