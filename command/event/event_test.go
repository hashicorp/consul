package event

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestEventCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestEventCommand(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t, t.Name(), ``)
	defer a1.Shutdown()

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-http-addr=" + a1.HTTPAddr(), "-name=cmd"}

	code := cmd.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "Event ID: ") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
