package monitor

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/logger"
	"github.com/mitchellh/cli"
)

func TestMonitorCommand_exitsOnSignalBeforeLinesArrive(t *testing.T) {
	t.Parallel()
	logWriter := logger.NewLogWriter(512)
	a := &agent.TestAgent{
		Name:      t.Name(),
		LogWriter: logWriter,
		LogOutput: io.MultiWriter(os.Stderr, logWriter),
	}
	a.Start()
	defer a.Shutdown()

	shutdownCh := make(chan struct{})

	ui := cli.NewMockUi()
	c := New(ui, shutdownCh)
	// Only wait for ERR which shouldn't happen so should leave logs empty for a
	// while
	args := []string{"-http-addr=" + a.HTTPAddr(), "-log-level=ERR"}

	// Buffer it so we don't deadlock when blocking send on shutdownCh triggers
	// Run to return before we can select on it.
	exitCode := make(chan int, 1)

	// Run the monitor in another go routine. If this doesn't exit on our "signal"
	// then the whole test will hang and we'll panic (to not blow up if people run
	// the suite without -timeout)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done() // Signal that this goroutine is at least running now
		exitCode <- c.Run(args)
	}()

	// Wait for that routine to at least be running
	wg.Wait()

	// Simulate signal in a few milliseconds without blocking this thread
	go func() {
		time.Sleep(5 * time.Millisecond)
		shutdownCh <- struct{}{}
	}()

	// Wait for a second - shouldn't take long to handle the signal before we
	// panic. We simulate inside the select since the other goroutine might
	// already have exited if there was some error and we'd block forever trying
	// to write to unbuffered shutdownCh. We don't just buffer it because then it
	// doesn't model the real shutdownCh we use for signal watching and could mask
	// bugs in the handling.
	select {
	case ret := <-exitCode:
		if ret != 0 {
			t.Fatal("command returned with non-zero code")
		}
		// OK!
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for exit")
	}
}
