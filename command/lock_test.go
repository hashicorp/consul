package command

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mitchellh/cli"
)

func TestLockCommand_implements(t *testing.T) {
	var _ cli.Command = &LockCommand{}
}

func TestLockCommand_BadArgs(t *testing.T) {
	ui := new(cli.MockUi)
	c := &LockCommand{Ui: ui}

	if code := c.Run([]string{"-try=blah"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"-try=-10s"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}
}

func TestLockCommand_Run(t *testing.T) {
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

func runTry(t *testing.T, n int) {
	a1 := testAgent(t)
	defer a1.Shutdown()
	waitForLeader(t, a1.httpAddr)

	// Define a long-running command.
	nArg := fmt.Sprintf("-n=%d", n)
	args := []string{"-http-addr=" + a1.httpAddr, nArg, "-try=250ms", "test/prefix", "sleep 2"}

	// Run several commands at once.
	var wg sync.WaitGroup
	locked := make([]bool, n+1)
	tried := make([]bool, n+1)
	for i := 0; i < n+1; i++ {
		wg.Add(1)
		go func(index int) {
			ui := new(cli.MockUi)
			c := &LockCommand{Ui: ui}

			code := c.Run(append([]string{"-try=250ms"}, args...))
			if code == 0 {
				locked[index] = true
			} else {
				reason := ui.ErrorWriter.String()
				if !strings.Contains(reason, "Shutdown triggered or timeout during lock acquisition") {
					t.Fatalf("bad reason: %s", reason)
				}
				tried[index] = true
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Tally up the outcomes.
	totalLocked := 0
	totalTried := 0
	for i := 0; i < n+1; i++ {
		if locked[i] == tried[i] {
			t.Fatalf("command %d didn't lock or try, or did both", i+1)
		}
		if locked[i] {
			totalLocked++
		}
		if tried[i] {
			totalTried++
		}
	}

	// We can't check exact counts because sometimes the try attempts may
	// fail because they get woken up but need to do another try, but we
	// should get one of each outcome.
	if totalLocked == 0 || totalTried == 0 {
		t.Fatalf("unexpected outcome: locked=%d, tried=%d", totalLocked, totalTried)
	}
}

func TestLockCommand_Try_Lock(t *testing.T) {
	runTry(t, 1)
}

func TestLockCommand_Try_Semaphore(t *testing.T) {
	runTry(t, 2)
	runTry(t, 3)
}
