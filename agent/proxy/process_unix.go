// +build !windows

package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// findProcess for non-Windows. Note that this very likely doesn't
// work for all non-Windows platforms Go supports and we should expand
// support as we experience it.
func findProcess(pid int) (*os.Process, error) {
	// FindProcess never fails on unix-like systems.
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// On Unix-like systems, we can verify a process is alive by sending
	// a 0 signal. This will do nothing to the process but will still
	// return errors if the process is gone.
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return p, nil
	}

	return nil, fmt.Errorf("process %d is dead or running as another user", pid)
}

// configureDaemon is called prior to Start to allow system-specific setup.
func configureDaemon(cmd *exec.Cmd) {
	// Start it in a new sessions (and hence process group) so that killing agent
	// (even with Ctrl-C) won't kill proxy.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
