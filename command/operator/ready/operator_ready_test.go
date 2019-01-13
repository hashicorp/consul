package ready

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestOperatorReadyCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorReadyCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	t.Run("Test the ready command", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{"-http-addr=" + a.HTTPAddr(), "-address=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
	})

}
