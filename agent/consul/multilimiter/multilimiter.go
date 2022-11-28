package multilimiter

import (
	"bytes"
	"context"
	radix "github.com/hashicorp/go-immutable-radix"
	"golang.org/x/time/rate"
	"sync/atomic"
	"time"
)

var _ RateLimiter = &MultiLimiter{}

const separator = "♣"

func makeKey(keys ...[]byte) KeyType {
	return bytes.Join(keys, []byte(separator))
}

func Key(prefix, key []byte) KeyType {
	return makeKey(prefix, key)
}

// RateLimiter is the interface implemented by MultiLimiter
type RateLimiter interface {
	Run(ctx context.Context)
	Allow(entity LimitedEntity) bool
	UpdateConfig(c LimiterConfig, prefix []byte)
}

// MultiLimiter implement RateLimiter interface and represent a set of rate limiters
// specific to different LimitedEntities and queried by a LimitedEntities.Key()
type MultiLimiter struct {
	limiters *atomic.Pointer[radix.Tree]
	config   *atomic.Pointer[Config]
}

type KeyType = []byte

// LimitedEntity is an interface used by MultiLimiter.Allow to determine
// which rate limiter to use to allow the request
type LimitedEntity interface {
	Key() KeyType
}

// Limiter define a limiter to be part of the MultiLimiter structure
type Limiter struct {
	limiter    *rate.Limiter
	lastAccess atomic.Int64
	config     *atomic.Pointer[LimiterConfig]
}

// LimiterConfig is a Limiter configuration
type LimiterConfig struct {
	Rate  rate.Limit
	Burst int
}

// Config is a MultiLimiter configuration
type Config struct {
	LimiterConfig
	ReconcileCheckLimit    time.Duration
	ReconcileCheckInterval time.Duration
}

// UpdateConfig will update the MultiLimiter Config
// which will cascade to all the Limiter(s) LimiterConfig
func (m *MultiLimiter) UpdateConfig(c LimiterConfig, prefix []byte) {

	if prefix == nil {
		prefix = []byte("")
	}
	limiters := m.limiters.Load()
	l, ok := limiters.Get(prefix)
	if ok {
		limiter := l.(*Limiter)
		if limiter.config != nil {
			limiter.config.Store(&c)
			return
		}
	}

	config := atomic.Pointer[LimiterConfig]{}
	config.Store(&c)
	newLimiters, _, _ := limiters.Insert(prefix, &Limiter{config: &config})
	m.limiters.Store(newLimiters)
	return
}

// NewMultiLimiter create a new MultiLimiter
func NewMultiLimiter(c Config) *MultiLimiter {
	limiters := atomic.Pointer[radix.Tree]{}
	config := atomic.Pointer[Config]{}
	config.Store(&c)
	limiters.Store(radix.New())
	if c.ReconcileCheckLimit == 0 {
		c.ReconcileCheckLimit = 30 * time.Millisecond
	}
	if c.ReconcileCheckInterval == 0 {
		c.ReconcileCheckLimit = 1 * time.Second
	}
	m := &MultiLimiter{limiters: &limiters, config: &config}
	return m
}

// Run the cleanup routine to remove old entries of Limiters based on ReconcileCheckLimit and ReconcileCheckInterval.
func (m *MultiLimiter) Run(ctx context.Context) {
	go func() {
		for {
			m.reconcileLimitedOnce(ctx)
		}
	}()
}

// todo: split without converting to a string
func splitKey(key []byte) ([]byte, []byte) {

	ret := bytes.SplitN(key, []byte(separator), 2)
	if len(ret) != 2 {
		return []byte(""), []byte("")
	}
	return ret[0], ret[1]
}

// Allow should be called by a request processor to check if the current request is Limited
// The request processor should provide a LimitedEntity that implement the right Key()
func (m *MultiLimiter) Allow(e LimitedEntity) bool {
	prefix, _ := splitKey(e.Key())
	limiters := m.limiters.Load()
	l, ok := limiters.Get(e.Key())
	now := time.Now().Unix()
	if ok {
		limiter := l.(*Limiter)
		if limiter.limiter != nil {
			limiter.lastAccess.Store(now)
			return limiter.limiter.Allow()
		}
	}
	c, okP := limiters.Get(prefix)
	var prefixLimiter *Limiter
	var config = &m.config.Load().LimiterConfig
	if okP {
		prefixLimiter = c.(*Limiter)
		if prefixLimiter.config != nil {
			config = prefixLimiter.config.Load()
		}
	}

	limiter := &Limiter{limiter: rate.NewLimiter(config.Rate, config.Burst)}
	limiter.lastAccess.Store(now)
	tree, _, _ := limiters.Insert(e.Key(), limiter)
	m.limiters.Store(tree)
	return limiter.limiter.Allow()
}

// reconcileLimitedOnce is called by the MultiLimiter clean up routine to remove old Limited entries
// it will wait for ReconcileCheckInterval before traversing the radix tree and removing all entries
// with lastAccess > ReconcileCheckLimit
func (m *MultiLimiter) reconcileLimitedOnce(ctx context.Context) {
	c := m.config.Load()
	waiter := time.NewTimer(c.ReconcileCheckInterval)
	defer waiter.Stop()
	select {
	case <-ctx.Done():
		return
	case now := <-waiter.C:
		limiters := m.limiters.Load()
		storedLimiters := limiters
		iter := limiters.Root().Iterator()
		k, v, ok := iter.Next()
		var txn *radix.Txn
		txn = limiters.Txn()
		// remove all expired limiters
		for ok {
			switch t := v.(type) {
			case *Limiter:
				if t.limiter != nil {
					lastAccess := t.lastAccess.Load()
					lastAccessT := time.Unix(lastAccess, 0)
					diff := now.Sub(lastAccessT)

					if diff > c.ReconcileCheckLimit {
						txn.Delete(k)
					}
				}
			}
			k, v, ok = iter.Next()
		}
		iter = txn.Root().Iterator()
		k, v, ok = iter.Next()

		// make sure all limiters have the latest config of their prefix
		for ok {
			switch pl := v.(type) {
			case *Limiter:
				// check if it has a limiter, if so that's a lead
				if pl.limiter != nil {
					// find the prefix for the leaf and check if the config is up-to-date
					// it's possible that this end up being the same node
					prefix, _ := splitKey(k)
					v, ok := txn.Get(prefix)
					if ok {
						switch cl := v.(type) {
						case *Limiter:
							if cl.config != nil {
								clConfig := cl.config.Load()
								if !clConfig.isApplied(pl.limiter) {
									limiter := Limiter{limiter: rate.NewLimiter(clConfig.Rate, clConfig.Burst)}
									limiter.lastAccess.Store(pl.lastAccess.Load())
									txn.Insert(k, &limiter)
								}
							}
						}
					}
				}
			}
			k, v, ok = iter.Next()
		}
		limiters = txn.Commit()
		m.limiters.CompareAndSwap(storedLimiters, limiters)
	}
}

func (lc *LimiterConfig) isApplied(l *rate.Limiter) bool {
	return l.Limit() == lc.Rate && l.Burst() == lc.Burst
}
