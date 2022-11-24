package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
)

func argFail(t *testing.T, args []string, expected string) {
	ui := cli.NewMockUi()
	c := New(ui, nil)
	c.flags.SetOutput(ui.ErrorWriter)
	if code := c.Run(args); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}
	if reason := ui.ErrorWriter.String(); !strings.Contains(reason, expected) {
		t.Fatalf("bad reason: got='%s', expected='%s'", reason, expected)
	}

}

func TestLockCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi(), nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestLockCommand_BadArgs(t *testing.T) {
	t.Parallel()
	argFail(t, []string{"-try=blah", "test/prefix", "date"}, "parse error")
	argFail(t, []string{"-try=-10s", "test/prefix", "date"}, "Timeout must be positive")
	argFail(t, []string{"-monitor-retry=-5", "test/prefix", "date"}, "must be >= 0")
}

func TestLockCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "test/prefix", "touch", filePath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Check for the file
	_, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLockCommand_NoShell(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-shell=false", "test/prefix", "touch", filePath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Check for the file
	_, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLockCommand_TryLock(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-try=10s", "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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

func TestLockCommand_TrySemaphore(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "-try=10s", "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-monitor-retry=9", "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)

	filePath := filepath.Join(a.Config.DataDir, "test_touch")
	args := []string{"-http-addr=" + a.HTTPAddr(), "-n=3", "-monitor-retry=9", "test/prefix", "touch", filePath}

	// Run the command.
	var lu *LockUnlock
	code := c.run(args, &lu)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	_, err := os.ReadFile(filePath)
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

func TestLockCommand_ChildExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	tt := []struct {
		name string
		args []string
		want int
	}{
		{
			name: "clean exit",
			args: []string{"-http-addr=" + a.HTTPAddr(), "-child-exit-code", "test/prefix", "sh", "-c", "exit", "0"},
			want: 0,
		},
		{
			name: "error exit",
			args: []string{"-http-addr=" + a.HTTPAddr(), "-child-exit-code", "test/prefix", "exit", "1"},
			want: 2,
		},
		{
			name: "not propagated",
			args: []string{"-http-addr=" + a.HTTPAddr(), "test/prefix", "sh", "-c", "exit", "1"},
			want: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui, nil)

			if got := c.Run(tc.args); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}
