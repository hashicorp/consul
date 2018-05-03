// +build !windows

package proxy

import (
	"os"
	"syscall"
)

// processAlive for non-Windows. Note that this very likely doesn't
// work for all non-Windows platforms Go supports and we should expand
// support as we experience it.
func processAlive(p *os.Process) error {
	// On Unix-like systems, we can verify a process is alive by sending
	// a 0 signal. This will do nothing to the process but will still
	// return errors if the process is gone.
	err := p.Signal(syscall.Signal(0))
	if err == nil || err == syscall.EPERM {
		return nil
	}

	return err
}
