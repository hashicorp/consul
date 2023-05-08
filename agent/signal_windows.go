//go:build windows
// +build windows

package agent

import (
	"os"
)

var forwardSignals = []os.Signal{os.Interrupt}
