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
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, CleanupCheckLimit: 1 * time.Millisecond, CleanupCheckInterval: 10 * time.Millisecond}
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
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, CleanupCheckLimit: 1 * time.Millisecond, CleanupCheckInterval: 10 * time.Millisecond}
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

	// Wait > CleanupCheckInterval and check that the key was cleaned up
	time.Sleep(20 * time.Millisecond)
	limiters = m.limiters.Load()
	l, ok = limiters.Get(key)
	require.False(t, ok)
	require.Nil(t, l)

	// Stop the cleanup routine, check that a key is not cleaned up after > CleanupCheckInterval
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
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, CleanupCheckLimit: 1 * time.Millisecond, CleanupCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	require.Equal(t, *m.config.Load(), c)

	// Allow an IP and check defaultConfig is applied to that IP
	ip := makeKey("127.0.0.1")
	m.Allow(ipLimited{key: ip})
	l, ok := m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter := l.(*Limiter)
	require.True(t, c.Equal(limiter.config.Load()))

	// Allow an IP with prefix and check defaultConfig is applied to that IP
	prefix := "namespace.write"
	ip = makeKey(prefix, "127.0.0.1")
	m.Allow(ipLimited{key: ip})
	l, ok = m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter = l.(*Limiter)
	require.True(t, c.Equal(limiter.config.Load()))

	// Update m config to c2 and check that c2 applied to m
	c2 := LimiterConfig{Rate: 1}
	m.UpdateConfig(c2, []byte(prefix))
	// call cleanupLimitedOnce to make sure the update is applied
	m.cleanupLimitedOnce(context.Background())
	m.Allow(ipLimited{key: ip})
	l, ok = m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter = l.(*Limiter)
	require.Equal(t, c2, *limiter.config.Load())
	time.Sleep(20 * time.Millisecond)
	m.cleanupLimitedOnce(context.Background())
	limiters := m.limiters.Load()
	l, ok = limiters.Get(ip)
	require.False(t, ok)
	require.Nil(t, l)

	c3 := LimiterConfig{Rate: 2}
	m.UpdateConfig(c3, []byte(prefix))
	// call cleanupLimitedOnce to make sure the update is applied
	m.cleanupLimitedOnce(context.Background())
	m.Allow(ipLimited{key: ip})
	l, ok = m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter = l.(*Limiter)
	require.Equal(t, c3, *limiter.config.Load())

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
