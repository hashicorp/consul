// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package multilimiter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	radix "github.com/hashicorp/go-immutable-radix"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type Limited struct {
	key KeyType
}

func (l Limited) Key() []byte {
	return l.key
}

func TestNewMultiLimiter(t *testing.T) {
	c := Config{}
	m := NewMultiLimiter(c)
	require.NotNil(t, m)
	require.NotNil(t, m.limiters)
}

func TestRateLimiterUpdate(t *testing.T) {
	c := Config{ReconcileCheckLimit: 1 * time.Hour, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	key := Key([]byte("test"))

	c1 := LimiterConfig{Rate: 10}
	m.UpdateConfig(c1, key)
	//Allow a key
	m.Allow(Limited{key: key})
	storeLimiter(m)
	limiters := m.limiters.Load()
	l1, ok1 := limiters.Get(key)
	retry.RunWith(&retry.Timer{Wait: 100 * time.Millisecond, Timeout: 2 * time.Second}, t, func(r *retry.R) {
		limiters = m.limiters.Load()
		l1, ok1 = limiters.Get(key)
		// check key exist
		require.True(r, ok1)
		require.NotNil(r, l1)
	})

	la1 := l1.(*Limiter).lastAccess.Load()

	// Sleep a bit just to make sure time change
	time.Sleep(time.Millisecond)
	// allow same key again
	m.Allow(Limited{key: key})
	limiters = m.limiters.Load()
	l2, ok2 := limiters.Get(key)

	// check it exist and it's same key
	require.True(t, ok2)
	require.NotNil(t, l2)
	require.Equal(t, l1, l2)

	// last access should be different
	la2 := l1.(*Limiter).lastAccess.Load()
	require.NotEqual(t, la1, la2)

}

func TestRateLimiterCleanup(t *testing.T) {

	// Create a limiter and Allow a key, check the key exists
	c := Config{ReconcileCheckLimit: 1 * time.Second, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	limiters := m.limiters.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	key := Key([]byte("test"))
	m.UpdateConfig(LimiterConfig{Rate: 0.1}, key)
	m.Allow(Limited{key: key})
	retry.RunWith(&retry.Timer{Wait: 100 * time.Millisecond, Timeout: 10 * time.Second}, t, func(r *retry.R) {
		l := m.limiters.Load()
		require.NotEqual(r, limiters, l)
		limiters = l
	})

	retry.RunWith(&retry.Timer{Wait: 100 * time.Millisecond, Timeout: 10 * time.Second}, t, func(r *retry.R) {
		v, ok := limiters.Get(key)
		require.True(r, ok)
		require.NotNil(r, v)
	})

	time.Sleep(c.ReconcileCheckInterval)
	// Wait > ReconcileCheckInterval and check that the key was cleaned up
	retry.RunWith(&retry.Timer{Wait: 100 * time.Millisecond, Timeout: 2 * time.Second}, t, func(r *retry.R) {
		l := m.limiters.Load()
		require.NotEqual(r, limiters, l)
		limiters = l
		v, ok := limiters.Get(key)
		require.False(r, ok)
		require.Nil(r, v)
	})

}

func storeLimiter(m *MultiLimiter) {
	txn := m.limiters.Load().Txn()
	mockTicker := mockTicker{tickerCh: make(chan time.Time, 1)}
	ctx := context.Background()
	reconcileCheckLimit := m.defaultConfig.Load().ReconcileCheckLimit
	m.reconcile(ctx, &mockTicker, txn, reconcileCheckLimit)
	mockTicker.tickerCh <- time.Now()
	m.reconcile(ctx, &mockTicker, txn, reconcileCheckLimit)
}

func reconcile(m *MultiLimiter) {
	txn := m.limiters.Load().Txn()
	mockTicker := mockTicker{tickerCh: make(chan time.Time, 1)}
	ctx := context.Background()
	reconcileCheckLimit := m.defaultConfig.Load().ReconcileCheckLimit
	mockTicker.tickerCh <- time.Now()
	txn = m.reconcile(ctx, &mockTicker, txn, reconcileCheckLimit)
	m.limiters.Store(txn.Commit())
}

func TestRateLimiterStore(t *testing.T) {
	// Create a MultiLimiter m with a defaultConfig c and check the defaultConfig is applied

	t.Run("Store multiple transactions", func(t *testing.T) {
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)
		ipNoPrefix1 := Key([]byte(""), []byte("127.0.0.1"))
		c1 := LimiterConfig{Rate: 1}
		ipNoPrefix2 := Key([]byte(""), []byte("127.0.0.2"))
		c2 := LimiterConfig{Rate: 2}

		{
			// Update config for ipNoPrefix1 and check it's applied
			m.UpdateConfig(c1, ipNoPrefix1)
			m.Allow(ipLimited{key: ipNoPrefix1})
			storeLimiter(m)
			l, ok := m.limiters.Load().Get(ipNoPrefix1)
			require.True(t, ok)
			require.NotNil(t, l)
			limiter := l.(*Limiter)
			require.True(t, c1.isApplied(limiter.limiter))
		}

		{
			// Update config for ipNoPrefix2 and check it's applied
			m.UpdateConfig(c2, ipNoPrefix2)
			m.Allow(ipLimited{key: ipNoPrefix2})
			storeLimiter(m)
			l, ok := m.limiters.Load().Get(ipNoPrefix2)
			require.True(t, ok)
			require.NotNil(t, l)
			limiter := l.(*Limiter)
			require.True(t, c2.isApplied(limiter.limiter))

			//Check that ipNoPrefix1 is unchanged
			l, ok = m.limiters.Load().Get(ipNoPrefix1)
			require.True(t, ok)
			require.NotNil(t, l)
			limiter = l.(*Limiter)
			require.True(t, c1.isApplied(limiter.limiter))
		}
	})
	t.Run("runStore store multiple Limiters", func(t *testing.T) {
		c := Config{ReconcileCheckLimit: 10 * time.Second, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)
		ctx, cancel := context.WithCancel(context.Background())
		m.Run(ctx)
		defer cancel()

		// Create a limiter for ipNoPrefix1
		ipNoPrefix1 := Key([]byte(""), []byte("127.0.0.1"))
		c1 := LimiterConfig{Rate: 1}
		limiters := m.limiters.Load()
		m.UpdateConfig(c1, ipNoPrefix1)
		m.Allow(ipLimited{key: ipNoPrefix1})
		retry.RunWith(&retry.Timer{Wait: 1 * time.Second, Timeout: 5 * time.Second}, t, func(r *retry.R) {
			l := m.limiters.Load()
			require.NotEqual(r, limiters, l)
			limiters = l
		})

		// Check that ipNoPrefix1 have the expected limiter
		l, ok := m.limiters.Load().Get(ipNoPrefix1)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))

		// Create a limiter for ipNoPrefix2
		ipNoPrefix2 := Key([]byte(""), []byte("127.0.0.2"))
		c2 := LimiterConfig{Rate: 2}
		m.UpdateConfig(c2, ipNoPrefix2)
		m.Allow(ipLimited{key: ipNoPrefix2})
		retry.RunWith(&retry.Timer{Wait: 1 * time.Second, Timeout: 5 * time.Second}, t, func(r *retry.R) {
			l := m.limiters.Load()
			require.NotEqual(r, limiters, l)
			limiters = l
		})

		// Check that ipNoPrefix1 have the expected limiter
		l, ok = m.limiters.Load().Get(ipNoPrefix1)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter = l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))

		// Check that ipNoPrefix2 have the expected limiter
		l, ok = m.limiters.Load().Get(ipNoPrefix2)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter = l.(*Limiter)
		require.True(t, c2.isApplied(limiter.limiter))
	})

}

