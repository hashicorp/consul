package multilimiter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	radix "github.com/hashicorp/go-immutable-radix"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"math/rand"
	"testing"
	"time"
)

type Limited struct {
	key KeyType
}

func (l Limited) Key() []byte {
	return l.key
}

func TestNewMultiLimiter(t *testing.T) {
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}}
	m := NewMultiLimiter(c)
	require.NotNil(t, m)
	require.NotNil(t, m.limiters)
}

func TestRateLimiterUpdate(t *testing.T) {
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, ReconcileCheckLimit: 1 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	key := makeKey([]byte("test"))
	m.Allow(Limited{key: key})
	limiters := m.limiters.Load()
	l1, ok1 := limiters.Get(key)
	require.True(t, ok1)
	require.NotNil(t, l1)
	la1 := l1.(*Limiter).lastAccess.Load()
	m.Allow(Limited{key: key})
	limiters = m.limiters.Load()
	l2, ok2 := limiters.Get(key)
	require.True(t, ok2)
	require.NotNil(t, l2)
	require.Equal(t, l1, l2)
	la2 := l1.(*Limiter).lastAccess.Load()
	require.Equal(t, la1, la2)

}

func TestRateLimiterCleanup(t *testing.T) {

	// Create a limiter and Allow a key, check the key exists
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, ReconcileCheckLimit: 1 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	key := makeKey([]byte("test"))
	m.Allow(Limited{key: key})
	limiters := m.limiters.Load()
	l, ok := limiters.Get(key)
	require.True(t, ok)
	require.NotNil(t, l)

	// Wait > ReconcileCheckInterval and check that the key was cleaned up
	time.Sleep(20 * time.Millisecond)
	limiters = m.limiters.Load()
	l, ok = limiters.Get(key)
	require.False(t, ok)
	require.Nil(t, l)

	// Stop the cleanup routine, check that a key is not cleaned up after > ReconcileCheckInterval
	cancel()
	m.Allow(Limited{key: key})
	time.Sleep(20 * time.Millisecond)
	limiters = m.limiters.Load()
	l, ok = limiters.Get(key)
	require.True(t, ok)
	require.NotNil(t, l)
}

func TestRateLimiterUpdateConfig(t *testing.T) {

	// Create a MultiLimiter m with a config c and check the config is applied
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	require.Equal(t, *m.config.Load(), c)

	t.Run("Allow a key and check defaultConfig is applied to that key", func(t *testing.T) {
		ipNoPrefix := Key([]byte(""), []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ipNoPrefix})
		l, ok := m.limiters.Load().Get(ipNoPrefix)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.isApplied(limiter.limiter))
	})

	t.Run("Allow 2 keys with prefix and check defaultConfig is applied to those keys", func(t *testing.T) {
		prefix := []byte("namespace.write")
		ip := Key(prefix, []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		ip2 := Key(prefix, []byte("127.0.0.2"))
		m.Allow(ipLimited{key: ip2})
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.LimiterConfig.isApplied(limiter.limiter))
	})
	t.Run("Apply a config to 'namespace.write' check the config is applied to existing keys under that prefix", func(t *testing.T) {
		prefix := []byte("namespace.write")
		ip := Key(prefix, []byte("127.0.0.1"))
		c3 := LimiterConfig{Rate: 2}
		m.UpdateConfig(c3, prefix)
		// call reconcileLimitedOnce to make sure the update is applied
		m.reconcileLimitedOnce(context.Background())
		m.Allow(ipLimited{key: ip})
		l3, ok3 := m.limiters.Load().Get(ip)
		require.True(t, ok3)
		require.NotNil(t, l3)
		limiter3 := l3.(*Limiter)
		require.True(t, c3.isApplied(limiter3.limiter))
	})
	t.Run("Allow an IP with prefix and check prefix config is applied to new keys under that prefix", func(t *testing.T) {
		c := LimiterConfig{Rate: 3}
		prefix := []byte("namespace.read")
		m.UpdateConfig(c, prefix)
		ip := Key(prefix, []byte("127.0.0.1"))
		m.Allow(ipLimited{key: ip})
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.isApplied(limiter.limiter))
	})

	t.Run("Allow an IP with prefix and check after it's cleaned new Allow would give it the right config", func(t *testing.T) {
		prefix := []byte("namespace.read")
		ip := Key(prefix, []byte("127.0.0.1"))
		c := LimiterConfig{Rate: 1}
		m.UpdateConfig(c, prefix)
		// call reconcileLimitedOnce to make sure the update is applied
		m.reconcileLimitedOnce(context.Background())
		m.Allow(ipLimited{key: ip})
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.isApplied(limiter.limiter))
		time.Sleep(200 * time.Millisecond)
		m.reconcileLimitedOnce(context.Background())
		l, ok = m.limiters.Load().Get(ip)
		require.False(t, ok)
		require.Nil(t, l)
	})
}

