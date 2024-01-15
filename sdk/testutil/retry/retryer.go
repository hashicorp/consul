// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

import "time"

// Retryer provides an interface for repeating operations
// until they succeed or an exit condition is met.
type Retryer interface {
	// Continue returns true if the operation should be repeated, otherwise it
	// returns false to indicate retrying should stop.
	Continue() bool
}

// DefaultRetryer provides default retry.Run() behavior for unit tests, namely
// 7s timeout with a wait of 25ms
func DefaultRetryer() Retryer {
	return &Timer{Timeout: 7 * time.Second, Wait: 25 * time.Millisecond}
}

// ThirtySeconds repeats an operation for thirty seconds and waits 500ms in between.
// Best for known slower operations like waiting on eventually consistent state.
func ThirtySeconds() *Timer {
	return &Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}
}

// TwoSeconds repeats an operation for two seconds and waits 25ms in between.
func TwoSeconds() *Timer {
	return &Timer{Timeout: 2 * time.Second, Wait: 25 * time.Millisecond}
}

// ThreeTimes repeats an operation three times and waits 25ms in between.
func ThreeTimes() *Counter {
	return &Counter{Count: 3, Wait: 25 * time.Millisecond}
}
