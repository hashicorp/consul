package consul

import (
	"sync"

	"github.com/hashicorp/consul/lib/semaphore"
	"golang.org/x/time/rate"
)

type connectSignRateLimiter struct {
	// csrRateLimiter limits the rate of signing new certs if configured. Lazily
	// initialized from current config to support dynamic changes.
	// csrRateLimiterMu must be held while dereferencing the pointer or storing a
	// new one, but methods can be called on the limiter object outside of the
	// locked section. This is done only in the getCSRRateLimiterWithLimit method.
	csrRateLimiter   *rate.Limiter
	csrRateLimiterMu sync.RWMutex

	// csrConcurrencyLimiter is a dynamically resizable semaphore used to limit
	// Sign RPC concurrency if configured. The zero value is usable as soon as
	// SetSize is called which we do dynamically in the RPC handler to avoid
	// having to hook elaborate synchronization mechanisms through the CA config
	// endpoint and config reload etc.
	csrConcurrencyLimiter semaphore.Dynamic
}

// getCSRRateLimiterWithLimit returns a rate.Limiter with the desired limit set.
// It uses the shared server-wide limiter unless the limit has been changed in
// config or the limiter has not been setup yet in which case it just-in-time
// configures the new limiter. We assume that limit changes are relatively rare
// and that all callers (there is currently only one) use the same config value
// as the limit. There might be some flapping if there are multiple concurrent
// requests in flight at the time the config changes where A sees the new value
// and updates, B sees the old but then gets this lock second and changes back.
// Eventually though and very soon (once all current RPCs are complete) we are
// guaranteed to have the correct limit set by the next RPC that comes in so I
// assume this is fine. If we observe strange behavior because of it, we could
// add hysteresis that prevents changes too soon after a previous change but
// that seems unnecessary for now.
func (l *connectSignRateLimiter) getCSRRateLimiterWithLimit(limit rate.Limit) *rate.Limiter {
	l.csrRateLimiterMu.RLock()
	lim := l.csrRateLimiter
	l.csrRateLimiterMu.RUnlock()

	// If there is a current limiter with the same limit, return it. This should
	// be the common case.
	if lim != nil && lim.Limit() == limit {
		return lim
	}

	// Need to change limiter, get write lock
	l.csrRateLimiterMu.Lock()
	defer l.csrRateLimiterMu.Unlock()
	// No limiter yet, or limit changed in CA config, reconfigure a new limiter.
	// We use burst of 1 for a hard limit. Note that either bursting or waiting is
	// necessary to get expected behavior in fact of random arrival times, but we
	// don't need both and we use Wait with a small delay to smooth noise. See
	// https://github.com/banks/sim-rate-limit-backoff/blob/master/README.md.
	l.csrRateLimiter = rate.NewLimiter(limit, 1)
	return l.csrRateLimiter
}
