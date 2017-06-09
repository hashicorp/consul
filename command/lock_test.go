package command

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func testLockCommand(t *testing.T) (*cli.MockUi, *LockCommand) {
	ui := cli.NewMockUi()
	return ui, &LockCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetHTTP,
		},
	}
}

func TestLockCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &LockCommand{}
}

func argFail(t *testing.T, args []string, expected string) {
	ui, c := testLockCommand(t)
	if code := c.Run(args); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}
	if reason := ui.ErrorWriter.String(); !strings.Contains(reason, expected) {
		t.Fatalf("bad reason: got='%s', expected='%s'", reason, expected)
	}

}

func TestLockCommand_BadArgs(t *testing.T) {
	t.Parallel()
	argFail(t, []string{"-try=blah", "test/prefix", "date"}, "invalid duration")
	argFail(t, []string{"-try=-10s", "test/prefix", "date"}, "Timeout must be positive")
	argFail(t, []string{"-monitor-retry=-5", "test/prefix", "date"}, "must be >= 0")
}

func TestLockCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "test/prefix", touchCmd}

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

func TestLockCommand_Try_Lock(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-try=10s", "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the try options were set correctly.
	opts, ok := lu.rawOpts.(*api.LockOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if !opts.LockTryOnce || opts.LockWaitTime != 10*time.Second {
		t.Fatalf("bad: %#v", opts)
	}
}

func TestLockCommand_Try_Semaphore(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "-try=10s", "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the try options were set correctly.
	opts, ok := lu.rawOpts.(*api.SemaphoreOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if !opts.SemaphoreTryOnce || opts.SemaphoreWaitTime != 10*time.Second {
		t.Fatalf("bad: %#v", opts)
	}
}

func TestLockCommand_MonitorRetry_Lock_Default(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the monitor options were set correctly.
	opts, ok := lu.rawOpts.(*api.LockOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if opts.MonitorRetries != defaultMonitorRetry ||
		opts.MonitorRetryTime != defaultMonitorRetryTime {
		t.Fatalf("bad: %#v", opts)
	}
}

func TestLockCommand_MonitorRetry_Semaphore_Default(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the monitor options were set correctly.
	opts, ok := lu.rawOpts.(*api.SemaphoreOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if opts.MonitorRetries != defaultMonitorRetry ||
		opts.MonitorRetryTime != defaultMonitorRetryTime {
		t.Fatalf("bad: %#v", opts)
	}
}

func TestLockCommand_MonitorRetry_Lock_Arg(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-monitor-retry=9", "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the monitor options were set correctly.
	opts, ok := lu.rawOpts.(*api.LockOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if opts.MonitorRetries != 9 ||
		opts.MonitorRetryTime != defaultMonitorRetryTime {
		t.Fatalf("bad: %#v", opts)
	}
}

func TestLockCommand_MonitorRetry_Semaphore_Arg(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testLockCommand(t)
	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	touchCmd := fmt.Sprintf("touch '%s'", filePath)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "-monitor-retry=9", "test/prefix", touchCmd}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the monitor options were set correctly.
	opts, ok := lu.rawOpts.(*api.SemaphoreOptions)
	if !ok {
		t.Fatalf("bad type")
	}
	if opts.MonitorRetries != 9 ||
		opts.MonitorRetryTime != defaultMonitorRetryTime {
		t.Fatalf("bad: %#v", opts)
	}
}
