package controller

import (
	"math"
	"sync"
	"time"
)

// much of this is a re-implementation of:
// https://github.com/kubernetes/client-go/blob/release-1.25/util/workqueue/default_rate_limiters.go

// Limiter is an interface for a rate limiter that can limit
// the number of retries processed in the work queue.
type Limiter interface {
	// NextRetry returns the remaining time until the queue should
	// reprocess a Request.
	NextRetry(request Request) time.Duration
	// Forget causes the Limiter to reset the backoff for the Request.
	Forget(request Request)
}

var _ Limiter = &ratelimiter{}

type ratelimiter struct {
	failures map[Request]int
	base     time.Duration
	max      time.Duration
	mutex    sync.RWMutex
}

// NewRateLimiter returns a Limiter that does per-item exponential
// backoff.
func NewRateLimiter(base, max time.Duration) Limiter {
	return &ratelimiter{
		failures: make(map[Request]int),
		base:     base,
		max:      max,
	}
}

// NextRetry returns the remaining time until the queue should
// reprocess a Request.
func (r *ratelimiter) NextRetry(request Request) time.Duration {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	exponent := r.failures[request]
	r.failures[request] = r.failures[request] + 1

	backoff := float64(r.base.Nanoseconds()) * math.Pow(2, float64(exponent))
	// make sure we don't overflow time.Duration
	if backoff > math.MaxInt64 {
		return r.max
	}

	calculated := time.Duration(backoff)
	if calculated > r.max {
		return r.max
	}

	return calculated
}

// Forget causes the Limiter to reset the backoff for the Request.
func (r *ratelimiter) Forget(request Request) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.failures, request)
}
