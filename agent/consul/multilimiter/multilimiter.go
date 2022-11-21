package multilimiter

import (
	"context"
	radix "github.com/hashicorp/go-immutable-radix"
	"golang.org/x/time/rate"
	"sync/atomic"
	"time"
)

type LimitedEntity interface {
	Key() []byte
}

type RateLimiter interface {
	Start()
	Stop()
	Allow(entity LimitedEntity) bool
	UpdateConfig(c Config)
}

type Limiter struct {
	limiter    *rate.Limiter
	lastAccess atomic.Int64
}

type Config struct {
	Rate         rate.Limit
	Burst        int
	CleanupLimit time.Duration
	CleanupTick  time.Duration
}

type MultiLimiter struct {
	limiters *atomic.Pointer[radix.Tree]
	config   *atomic.Pointer[Config]
	cancel   context.CancelFunc
}

func (m *MultiLimiter) UpdateConfig(c Config) {
	m.config.CompareAndSwap(m.config.Load(), &c)
}

func NewMultiLimiter(c Config) *MultiLimiter {
	limiters := atomic.Pointer[radix.Tree]{}
	config := atomic.Pointer[Config]{}
	config.Store(&c)
	limiters.Store(radix.New())
	if c.CleanupLimit == 0 {
		c.CleanupLimit = 30 * time.Millisecond
	}
	if c.CleanupTick == 0 {
		c.CleanupLimit = 1 * time.Second
	}
	m := &MultiLimiter{limiters: &limiters, config: &config}
	return m
}

func (m *MultiLimiter) Start() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	m.cancel = cancelFunc
	go func() {
		for {
			m.cleanupLimited(ctx)
		}
	}()
}

func (m *MultiLimiter) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *MultiLimiter) Allow(e LimitedEntity) bool {
	limiters := m.limiters.Load()
	l, ok := limiters.Get(e.Key())
	now := time.Now().Unix()
	if ok {
		limiter := l.(*Limiter)
		limiter.lastAccess.Store(now)
		return limiter.limiter.Allow()
	}
	c := m.config.Load()
	limiter := &Limiter{limiter: rate.NewLimiter(c.Rate, c.Burst)}
	limiter.lastAccess.Store(now)
	tree, _, _ := limiters.Insert(e.Key(), limiter)
	m.limiters.Store(tree)

	return limiter.limiter.Allow()
}

// Every minute check the map for visitors that haven't been seen for
// more than the CleanupLimit and delete the entries.
func (m *MultiLimiter) cleanupLimited(ctx context.Context) {
	c := m.config.Load()
	waiter := time.After(c.CleanupTick)

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

			if diff > c.CleanupLimit {
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
