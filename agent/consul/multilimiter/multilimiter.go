package multilimiter

import (
	"context"
	radix "github.com/hashicorp/go-immutable-radix"
	"golang.org/x/time/rate"
	"sync/atomic"
	"time"
)

// LimitedEntity is an interface used by MultiLimiter.Allow to determine
// which rate limiter to use to allow the request
type LimitedEntity interface {
	Key() []byte
}

// RateLimiter is the interface implemented by MultiLimiter
type RateLimiter interface {
	Start()
	Stop()
	Allow(entity LimitedEntity) bool
	UpdateConfig(c Config)
}

// Limiter define a limiter to be part of the MultiLimiter structure
type Limiter struct {
	limiter    *rate.Limiter
	lastAccess atomic.Int64
	config     atomic.Pointer[LimiterConfig]
}

// LimiterConfig is a Limiter configuration
type LimiterConfig struct {
	Rate  rate.Limit
	Burst int
}

// Config is a MultiLimiter configuration
type Config struct {
	LimiterConfig
	CleanupCheckLimit    time.Duration
	CleanupCheckInterval time.Duration
}

func (c *Config) Equal(lc *LimiterConfig) bool {
	return c.Burst == lc.Burst && c.Rate == lc.Rate
}

// MultiLimiter implement RateLimiter interface and represent a set of rate limiters
// specific to different LimitedEntities and queried by a LimitedEntities.Key()
type MultiLimiter struct {
	limiters *atomic.Pointer[radix.Tree]
	config   *atomic.Pointer[Config]
	cancel   context.CancelFunc
}

// UpdateConfig will update the MultiLimiter Config
// which will cascade to all the Limiter(s) LimiterConfig
func (m *MultiLimiter) UpdateConfig(c Config) {
	m.config.CompareAndSwap(m.config.Load(), &c)
}

// NewMultiLimiter create a new MultiLimiter
func NewMultiLimiter(c Config) *MultiLimiter {
	limiters := atomic.Pointer[radix.Tree]{}
	config := atomic.Pointer[Config]{}
	config.Store(&c)
	limiters.Store(radix.New())
	if c.CleanupCheckLimit == 0 {
		c.CleanupCheckLimit = 30 * time.Millisecond
	}
	if c.CleanupCheckInterval == 0 {
		c.CleanupCheckLimit = 1 * time.Second
	}
	m := &MultiLimiter{limiters: &limiters, config: &config}
	return m
}

// Run the cleanup routine to remove old entries of Limiters based on CleanupCheckLimit and CleanupCheckInterval.
func (m *MultiLimiter) Run(ctx context.Context) {
	for {
		m.cleanupLimitedOnce(ctx)
	}
}

// Allow should be called by a request processor to check if the current request is Limited
// The request processor should provide a LimitedEntity that implement the right Key()
func (m *MultiLimiter) Allow(e LimitedEntity) bool {
	limiters := m.limiters.Load()
	l, ok := limiters.Get(e.Key())
	now := time.Now().Unix()
	c := m.config.Load()
	if ok {
		limiter := l.(*Limiter)
		if c.Equal(limiter.config.Load()) {
			limiter.lastAccess.Store(now)
			return limiter.limiter.Allow()
		}
	}

	limiter := &Limiter{limiter: rate.NewLimiter(c.Rate, c.Burst), config: atomic.Pointer[LimiterConfig]{}}
	limiter.config.Store(&c.LimiterConfig)
	limiter.lastAccess.Store(now)
	tree, _, _ := limiters.Insert(e.Key(), limiter)
	m.limiters.Store(tree)

	return limiter.limiter.Allow()
}

// cleanupLimitedOnce is called by the MultiLimiter clean up routine to remove old Limited entries
// it will wait for CleanupCheckInterval before traversing the radix tree and removing all entries
// with lastAccess > CleanupCheckLimit
func (m *MultiLimiter) cleanupLimitedOnce(ctx context.Context) {
	c := m.config.Load()
	waiter := time.After(c.CleanupCheckInterval)

	select {
	case <-ctx.Done():
		return
	case now := <-waiter:
		limiters := m.limiters.Load()
		storedLimiters := limiters
		iter := limiters.Root().Iterator()
		k, v, ok := iter.Next()
		var txn *radix.Txn
		for ok {
			limiter := v.(*Limiter)
			lastAccess := limiter.lastAccess.Load()
			lastAccessT := time.Unix(lastAccess, 0)
			diff := now.Sub(lastAccessT)

			if diff > c.CleanupCheckLimit {
				if txn == nil {
					txn = limiters.Txn()
				}
				txn.Delete(k)
			}
			k, v, ok = iter.Next()
		}
		if txn != nil {
			limiters = txn.Commit()

			m.limiters.CompareAndSwap(storedLimiters, limiters)
		}
	}
}
