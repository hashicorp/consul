// +build windows

package command

import (
	"os"
	"syscall"
)

// signalPid sends a sig signal to the process with process id pid.
// Interrupts et al is not implemented on Windows. Always send a SIGKILL.
func signalPid(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = sig
	return p.Signal(syscall.SIGKILL)
}