func TestRateLimiterUpdateConfig(t *testing.T) {

	// Create a MultiLimiter m with a defaultConfig c and check the defaultConfig is applied

	t.Run("Allow a key and check defaultConfig is applied to that key", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ipNoPrefix
		ipNoPrefix := Key([]byte(""), []byte("127.0.0.1"))
		c1 := LimiterConfig{Rate: 1}
		m.UpdateConfig(c1, ipNoPrefix)
		m.Allow(ipLimited{key: ipNoPrefix})
		storeLimiter(m)

		// Verify the expected limiter is applied
		l, ok := m.limiters.Load().Get(ipNoPrefix)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})

	t.Run("Update nil prefix and make sure it's written in the root", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for nil
		prefix := []byte(nil)
		c1 := LimiterConfig{Rate: 1}
		m.UpdateConfig(c1, prefix)

		// Verify the expected limiter is applied
		v, ok := m.limitersConfigs.Load().Get([]byte(""))
		require.True(t, ok)
		require.NotNil(t, v)
		c2 := v.(*LimiterConfig)
		require.Equal(t, c1, *c2)
	})

	t.Run("Allow 2 keys with prefix and check defaultConfig is applied to those keys", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		prefix := []byte("namespace.write")
		ip := Key(prefix, []byte("127.0.0.1"))
		c1 := LimiterConfig{Rate: 1}
		m.UpdateConfig(c1, ip)
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Create a limiter for ip2
		ip2 := Key(prefix, []byte("127.0.0.2"))
		c2 := LimiterConfig{Rate: 2}
		m.UpdateConfig(c2, ip2)
		m.Allow(ipLimited{key: ip2})

		//Verify the config is applied for ip
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})
	t.Run("Apply a defaultConfig to 'namespace.write' check the defaultConfig is applied to existing keys under that prefix", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		prefix := []byte("namespace.write")
		ip := Key(prefix, []byte("127.0.0.1"))
		c3 := LimiterConfig{Rate: 2}
		m.UpdateConfig(c3, prefix)
		// call reconcileLimitedOnce to make sure the update is applied
		m.reconcileConfig(m.limiters.Load().Txn())
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Verify the config is applied for ip
		l3, ok3 := m.limiters.Load().Get(ip)
		require.True(t, ok3)
		require.NotNil(t, l3)
		limiter3 := l3.(*Limiter)
		require.True(t, c3.isApplied(limiter3.limiter))
	})
	t.Run("Allow an IP with prefix and check prefix defaultConfig is applied to new keys under that prefix", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		c1 := LimiterConfig{Rate: 3}
		prefix := []byte("namespace.read")
		m.UpdateConfig(c1, prefix)
		ip := Key(prefix, []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Verify the config is applied for ip
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})

	t.Run("Allow an IP with prefix and check prefix defaultConfig is applied to new keys under that prefix, delete config and check default applied", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Second, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		c1 := LimiterConfig{Rate: 3}
		prefix := []byte("namespace.read")
		m.UpdateConfig(c1, prefix)
		ip := Key(prefix, []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Verify the config is applied for ip
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))

		// Delete the prefix
		m.DeleteConfig(prefix)
		reconcile(m)

		// Verify the limiter is removed
		_, ok = m.limiters.Load().Get(ip)
		require.False(t, ok)
	})
	t.Run("Allow an IP with prefix and check prefix config is applied to new keys under that prefix", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		c1 := LimiterConfig{Rate: 3}
		prefix := Key([]byte("ip.ratelimit"), []byte("127.0"))
		m.UpdateConfig(c1, prefix)
		ip := Key([]byte("ip.ratelimit"), []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Verify the config is applied for ip
		load := m.limiters.Load()
		l, ok := load.Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})

	t.Run("Allow an IP with 2 prefixes and check prefix config is applied to new keys under that prefix", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for "127.0" ip with config c1
		c1 := LimiterConfig{Rate: 3}
		prefix := Key([]byte("ip.ratelimit"), []byte("127.0"))
		m.UpdateConfig(c1, prefix)

		//Create a limiter for "127.0.0" ip with config c2
		prefix = Key([]byte("ip.ratelimit"), []byte("127.0.0"))
		c2 := LimiterConfig{Rate: 6}
		m.UpdateConfig(c2, prefix)
		ip := Key([]byte("ip.ratelimit"), []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		// Verify that "127.0.0.1" have the right limiter config
		load := m.limiters.Load()
		l, ok := load.Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c2.isApplied(limiter.limiter))

		//Create a limiter for "127.0.1.1" ip with config c2
		ip = Key([]byte("ip.ratelimit"), []byte("127.0.1.1"))
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		// Verify that "127.0.1.1" have the right limiter config
		load = m.limiters.Load()
		l, ok = load.Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter = l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})

	t.Run("Allow an IP with prefix and check after it's cleaned new Allow would give it the right defaultConfig", func(t *testing.T) {
		//Create a multilimiter
		c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(c)
		require.Equal(t, *m.defaultConfig.Load(), c)

		//Create a limiter for ip
		prefix := []byte("namespace.read")
		ip := Key(prefix, []byte("127.0.0.1"))
		c1 := LimiterConfig{Rate: 1}
		m.UpdateConfig(c1, prefix)
		// call reconcileLimitedOnce to make sure the update is applied
		reconcile(m)
		m.Allow(ipLimited{key: ip})
		storeLimiter(m)

		//Verify the config is applied for ip
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c1.isApplied(limiter.limiter))
	})
}

