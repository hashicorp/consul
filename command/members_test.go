package command

import (
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
	"testing"
)

func TestMembersCommand_implements(t *testing.T) {
	var _ cli.Command = &MembersCommand{}
}

func TestMembersCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MembersCommand{Ui: ui}
	args := []string{"-rpc-addr=" + a1.addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a1.config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_WAN(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MembersCommand{Ui: ui}
	args := []string{"-rpc-addr=" + a1.addr, "-wan"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), fmt.Sprintf("%d", a1.config.Ports.SerfWan)) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_statusFilter(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MembersCommand{Ui: ui}
	args := []string{
		"-rpc-addr=" + a1.addr,
		"-status=a.*e",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a1.config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandRun_statusFilter_failed(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MembersCommand{Ui: ui}
	args := []string{
		"-rpc-addr=" + a1.addr,
		"-status=(fail|left)",
	}

	code := c.Run(args)
	if code == 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if strings.Contains(ui.OutputWriter.String(), a1.config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	if code != 2 {
		t.Fatalf("bad: %d", code)
	}
}
