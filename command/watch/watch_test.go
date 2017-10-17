package watch

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestWatchCommandRun(t *testing.T) {
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
