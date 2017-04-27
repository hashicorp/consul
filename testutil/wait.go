package testutil

import (
	"fmt"
	"time"
)

const (
	wait    = 25 * time.Millisecond
	timeout = 5 * time.Second
)

func WaitForResult(f func() (bool, error)) error {
	stop := time.Now().Add(timeout)
	for {
		ok, err := f()
		if ok {
			return nil
		}
		if time.Now().After(stop) {
			return fmt.Errorf("timeout: %s", err)
		}
		time.Sleep(wait)
	}
}
