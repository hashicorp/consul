package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func testJoinCommand(t *testing.T) (*cli.MockUi, *JoinCommand) {
	ui := cli.NewMockUi()
	return ui, &JoinCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetClientHTTP,
		},
	}
}

func TestJoinCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &JoinCommand{}
}

func TestJoinCommandRun(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t.Name(), nil)
	a2 := agent.NewTestAgent(t.Name(), nil)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui, c := testJoinCommand(t)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		fmt.Sprintf("127.0.0.1:%d", a2.Config.Ports.SerfLan),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.LANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.LANMembers())
	}
}

func TestJoinCommandRun_wan(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t.Name(), nil)
	a2 := agent.NewTestAgent(t.Name(), nil)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui, c := testJoinCommand(t)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		"-wan",
		fmt.Sprintf("127.0.0.1:%d", a2.Config.Ports.SerfWan),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.WANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.WANMembers())
	}
}

func TestJoinCommandRun_noAddrs(t *testing.T) {
	t.Parallel()
	ui, c := testJoinCommand(t)
	args := []string{"-http-addr=foo"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "one address") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
