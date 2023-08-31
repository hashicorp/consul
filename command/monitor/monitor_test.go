// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package monitor

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestMonitorCommand_exitsOnSignalBeforeLinesArrive(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.StartTestAgent(t, agent.TestAgent{})
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

func TestMonitorCommand_LogJSONValidFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.StartTestAgent(t, agent.TestAgent{})
	defer a.Shutdown()

	shutdownCh := make(chan struct{})

	ui := cli.NewMockUi()
	c := New(ui, shutdownCh)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-log-json"}

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

func TestMonitorCommand_LogJSONValidFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.StartTestAgent(t, agent.TestAgent{})
	defer a.Shutdown()

	shutdownCh := make(chan struct{})

	ui := cli.NewMockUi()
	c := New(ui, shutdownCh)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-log-json"}

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

	// Read the logs and try to json marshall it
	go func() {
		time.Sleep(1 * time.Second)
		outputs := ui.OutputWriter.String()
		for count, output := range strings.Split(outputs, "\n") {
			if output != "" && count > 0 {
				jsonLog := new(map[string]interface{})
				err := json.Unmarshal([]byte(output), jsonLog)
				if err != nil {
					exitCode <- -1
				}
				if len(*jsonLog) <= 0 {
					exitCode <- 1
				}
			}
		}
		shutdownCh <- struct{}{}

	}()

	select {
	case ret := <-exitCode:
		if ret != 0 {
			t.Fatal("command returned with non-zero code")
		}
		// OK!
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for exit")
	}
}
