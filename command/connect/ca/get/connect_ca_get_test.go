package get

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/testrpc"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestConnectCAGetConfigCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestConnectCAGetConfigCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, `"Provider": "consul"`) {
		t.Fatalf("bad: %s", output)
	}
}
