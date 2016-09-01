package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestOperator_Implements(t *testing.T) {
	var _ cli.Command = &OperatorCommand{}
}

func TestOperator_Raft_ListPeers(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	ui := new(cli.MockUi)
	c := &OperatorCommand{Ui: ui}
	args := []string{"raft", "-http-addr=" + a1.httpAddr, "-list-peers"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "leader") {
		t.Fatalf("bad: %s", output)
	}
}

func TestOperator_Raft_RemovePeer(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	ui := new(cli.MockUi)
	c := &OperatorCommand{Ui: ui}
	args := []string{"raft", "-http-addr=" + a1.httpAddr, "-remove-peer", "-address=nope"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// If we get this error, it proves we sent the address all they through.
	output := strings.TrimSpace(ui.ErrorWriter.String())
	if !strings.Contains(output, "address \"nope\" was not found in the Raft configuration") {
		t.Fatalf("bad: %s", output)
	}
}