func FuzzSingleConfig(f *testing.F) {
	c := Config{ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	require.Equal(f, *m.defaultConfig.Load(), c)
	f.Add(Key(randIP()))
	f.Add(Key(randIP(), randIP()))
	f.Add(Key(randIP(), randIP(), randIP()))
	f.Add(Key(randIP(), randIP(), randIP(), randIP()))
	c1 := LimiterConfig{
		Rate:  100,
		Burst: 123,
	}
	f.Fuzz(func(t *testing.T, ff []byte) {
		m.UpdateConfig(c1, ff)
		m.Allow(Limited{key: ff})
		storeLimiter(m)
		checkLimiter(t, ff, m.limiters.Load().Txn())
		checkTree(t, m.limiters.Load().Txn())
	})
}

func FuzzSplitKey(f *testing.F) {
	f.Add(Key(randIP(), randIP()))
	f.Add(Key(randIP(), randIP(), randIP()))
	f.Add(Key(randIP(), randIP(), randIP(), randIP()))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, ff []byte) {
		prefix, suffix := splitKey(ff)
		require.NotNil(t, prefix)
		require.NotNil(t, suffix)
		if len(prefix) == 0 && len(suffix) == 0 {
			return
		}
		joined := bytes.Join([][]byte{prefix, suffix}, []byte(separator))
		require.Equal(t, ff, joined)
		require.False(t, bytes.Contains(prefix, []byte(separator)))
	})
}

