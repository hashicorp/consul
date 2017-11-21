package join

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestJoinCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestJoinCommandJoin_lan(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t.Name(), ``)
	a2 := agent.NewTestAgent(t.Name(), ``)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		a2.Config.SerfBindAddrLAN.String(),
	}

	code := cmd.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.LANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.LANMembers())
	}
}

func TestJoinCommand_wan(t *testing.T) {
	t.Parallel()
	a1 := agent.NewTestAgent(t.Name(), ``)
	a2 := agent.NewTestAgent(t.Name(), ``)
	defer a1.Shutdown()
	defer a2.Shutdown()

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{
		"-http-addr=" + a1.HTTPAddr(),
		"-wan",
		a2.Config.SerfBindAddrWAN.String(),
	}

	code := cmd.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.WANMembers()) != 2 {
		t.Fatalf("bad: %#v", a1.WANMembers())
	}
}

func TestJoinCommand_noAddrs(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-http-addr=foo"}

	code := cmd.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "one address") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
