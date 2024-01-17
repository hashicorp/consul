// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

import "time"

// Timer repeats an operation for a given amount
// of time and waits between subsequent operations.
type Timer struct {
	Timeout time.Duration
	Wait    time.Duration

	// stop is the timeout deadline.
	// TODO: Next()?
	// Set on the first invocation of Next().
	stop time.Time
}

func (r *Timer) Continue() bool {
	if r.stop.IsZero() {
		r.stop = time.Now().Add(r.Timeout)
		return true
	}
	if time.Now().After(r.stop) {
		return false
	}
	time.Sleep(r.Wait)
	return true
}
