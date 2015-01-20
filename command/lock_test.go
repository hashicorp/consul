package command

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
)

func TestLockCommand_implements(t *testing.T) {
	var _ cli.Command = &LockCommand{}
}

func TestLockCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	ui := new(cli.MockUi)
	c := &LockCommand{Ui: ui}
	filePath := filepath.Join(a1.dir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a1.httpAddr, "test/prefix", touchCmd}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Check for the file
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}
