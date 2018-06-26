package watch

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestWatchCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi(), nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestWatchCommand(t *testing.T) {
	t.Parallel()
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

func TestWatchCommandNoConnect(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-type=connect_leaf"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.ErrorWriter.String(),
		"Type connect_leaf is not supported in the CLI tool") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
