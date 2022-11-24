package multilimiter

import (
	"context"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

type Limited struct {
	key keyType
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
	key := makeKey("test")
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
	key := makeKey("test")
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
		ipNoPrefix := Key("", "127.0.0.1")
		m.Allow(ipLimited{key: ipNoPrefix})
		l, ok := m.limiters.Load().Get(ipNoPrefix)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.isApplied(limiter.limiter))
	})

	t.Run("Allow 2 keys with prefix and check defaultConfig is applied to those keys", func(t *testing.T) {
		prefix := "namespace.write"
		ip := Key(prefix, "127.0.0.1")
		m.Allow(ipLimited{key: ip})
		ip2 := Key(prefix, "127.0.0.2")
		m.Allow(ipLimited{key: ip2})
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.LimiterConfig.isApplied(limiter.limiter))
	})
	t.Run("Apply a config to 'namespace.write' check the config is applied to existing keys under that prefix", func(t *testing.T) {
		prefix := "namespace.write"
		ip := Key(prefix, "127.0.0.1")
		c3 := LimiterConfig{Rate: 2}
		m.UpdateConfig(c3, []byte(prefix))
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
		prefix := "namespace.read"
		m.UpdateConfig(c, []byte(prefix))
		ip := Key(prefix, "127.0.0.1")
		m.Allow(ipLimited{key: ip})
		l, ok := m.limiters.Load().Get(ip)
		require.True(t, ok)
		require.NotNil(t, l)
		limiter := l.(*Limiter)
		require.True(t, c.isApplied(limiter.limiter))
	})

	t.Run("Allow an IP with prefix and check after it's cleaned new Allow would give it the right config", func(t *testing.T) {
		prefix := "namespace.read"
		ip := Key(prefix, "127.0.0.1")
		c := LimiterConfig{Rate: 1}
		m.UpdateConfig(c, []byte(prefix))
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
