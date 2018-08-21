// +build !darwin,!linux,!windows

package proxy

import "os"

// exitStatus for other platforms where we don't know how to extract it.
func exitStatus(ps *os.ProcessState) (int, bool) {
	return 0, false
}
