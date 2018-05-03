// +build windows

package proxy

import (
	"fmt"
	"os"
)

func processAlive(p *os.Process) error {
	// On Windows, os.FindProcess will error if the process is not alive,
	// so we don't have to do any further checking. The nature of it being
	// non-nil means it seems to be healthy.
	if p == nil {
		return fmt.Errof("process no longer alive")
	}

	return nil
}
