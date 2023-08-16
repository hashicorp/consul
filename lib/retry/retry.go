// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package retry

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Jitter should return a new wait duration optionally with some time added or
// removed to create some randomness in wait time.
type Jitter func(baseTime time.Duration) time.Duration

// NewJitter returns a new random Jitter that is up to percent longer than the
// original wait time.
func NewJitter(percent int64) Jitter {
	if percent < 0 {
		percent = 0
	}

	return func(baseTime time.Duration) time.Duration {
		if percent == 0 {
			return baseTime
		}
		max := (int64(baseTime) * percent) / 100
		if max < 0 { // overflow
			return baseTime
		}
		return baseTime + time.Duration(rand.Int63n(max))
	}
}

// Waiter records the number of failures and performs exponential backoff when
// there are consecutive failures.
type Waiter struct {
	// MinFailures before exponential backoff starts. Any failures before
	// MinFailures is reached will wait MinWait time.
	MinFailures uint
	// MinWait time. Returned after the first failure.
	MinWait time.Duration
	// MaxWait time applied before Jitter. Note that the actual maximum wait time
	// is MaxWait + MaxWait * Jitter.
	MaxWait time.Duration
	// Jitter to add to each wait time. The Jitter is applied after MaxWait, which
	// may cause the actual wait time to exceed MaxWait.
	Jitter Jitter
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
	// apply MaxWait before jitter so that multiple waiters with the same MaxWait
	// do not converge when they hit their max.
	if w.MaxWait != 0 && waitTime > w.MaxWait {
		waitTime = w.MaxWait
	}
	if w.Jitter != nil {
		waitTime = w.Jitter(waitTime)
	}
	if waitTime < w.MinWait {
		return w.MinWait
	}
	return waitTime
}

// Reset the failure count to 0.
// Reset must be called if the operation done after Wait did not fail.
func (w *Waiter) Reset() {
	w.failures = 0
}

// Failures returns the count of consecutive failures.
func (w *Waiter) Failures() int {
	return int(w.failures)
}

// Wait increases the number of failures by one, and then blocks until the context
// is cancelled, or until the wait time is reached.
//
// The wait time increases exponentially as the number of failures increases.
// Every call to Wait increments the failures count, so Reset must be called
// after Wait when there wasn't a failure.
//
// The only non-nil error that Wait returns will come from ctx.Err(),
// such as when the context is canceled. This makes it suitable for
// long-running routines that do not get re-initialized, such as replication.
func (w *Waiter) Wait(ctx context.Context) error {
	delay := w.WaitDuration()
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// WaitDuration increases the number of failures by one, and returns the
// duration the caller must wait for. This is an alternative to the Wait
// method for cases where you want to handle the timer yourself (e.g. as
// part of a larger select statement).
func (w *Waiter) WaitDuration() time.Duration {
	w.failures++
	return w.delay()
}

// NextWait returns the period the next call to Wait with block for assuming
// it's context is not cancelled. It's useful for informing a user how long
// it will be before the next attempt is made.
func (w *Waiter) NextWait() time.Duration {
	return w.delay()
}

// RetryLoop retries an operation until either operation completes without error
// or Waiter's context is canceled.
func (w *Waiter) RetryLoop(ctx context.Context, operation func() error) error {
	var lastError error
	for {
		if err := w.Wait(ctx); err != nil {
			// The error will only be non-nil if the context is canceled.
			return fmt.Errorf("could not retry operation: %w", lastError)
		}

		if err := operation(); err == nil {
			// Reset the failure count seen by the waiter if there was no error.
			w.Reset()
			return nil
		} else {
			lastError = err
		}
	}
}
