package multilimiter

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-memdb"
	"golang.org/x/time/rate"
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
	lastAccess time.Time
	key        string
}

type Config struct {
	Rate         rate.Limit
	Burst        int
	CleanupLimit time.Duration
	CleanupTick  time.Duration
}

type MultiLimiter struct {
	db *memdb.MemDB
	// limiters *atomic.Pointer[radix.Tree]
	config *atomic.Pointer[Config]
	cancel context.CancelFunc
}

func (m *MultiLimiter) UpdateConfig(c Config) {
	m.config.CompareAndSwap(m.config.Load(), &c)
}

func NewMultiLimiter(c Config) *MultiLimiter {
	config := atomic.Pointer[Config]{}
	config.Store(&c)

	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"limiter": &memdb.TableSchema{
				Name: "limiter",
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "key"},
					},
				},
			},
		},
	}
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}

	if c.CleanupLimit == 0 {
		c.CleanupLimit = 30 * time.Millisecond
	}
	if c.CleanupTick == 0 {
		c.CleanupLimit = 1 * time.Second
	}
	m := &MultiLimiter{db: db, config: &config}
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
	txn := m.db.Txn(true)
	defer txn.Abort()
	raw, err := txn.First("limiter", "id", e.Key())
	now := time.Now()

	var l *Limiter
	key := string(e.Key())
	if err == nil {
		limiter := raw.(*Limiter)
		l = &Limiter{key: key, limiter: limiter.limiter}
	} else {
		c := m.config.Load()
		l = &Limiter{key: key, limiter: rate.NewLimiter(c.Rate, c.Burst)}
	}
	l.lastAccess = now
	err = txn.Insert("limiter", l)
	if err != nil {
		panic(err)
	}
	txn.Commit()

	return l.limiter.Allow()
}

// Every minute check the map for visitors that haven't been seen for
// more than 3 minutes and delete the entries.
func (m *MultiLimiter) cleanupLimited(ctx context.Context) {
	c := m.config.Load()
	waiter := time.After(c.CleanupTick)

	select {
	case <-ctx.Done():
		return
	case now := <-waiter:
		txn := m.db.Txn(false)
		defer txn.Abort()
		raw, err := txn.First("limiter", "id", "foo")

		var l *Limiter
		if err == nil {
			limiter := raw.(*Limiter)
			l = &Limiter{limiter: limiter.limiter}
		} else {
			c := m.config.Load()
			l = &Limiter{limiter: rate.NewLimiter(c.Rate, c.Burst)}
		}

		l.lastAccess = now
		// txn.Insert("limiter", l)
		// limiters := m.limiters.Load()
		// storedLimiters := limiters
		// iter := limiters.Root().Iterator()
		// k, v, ok := iter.Next()
		// var txn *radix.Txn
		// for ok {
		// 	limiter := v.(*Limiter)
		// 	lastAccess := limiter.lastAccess.Load()
		// 	lastAccessT := time.Unix(lastAccess, 0)
		// 	diff := now.Sub(lastAccessT)

		// 	if diff > c.CleanupLimit {
		// 		if txn == nil {
		// 			txn = limiters.Txn()
		// 		}
		// 		txn.Delete(k)
		// 	}
		// 	k, v, ok = iter.Next()
		// }
		// if txn != nil {
		// 	limiters = txn.Commit()

		// 	m.limiters.CompareAndSwap(storedLimiters, limiters)
		// }
	}
}
