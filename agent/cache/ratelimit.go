package cache

import (
	"errors"
	"time"

	"github.com/hashicorp/consul/lib"
)

const (
	// RateLimiterHasBeenStopped is error returned when rate-limiter has been stopped
	RateLimiterHasBeenStopped = "RateLimiter is Stopped"
)

// RateLimiter is the interface to have a Rate-Limiter object
type RateLimiter interface {
	Take() error
	Stop() error
}

// fake RateLimiter when ticker would cost too much (too small delay)
type rateLimiterFake struct {
	stopped bool
}

// RateLimiterTicker let the user Control request at a given rate
type rateLimiterTicker struct {
	// The ticker
	ticker *time.Ticker
	// Stop channel
	stop  chan struct{}
	burst chan struct{}
}

// NewRateLimiter builds a new Rate Limiter
func NewRateLimiter(spec *lib.RateLimitSpec) RateLimiter {
	if spec == nil || spec.Period < 1 {
		return &rateLimiterFake{}
	}
	ticker := time.NewTicker(spec.Period)
	stop := make(chan struct{}, 1)
	burst := make(chan struct{}, spec.BurstSize)
	rateLimiter := &rateLimiterTicker{
		ticker,
		stop,
		burst,
	}
	rateLimiter.start()
	return rateLimiter
}

// Take wait until rate-limiter allows you to continue
func (r *rateLimiterTicker) Take() error {
	ticker := r.ticker
	if ticker == nil {
		return errors.New(RateLimiterHasBeenStopped)
	}
	for {
		select {
		case <-r.burst:
			return nil
		}
	}
}

// start is internal only as the ticker
// is started by default
func (r *rateLimiterTicker) start() {
	go func() {
		for {
			select {
			case <-r.stop:
				// Let watcher stop quickly
				ticker := r.ticker
				r.ticker = nil
				r.burst <- struct{}{}
				close(r.stop)
				ticker.Stop()
				close(r.burst)
				return
			case <-r.ticker.C:
				r.burst <- struct{}{}
			}
		}
	}()
}

// Stop and destroy the RateLimiter
func (r *rateLimiterTicker) Stop() error {
	tick := r.ticker
	if tick == nil {
		return errors.New(RateLimiterHasBeenStopped)
	}
	r.stop <- struct{}{}
	return nil
}

// Take is implementation of interface RateLimiter
func (r *rateLimiterFake) Take() error {
	if r.stopped {
		return errors.New(RateLimiterHasBeenStopped)
	}
	return nil
}

// Stop is implementation of interface RateLimiter
func (r *rateLimiterFake) Stop() error {
	if r.stopped {
		return errors.New(RateLimiterHasBeenStopped)
	}
	r.stopped = true
	return nil
}
