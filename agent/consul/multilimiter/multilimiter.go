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

type LimiterWithKey struct {
	l *Limiter
	k []byte
	t time.Time
}

// MultiLimiter implement RateLimiter interface and represent a set of rate limiters
// specific to different LimitedEntities and queried by a LimitedEntities.Key()
type MultiLimiter struct {
	limiters        *atomic.Pointer[radix.Tree]
	limitersConfigs *atomic.Pointer[radix.Tree]
	defaultConfig   *atomic.Pointer[Config]
	LimiterCh       chan *LimiterWithKey
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
	configs := m.limitersConfigs.Load()
	newConfigs, _, _ := configs.Insert(prefix, &c)
	m.limitersConfigs.Store(newConfigs)
}

// NewMultiLimiter create a new MultiLimiter
func NewMultiLimiter(c Config) *MultiLimiter {
	limiters := atomic.Pointer[radix.Tree]{}
	configs := atomic.Pointer[radix.Tree]{}
	config := atomic.Pointer[Config]{}
	config.Store(&c)
	limiters.Store(radix.New())
	configs.Store(radix.New())
	if c.ReconcileCheckLimit == 0 {
		c.ReconcileCheckLimit = 30 * time.Millisecond
	}
	if c.ReconcileCheckInterval == 0 {
		c.ReconcileCheckLimit = 1 * time.Second
	}
	chLimiter := make(chan *LimiterWithKey, 100)
	m := &MultiLimiter{limiters: &limiters, defaultConfig: &config, limitersConfigs: &configs, LimiterCh: chLimiter}

	return m
}

// Run the cleanup routine to remove old entries of Limiters based on ReconcileCheckLimit and ReconcileCheckInterval.
func (m *MultiLimiter) Run(ctx context.Context) {
	go m.runStore(ctx)
	go func() {
		waiter := time.NewTimer(0)
		for {
			c := m.defaultConfig.Load()
			waiter.Reset(c.ReconcileCheckInterval)
			select {
			case <-ctx.Done():
				waiter.Stop()
				return
			case now := <-waiter.C:
				m.reconcileLimitedOnce(now, c.ReconcileCheckLimit)
			}
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
	now := time.Now()
	unixNow := now.Unix()
	if ok {
		limiter := l.(*Limiter)
		if limiter.limiter != nil {
			limiter.lastAccess.Store(unixNow)
			return limiter.limiter.Allow()
		}
	}

	configs := m.limitersConfigs.Load()
	c, okP := configs.Get(prefix)
	var config = &m.defaultConfig.Load().LimiterConfig
	if okP {
		prefixConfig := c.(*LimiterConfig)
		if prefixConfig != nil {
			config = prefixConfig
		}
	}
	limiter := &Limiter{limiter: rate.NewLimiter(config.Rate, config.Burst)}
	limiter.lastAccess.Store(unixNow)
	m.LimiterCh <- &LimiterWithKey{l: limiter, k: e.Key(), t: now}
	return limiter.limiter.Allow()
}

func (m *MultiLimiter) runStore(ctx context.Context) {
	var txn *radix.Txn
	waiter := time.NewTimer(10 * time.Millisecond)
	defer waiter.Stop()
	for {
		select {
		case <-waiter.C:
			if txn != nil {
				tree := txn.Commit()
				m.limiters.Store(tree)
			}
		case lk := <-m.LimiterCh:
			limiters := m.limiters.Load()
			if txn == nil {
				txn = limiters.Txn()
			}
			v, ok := txn.Get(lk.k)
			if !ok {
				txn.Insert(lk.k, lk.l)
			} else {
				switch l := v.(type) {
				case *Limiter:
					l.lastAccess.Store(lk.t.Unix())
					l.limiter.AllowN(lk.t, 1)
				}
			}
		case <-ctx.Done():
			return
		}

	}
}

// reconcileLimitedOnce is called by the MultiLimiter clean up routine to remove old Limited entries
// it will wait for ReconcileCheckInterval before traversing the radix tree and removing all entries
// with lastAccess > ReconcileCheckLimit
func (m *MultiLimiter) reconcileLimitedOnce(now time.Time, reconcileCheckLimit time.Duration) {
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

				if diff > reconcileCheckLimit {
					txn.Delete(k)
				}
			}
		}
		k, v, ok = iter.Next()
	}
	iter = txn.Root().Iterator()
	k, v, ok = iter.Next()

	// make sure all limiters have the latest defaultConfig of their prefix
	for ok {
		switch pl := v.(type) {
		case *Limiter:
			// check if it has a limiter, if so that's a lead
			if pl.limiter != nil {
				// find the prefix for the leaf and check if the defaultConfig is up-to-date
				// it's possible that the prefix is equal to the key
				prefix, _ := splitKey(k)
				v, ok := m.limitersConfigs.Load().Get(prefix)
				if ok {
					switch cl := v.(type) {
					case *LimiterConfig:
						if cl != nil {
							if !cl.isApplied(pl.limiter) {
								limiter := Limiter{limiter: rate.NewLimiter(cl.Rate, cl.Burst)}
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

func (lc *LimiterConfig) isApplied(l *rate.Limiter) bool {
	return l.Limit() == lc.Rate && l.Burst() == lc.Burst
}
