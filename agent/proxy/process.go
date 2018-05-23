package proxy

import (
	"strings"
	"time"
)

// isProcessAlreadyFinishedErr does a janky comparison with an error string
// defined in os/exec_unix.go and os/exec_windows.go which we encounter due to
// races with polling the external process. These case tests to fail since Stop
// returns an error sometimes so we should notice if this string stops matching
// the error in a future go version.
func isProcessAlreadyFinishedErr(err error) bool {
	return strings.Contains(err.Error(), "os: process already finished")
}

// externalWait mimics process.Wait for an external process. The returned
// channel is closed when the process exits. It works by polling the process
// with signal 0 and verifying no error is returned. A closer func is also
// returned that can be invoked to terminate the waiter early.
func externalWait(pid int, pollInterval time.Duration) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	stopCh := make(chan struct{})
	closer := func() {
		close(stopCh)
	}
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			if _, err := findProcess(pid); err != nil {
				close(ch)
				return
			}
			time.Sleep(pollInterval)
		}
	}()
	return ch, closer
}
