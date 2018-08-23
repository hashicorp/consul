package proxy

import (
	"strings"
)

// isProcessAlreadyFinishedErr does a janky comparison with an error string
// defined in os/exec_unix.go and os/exec_windows.go which we encounter due to
// races with polling the external process. These case tests to fail since Stop
// returns an error sometimes so we should notice if this string stops matching
// the error in a future go version.
func isProcessAlreadyFinishedErr(err error) bool {
	return strings.Contains(err.Error(), "os: process already finished")
}
