package lib

import (
	"time"
)

const (
	defaultMinFailures = 0
	defaultMaxWait     = 2 * time.Minute
)

// RetryWaiter will record failed and successful operations and provide
// a channel to wait on before a failed operation can be retried.
type RetryWaiter struct {
	minFailures uint
	maxWait     time.Duration
	failures    uint
}

// Creates a new RetryWaiter
func NewRetryWaiter(minFailures int, maxWait time.Duration) *RetryWaiter {
	if minFailures < 0 {
		minFailures = defaultMinFailures
	}

	if maxWait <= 0 {
		maxWait = defaultMaxWait
	}

	return &RetryWaiter{
		minFailures: uint(minFailures),
		maxWait:     maxWait,
		failures:    0,
	}
}

// Marks that an operation is successful which resets the failure count.
// The chan that is returned will be immediately selectable
func (rw *RetryWaiter) Success() <-chan time.Time {
	rw.Reset()
	c := make(chan time.Time, 1)
	c <- time.Now()
	return c
}

// Marks that an operation failed. The chan returned will be selectable
// once the calculated retry wait amount of time has elapsed
func (rw *RetryWaiter) Failed() <-chan time.Time {
	waitTime := 0 * time.Second
	if rw.failures > rw.minFailures {
		shift := rw.failures - rw.minFailures
		waitTime = rw.maxWait
		if shift < 31 {
			waitTime = (1 << shift) * time.Second
		}
		if waitTime > rw.maxWait {
			waitTime = rw.maxWait
		}

		// maybe add up to 10% extra
		waitTime += RandomStagger(waitTime / 10)
	}

	// incrementing the failures after the waitTime calculation is
	// intentional. This means that the first failure beyond the minimum
	// failure threshold will cause a sleep of ~1s instead of ~2s.
	rw.failures += 1

	return time.After(waitTime)
}

func (rw *RetryWaiter) Reset() {
	rw.failures = 0
}

// Wait is a convenience method to call either Success or Failed based
// on a variable
func (rw *RetryWaiter) Wait(failure bool) <-chan time.Time {
	if failure {
		return rw.Failed()
	}
	return rw.Success()
}
