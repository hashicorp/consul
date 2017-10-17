package watch

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestWatchCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New(cli.NewMockUi(), nil).Help(), '\t') {
		t.Fatal("usage has tabs")
	}
}

func TestWatchCommand(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-type=nodes"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
