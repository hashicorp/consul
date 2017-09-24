package proxy

import (
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"
)

// Default CheckDown functions for use in the proxy plugin.
var checkDownFunc = func(upstream *staticUpstream) healthcheck.UpstreamHostDownFunc {
	return func(uh *healthcheck.UpstreamHost) bool {

		down := false

		uh.Lock()
		until := uh.OkUntil
		uh.Unlock()

		if !until.IsZero() && time.Now().After(until) {
			down = true
		}

		fails := atomic.LoadInt32(&uh.Fails)
		if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
			down = true
		}
		return down
	}
}