func checkLimiter(t require.TestingT, ff []byte, Tree *radix.Txn) {
	v, ok := Tree.Get(ff)
	require.True(t, ok)
	require.NotNil(t, v)
}

func FuzzUpdateConfig(f *testing.F) {

	f.Add(bytes.Join([][]byte{[]byte(""), Key(randIP()), Key(randIP(), randIP()), Key(randIP(), randIP(), randIP()), Key(randIP(), randIP(), randIP(), randIP())}, []byte(",")))
	f.Fuzz(func(t *testing.T, ff []byte) {
		cm := Config{ReconcileCheckLimit: 1 * time.Millisecond, ReconcileCheckInterval: 1 * time.Millisecond}
		m := NewMultiLimiter(cm)
		ctx, cancel := context.WithCancel(context.Background())
		m.Run(ctx)
		defer cancel()
		keys := bytes.Split(ff, []byte(","))
		for _, f := range keys {
			prefix, _ := splitKey(f)
			c := LimiterConfig{Rate: rate.Limit(rand.Float64()), Burst: rand.Int()}
			m.UpdateConfig(c, prefix)
			go m.Allow(Limited{key: f})
		}
		m.reconcileConfig(m.limiters.Load().Txn())
		checkTree(t, m.limiters.Load().Txn())
	})

}

func checkTree(t require.TestingT, tree *radix.Txn) {
	iterator := tree.Root().Iterator()
	kp, v, ok := iterator.Next()
	for ok {
		switch c := v.(type) {
		case *Limiter:
			if c.limiter != nil {
				prefix, _ := splitKey(kp)
				v, _ := tree.Get(prefix)
				switch c2 := v.(type) {
				case *LimiterConfig:
					if c2 != nil {
						applied := c2.isApplied(c.limiter)
						require.True(t, applied, fmt.Sprintf("%v,%v", kp, prefix))
					}

				}
			}
		default:
			require.Nil(t, v)
		}
		kp, v, ok = iterator.Next()
	}
}