func FuzzSingleConfig(f *testing.F) {
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, ReconcileCheckLimit: 100 * time.Millisecond, ReconcileCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	require.Equal(f, *m.config.Load(), c)
	f.Add(makeKey(randIP()))
	f.Add(makeKey(randIP(), randIP()))
	f.Add(makeKey(randIP(), randIP(), randIP()))
	f.Add(makeKey(randIP(), randIP(), randIP(), randIP()))
	f.Fuzz(func(t *testing.T, ff []byte) {
		m.Allow(Limited{key: ff})
		checkLimiter(t, ff, m.limiters.Load())
		checkTree(t, m.limiters.Load())
	})
}

func FuzzSplitKey(f *testing.F) {
	f.Add(makeKey(randIP(), randIP()))
	f.Add(makeKey(randIP(), randIP(), randIP()))
	f.Add(makeKey(randIP(), randIP(), randIP(), randIP()))
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

func checkLimiter(t require.TestingT, ff []byte, Tree *radix.Tree) {
	v, ok := Tree.Get(ff)
	require.True(t, ok)
	require.NotNil(t, v)
}

func FuzzUpdateConfig(f *testing.F) {

	f.Add(bytes.Join([][]byte{[]byte(""), makeKey(randIP()), makeKey(randIP(), randIP()), makeKey(randIP(), randIP(), randIP()), makeKey(randIP(), randIP(), randIP(), randIP())}, []byte(",")))
	f.Fuzz(func(t *testing.T, ff []byte) {
		cm := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, ReconcileCheckLimit: 100000 * time.Hour, ReconcileCheckInterval: 10 * time.Millisecond}
		m := NewMultiLimiter(cm)
		keys := bytes.Split(ff, []byte(","))
		for _, f := range keys {
			prefix, _ := splitKey(f)
			c := LimiterConfig{Rate: rate.Limit(rand.Float64()), Burst: rand.Int()}
			m.UpdateConfig(c, prefix)
			m.Allow(Limited{key: f})
		}
		m.reconcileLimitedOnce(context.Background())
		checkTree(t, m.limiters.Load())
	})

}

func checkTree(t require.TestingT, tree *radix.Tree) {
	iterator := tree.Root().Iterator()
	kp, v, ok := iterator.Next()
	for ok {
		switch c := v.(type) {
		case *Limiter:
			if c.limiter != nil {
				prefix, _ := splitKey(kp)
				v, _ := tree.Get(prefix)
				switch c2 := v.(type) {
				case *Limiter:
					if c2.config != nil {
						config := c2.config.Load()
						applied := config.isApplied(c.limiter)
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
	var Config = Config{LimiterConfig: LimiterConfig{Rate: 1.0, Burst: 500}}
	m := NewMultiLimiter(Config)
	ip := []byte{244, 233, 0, 1}
	for j := 0; j < b.N; j++ {
		m.Allow(ipLimited{key: ip})
	}
}

func BenchmarkTestRateLimiterIncIP(b *testing.B) {
	var Config = Config{LimiterConfig: LimiterConfig{Rate: 1.0, Burst: 500}}
	m := NewMultiLimiter(Config)
	buf := make([]byte, 4)
	for j := 0; j < b.N; j++ {
		binary.LittleEndian.PutUint32(buf, uint32(j))
		m.Allow(ipLimited{key: buf})
	}
}

func BenchmarkTestRateLimiterRandomIP(b *testing.B) {
	var Config = Config{LimiterConfig: LimiterConfig{Rate: 1.0, Burst: 500}}
	m := NewMultiLimiter(Config)
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
