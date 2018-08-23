// +build darwin linux windows

package proxy

import (
	"os"
	"syscall"
)

// exitStatus for platforms with syscall.WaitStatus which are listed
// at the top of this file in the build constraints.
func exitStatus(ps *os.ProcessState) (int, bool) {
	if status, ok := ps.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus(), true
	}

	return 0, false
}