type ipLimited struct {
	key []byte
}

func (i ipLimited) Key() []byte {
	return i.key
}

func BenchmarkTestRateLimiterFixedIP(b *testing.B) {
	var Config = Config{ReconcileCheckLimit: time.Microsecond, ReconcileCheckInterval: time.Millisecond}
	m := NewMultiLimiter(Config)
	ctx, cancel := context.WithCancel(context.Background())
	m.Run(ctx)
	defer cancel()
	ip := []byte{244, 233, 0, 1}
	for j := 0; j < b.N; j++ {
		m.Allow(ipLimited{key: ip})
	}
}

func BenchmarkTestRateLimiterAllowPrefill(b *testing.B) {

	cases := []struct {
		name    string
		prefill uint64
	}{
		{name: "no prefill", prefill: 0},
		{name: "prefill with 1K keys", prefill: 1000},
		{name: "prefill with 10K keys", prefill: 10_000},
		{name: "prefill with 100K keys", prefill: 100_000},
	}
	for _, tc := range cases {

		b.Run(tc.name, func(b *testing.B) {
			var Config = Config{ReconcileCheckLimit: time.Second, ReconcileCheckInterval: time.Second}
			m := NewMultiLimiter(Config)
			var i uint64
			for i = 0xdeaddead; i < 0xdeaddead+tc.prefill; i++ {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, i)
				m.Allow(ipLimited{key: buf})
			}
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				buf := make([]byte, 4)
				binary.LittleEndian.PutUint32(buf, uint32(j))
				m.Allow(ipLimited{key: buf})
			}
		})
	}

}

func BenchmarkTestRateLimiterAllowConcurrencyPrefill(b *testing.B) {

	cases := []struct {
		name    string
		prefill uint64
	}{
		{name: "no prefill", prefill: 0},
		{name: "prefill with 1K keys", prefill: 1000},
		{name: "prefill with 10K keys", prefill: 10_000},
		{name: "prefill with 100K keys", prefill: 100_000},
	}
	for _, tc := range cases {

		b.Run(tc.name, func(b *testing.B) {
			var Config = Config{ReconcileCheckLimit: time.Second, ReconcileCheckInterval: 100 * time.Second}
			m := NewMultiLimiter(Config)
			ctx, cancel := context.WithCancel(context.Background())
			m.Run(ctx)
			defer cancel()
			var i uint64
			for i = 0xdeaddead; i < 0xdeaddead+tc.prefill; i++ {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, i)
				m.Allow(ipLimited{key: buf})
			}
			wg := sync.WaitGroup{}
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				wg.Add(1)
				go func(n int) {
					buf := make([]byte, 4)
					binary.LittleEndian.PutUint32(buf, uint32(n))
					m.Allow(ipLimited{key: buf})
					wg.Done()
				}(j)
			}
			wg.Wait()
		})
	}

}

func BenchmarkTestRateLimiterRandomIP(b *testing.B) {
	var Config = Config{ReconcileCheckLimit: time.Microsecond, ReconcileCheckInterval: time.Millisecond}
	m := NewMultiLimiter(Config)
	ctx, cancel := context.WithCancel(context.Background())
	m.Run(ctx)
	defer cancel()
	for j := 0; j < b.N; j++ {
		m.Allow(ipLimited{key: randIP()})
	}
}

func randIP() []byte {
	buf := make([]byte, 4)

	ip := rand.Uint32()

	binary.LittleEndian.PutUint32(buf, ip)
	return buf
}

type mockTicker struct {
	tickerCh chan time.Time
}

func (m *mockTicker) Ticker() <-chan time.Time {
	return m.tickerCh
}

func splitKey(key []byte) ([]byte, []byte) {

	ret := bytes.SplitN(key, []byte(separator), 2)
	if len(ret) != 2 {
		return []byte(""), []byte("")
	}
	return ret[0], ret[1]
}
