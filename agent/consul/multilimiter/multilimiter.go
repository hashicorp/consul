package multilimiter

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"

	radix "github.com/hashicorp/go-immutable-radix"
	"golang.org/x/time/rate"
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
//
//go:generate mockery --name RateLimiter --inpackage --filename mock_RateLimiter.go
type RateLimiter interface {
	Run(ctx context.Context)
	Allow(entity LimitedEntity) bool
	UpdateConfig(c LimiterConfig, prefix []byte)
}

type limiterWithKey struct {
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
	limiterCh       chan *limiterWithKey
	configsLock     sync.Mutex
	once            sync.Once
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
	m.configsLock.Lock()
	defer m.configsLock.Unlock()
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
	chLimiter := make(chan *limiterWithKey, 100)
	m := &MultiLimiter{limiters: &limiters, defaultConfig: &config, limitersConfigs: &configs, limiterCh: chLimiter}

	return m
}

// Run the cleanup routine to remove old entries of Limiters based on ReconcileCheckLimit and ReconcileCheckInterval.
func (m *MultiLimiter) Run(ctx context.Context) {
	m.once.Do(func() {
		go func() {
			cfg := m.defaultConfig.Load()
			writeTimeout := cfg.ReconcileCheckInterval
			limiters := m.limiters.Load()
			txn := limiters.Txn()
			waiter := time.NewTicker(writeTimeout)
			wt := tickerWrapper{ticker: waiter}

			defer waiter.Stop()
			for {
				if txn = m.reconcile(ctx, wt, txn, cfg.ReconcileCheckLimit); txn == nil {
					return
				}
			}
		}()
	})

}

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
	unixNow := time.Now().UnixMilli()
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
	m.limiterCh <- &limiterWithKey{l: limiter, k: e.Key(), t: now}
	return limiter.limiter.Allow()
}

type ticker interface {
	Ticker() <-chan time.Time
}

type tickerWrapper struct {
	ticker *time.Ticker
}

func (t tickerWrapper) Ticker() <-chan time.Time {
	return t.ticker.C
}

func (m *MultiLimiter) reconcile(ctx context.Context, waiter ticker, txn *radix.Txn, reconcileCheckLimit time.Duration) *radix.Txn {
	select {
	case <-waiter.Ticker():
		tree := txn.Commit()
		m.limiters.Store(tree)
		txn = tree.Txn()
		m.cleanLimiters(time.Now(), reconcileCheckLimit, txn)
		m.reconcileConfig(txn)
		tree = txn.Commit()
		txn = tree.Txn()
	case lk := <-m.limiterCh:
		v, ok := txn.Get(lk.k)
		if !ok {
			txn.Insert(lk.k, lk.l)
		} else {
			if l, ok := v.(*Limiter); ok {
				l.lastAccess.Store(lk.t.Unix())
				l.limiter.AllowN(lk.t, 1)
			}
		}
	case <-ctx.Done():
		return nil
	}
	return txn
}

func (m *MultiLimiter) reconcileConfig(txn *radix.Txn) {
	iter := txn.Root().Iterator()
	// make sure all limiters have the latest defaultConfig of their prefix
	for k, v, ok := iter.Next(); ok; k, v, ok = iter.Next() {
		pl, ok := v.(*Limiter)
		if pl == nil || !ok {
			continue
		}
		if pl.limiter == nil {
			continue
		}

		// find the prefix for the leaf and check if the defaultConfig is up-to-date
		// it's possible that the prefix is equal to the key
		prefix, _ := splitKey(k)
		v, ok := m.limitersConfigs.Load().Get(prefix)
		if v == nil || !ok {
			continue
		}
		cl, ok := v.(*LimiterConfig)
		if cl == nil || !ok {
			continue
		}
		if cl.isApplied(pl.limiter) {
			continue
		}

		limiter := Limiter{limiter: rate.NewLimiter(cl.Rate, cl.Burst)}
		limiter.lastAccess.Store(pl.lastAccess.Load())
		txn.Insert(k, &limiter)

	}
}

func (m *MultiLimiter) cleanLimiters(now time.Time, reconcileCheckLimit time.Duration, txn *radix.Txn) {
	iter := txn.Root().Iterator()
	// remove all expired limiters
	for k, v, ok := iter.Next(); ok; k, v, ok = iter.Next() {
		t, isLimiter := v.(*Limiter)
		if !isLimiter || t.limiter == nil {
			continue
		}

		lastAccess := time.UnixMilli(t.lastAccess.Load())
		if now.Sub(lastAccess) > reconcileCheckLimit {
			txn.Delete(k)
		}
	}

}

func (lc *LimiterConfig) isApplied(l *rate.Limiter) bool {
	return l.Limit() == lc.Rate && l.Burst() == lc.Burst
}
