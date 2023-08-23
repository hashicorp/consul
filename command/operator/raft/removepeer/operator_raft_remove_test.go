package removepeer

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestOperatorRaftRemovePeerCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorRaftRemovePeerCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	t.Run("Test the remove-peer subcommand directly", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{"-http-addr=" + a.HTTPAddr(), "-address=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// If we get this error, it proves we sent the address all they through.
		output := strings.TrimSpace(ui.ErrorWriter.String())
		if !strings.Contains(output, "address \"nope\" was not found in the Raft configuration") {
			t.Fatalf("bad: %s", output)
		}
	})

	t.Run("Test the remove-peer subcommand with -id", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{"-http-addr=" + a.HTTPAddr(), "-id=nope"}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// If we get this error, it proves we sent the address all they through.
		output := strings.TrimSpace(ui.ErrorWriter.String())
		if !strings.Contains(output, "id \"nope\" was not found in the Raft configuration") {
			t.Fatalf("bad: %s", output)
		}
	})
}
