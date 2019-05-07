package lib

import (
	"time"
)

const (
	defaultMinFailures = 0
	defaultMaxWait     = 2 * time.Minute
)

// Interface used for offloading jitter calculations from the RetryWaiter
type Jitter interface {
	AddJitter(baseTime time.Duration) time.Duration
}

// Calculates a random jitter between 0 and up to a specific percentage of the baseTime
type JitterRandomStagger struct {
	// int64 because we are going to be doing math against an int64 to represent nanoseconds
	percent int64
}

// Creates a new JitterRandomStagger
func NewJitterRandomStagger(percent int) *JitterRandomStagger {
	if percent < 0 {
		percent = 0
	}

	return &JitterRandomStagger{
		percent: int64(percent),
	}
}

// Implments the Jitter interface
func (j *JitterRandomStagger) AddJitter(baseTime time.Duration) time.Duration {
	if j.percent == 0 {
		return baseTime
	}

	// time.Duration is actually a type alias for int64 which is why casting
	// to the duration type and then dividing works
	return baseTime + RandomStagger((baseTime*time.Duration(j.percent))/100)
}

// RetryWaiter will record failed and successful operations and provide
// a channel to wait on before a failed operation can be retried.
type RetryWaiter struct {
	minFailures uint
	minWait     time.Duration
	maxWait     time.Duration
	jitter      Jitter
	failures    uint
}

// Creates a new RetryWaiter
func NewRetryWaiter(minFailures int, minWait, maxWait time.Duration, jitter Jitter) *RetryWaiter {
	if minFailures < 0 {
		minFailures = defaultMinFailures
	}

	if maxWait <= 0 {
		maxWait = defaultMaxWait
	}

	if minWait <= 0 {
		minWait = 0 * time.Nanosecond
	}

	return &RetryWaiter{
		minFailures: uint(minFailures),
		minWait:     minWait,
		maxWait:     maxWait,
		failures:    0,
		jitter:      jitter,
	}
}

// calculates the necessary wait time before the
// next operation should be allowed.
func (rw *RetryWaiter) calculateWait() time.Duration {
	waitTime := rw.minWait
	if rw.failures > rw.minFailures {
		shift := rw.failures - rw.minFailures - 1
		waitTime = rw.maxWait
		if shift < 31 {
			waitTime = (1 << shift) * time.Second
		}
		if waitTime > rw.maxWait {
			waitTime = rw.maxWait
		}

		if rw.jitter != nil {
			waitTime = rw.jitter.AddJitter(waitTime)
		}
	}

	if waitTime < rw.minWait {
		waitTime = rw.minWait
	}

	return waitTime
}

// calculates the waitTime and returns a chan
// that will become selectable once that amount
// of time has elapsed.
func (rw *RetryWaiter) wait() <-chan struct{} {
	waitTime := rw.calculateWait()
	ch := make(chan struct{})
	if waitTime > 0 {
		time.AfterFunc(waitTime, func() { close(ch) })
	} else {
		// if there should be 0 wait time then we ensure
		// that the chan will be immediately selectable
		close(ch)
	}
	return ch
}

// Marks that an operation is successful which resets the failure count.
// The chan that is returned will be immediately selectable
func (rw *RetryWaiter) Success() <-chan struct{} {
	rw.Reset()
	return rw.wait()
}

// Marks that an operation failed. The chan returned will be selectable
// once the calculated retry wait amount of time has elapsed
func (rw *RetryWaiter) Failed() <-chan struct{} {
	rw.failures += 1
	ch := rw.wait()
	return ch
}

// Resets the internal failure counter
func (rw *RetryWaiter) Reset() {
	rw.failures = 0
}

// WaitIf is a convenice method to record whether the last
// operation was a success or failure and return a chan that
// will be selectablw when the next operation can be done.
func (rw *RetryWaiter) WaitIf(failure bool) <-chan struct{} {
	if failure {
		return rw.Failed()
	}
	return rw.Success()
}

// WaitIfErr is a convenience method to record whether the last
// operation was a success or failure based on whether the err
// is nil and then return a chan that will be selectable when
// the next operation can be done.
func (rw *RetryWaiter) WaitIfErr(err error) <-chan struct{} {
	return rw.WaitIf(err != nil)
}
