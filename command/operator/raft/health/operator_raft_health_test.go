package health

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/mitchellh/cli"
)

func TestOperatorRaftHealthCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorRaftHealthCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	// Test the health subcommand directly
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	retry.Run(t, func(r *retry.R) {
		code := c.Run(args)
		if code != 0 {
			r.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		output := strings.TrimSpace(ui.OutputWriter.String())
		expected := "alive   true    0s"
		if !strings.Contains(output, expected) {
			r.Fatalf("bad: %q, %q", output, expected)
		}
		expected = "0 servers can fail without causing an outage"
		if !strings.Contains(output, expected) {
			r.Fatalf("bad: %q, %q", output, expected)
		}
	})
}
