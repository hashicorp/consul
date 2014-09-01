package command

import (
	"github.com/mitchellh/cli"
	"strings"
	"testing"

	"github.com/hashicorp/consul/testutil"
)

func TestExecCommand_implements(t *testing.T) {
	var _ cli.Command = &ExecCommand{}
}

func TestExecCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	ui := new(cli.MockUi)
	c := &ExecCommand{Ui: ui}
	args := []string{"-http-addr=" + a1.httpAddr, "-wait=400ms", "uptime"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "load") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func waitForLeader(t *testing.T, httpAddr string) {
	client, err := HTTPClient(httpAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	testutil.WaitForResult(func() (bool, error) {
		_, qm, err := client.Catalog().Nodes(nil)
		if err != nil {
			return false, err
		}
		return qm.KnownLeader, nil
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}
