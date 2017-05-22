package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func TestReloadCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ReloadCommand{}
}

func TestReloadCommandRun(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Setup a dummy response to errCh to simulate a successful reload
	go func() {
		errCh := <-a.ReloadCh()
		errCh <- nil
	}()

	ui := cli.NewMockUi()
	c := &ReloadCommand{
		Command: base.Command{
			UI:    ui,
			Flags: base.FlagSetClientHTTP,
		},
	}
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "reload triggered") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
