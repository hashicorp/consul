// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package utils

import (
	"context"
	"time"
)

// NOTE: copied from github.com/hashicorp/consul/lib/retry to avoid a heavy dependency
// NOTE2: Jitter was removed

// Waiter records the number of failures and performs exponential backoff when
// when there are consecutive failures.
type Waiter struct {
	// MinFailures before exponential backoff starts. Any failures before
	// MinFailures is reached will wait MinWait time.
	MinFailures uint
	// MinWait time. Returned after the first failure.
	MinWait time.Duration
	// MaxWait time.
	MaxWait time.Duration
	// Factor is the multiplier to use when calculating the delay. Defaults to
	// 1 second.
	Factor   time.Duration
	failures uint
}

// delay calculates the time to wait based on the number of failures
func (w *Waiter) delay() time.Duration {
	if w.failures <= w.MinFailures {
		return w.MinWait
	}
	factor := w.Factor
	if factor == 0 {
		factor = time.Second
	}

	shift := w.failures - w.MinFailures - 1
	waitTime := w.MaxWait
	if shift < 31 {
		waitTime = (1 << shift) * factor
	}
	if w.MaxWait != 0 && waitTime > w.MaxWait {
		waitTime = w.MaxWait
	}
	if waitTime < w.MinWait {
		return w.MinWait
	}
	return waitTime
}

// Reset the failure count to 0.
func (w *Waiter) Reset() {
	w.failures = 0
}

// Failures returns the count of consecutive failures.
func (w *Waiter) Failures() int {
	return int(w.failures)
}

// Wait increase the number of failures by one, and then blocks until the context
// is cancelled, or until the wait time is reached.
// The wait time increases exponentially as the number of failures increases.
// Wait will return ctx.Err() if the context is cancelled.
func (w *Waiter) Wait(ctx context.Context) error {
	w.failures++
	timer := time.NewTimer(w.delay())
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
