package command

import (
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestJoinCommand_implements(t *testing.T) {
	var _ cli.Command = &JoinCommand{}
}

func TestJoinCommandRun(t *testing.T) {
	a1 := testAgent(t)
	a2 := testAgent(t)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui := new(cli.MockUi)
	c := &JoinCommand{Ui: ui}
	args := []string{
		"-rpc-addr=" + a1.addr,
		fmt.Sprintf("127.0.0.1:%d", a2.config.Ports.SerfLan),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.agent.LANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.agent.LANMembers())
	}
}

func TestJoinCommandRun_wan(t *testing.T) {
	a1 := testAgent(t)
	a2 := testAgent(t)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui := new(cli.MockUi)
	c := &JoinCommand{Ui: ui}
	args := []string{
		"-rpc-addr=" + a1.addr,
		"-wan",
		fmt.Sprintf("127.0.0.1:%d", a2.config.Ports.SerfWan),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.agent.WANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.agent.WANMembers())
	}
}

func TestJoinCommandRun_noAddrs(t *testing.T) {
	ui := new(cli.MockUi)
	c := &JoinCommand{Ui: ui}
	args := []string{"-rpc-addr=foo"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "one address") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
