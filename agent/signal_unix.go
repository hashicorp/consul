// +build !windows

package agent

import (
	"os"
	"syscall"
)

var forwardSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
