package transferleader

import (
	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestOperatorRaftListPeersCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

// This only test that the command behave correctly when only one agent is present
// and no leadership transfer is possible, testing for the functionality will be done at the RPC level.
func TestOperatorRaftTransferWithSingleNode(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	expected := "(cannot find peer)"

	// Test the list-peers subcommand directly
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{"-http-addr=" + a.HTTPAddr()}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.ErrorWriter.String())
	if !strings.Contains(output, expected) {
		t.Fatalf("bad: %q, %q", output, expected)
	}
}
