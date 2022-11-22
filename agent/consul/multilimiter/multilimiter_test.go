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
	key string
}

func (l Limited) Key() []byte {
	return []byte(l.key)
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
	m.Allow(Limited{key: "test"})
	limiters := m.limiters.Load()
	l1, ok1 := limiters.Get([]byte("test"))
	require.True(t, ok1)
	require.NotNil(t, l1)
	la1 := l1.(*Limiter).lastAccess.Load()
	m.Allow(Limited{key: "test"})
	limiters = m.limiters.Load()
	l2, ok2 := limiters.Get([]byte("test"))
	require.True(t, ok2)
	require.NotNil(t, l2)
	require.Equal(t, l1, l2)
	la2 := l1.(*Limiter).lastAccess.Load()
	require.Equal(t, la1, la2)

}

func TestRateLimiterCleanup(t *testing.T) {
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, CleanupCheckLimit: 1 * time.Millisecond, CleanupCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Allow(Limited{key: "test"})
	limiters := m.limiters.Load()
	l, ok := limiters.Get([]byte("test"))
	require.True(t, ok)
	require.NotNil(t, l)
	time.Sleep(20 * time.Millisecond)
	limiters = m.limiters.Load()
	l, ok = limiters.Get([]byte("test"))
	require.False(t, ok)
	require.Nil(t, l)
	cancel()
	m.Allow(Limited{key: "test"})
	time.Sleep(20 * time.Millisecond)
	limiters = m.limiters.Load()
	l, ok = limiters.Get([]byte("test"))
	require.True(t, ok)
	require.NotNil(t, l)
}

func TestRateLimiterUpdateConfig(t *testing.T) {
	c := Config{LimiterConfig: LimiterConfig{Rate: 0.1}, CleanupCheckLimit: 1 * time.Millisecond, CleanupCheckInterval: 10 * time.Millisecond}
	m := NewMultiLimiter(c)
	require.Equal(t, *m.config.Load(), c)
	ip := []byte("127.0.0.1")
	m.Allow(ipLimited{key: ip})
	l, ok := m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter := l.(*Limiter)
	require.True(t, c.Equal(limiter.config.Load()))
	c2 := Config{LimiterConfig: LimiterConfig{Rate: 1}, CleanupCheckLimit: 10 * time.Millisecond, CleanupCheckInterval: 100 * time.Millisecond}
	m.UpdateConfig(c2)
	l, ok = m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter = l.(*Limiter)
	require.True(t, c.Equal(limiter.config.Load()))
	m.Allow(ipLimited{key: ip})
	l, ok = m.limiters.Load().Get(ip)
	require.True(t, ok)
	require.NotNil(t, l)
	limiter = l.(*Limiter)
	require.True(t, c2.Equal(limiter.config.Load()))
	require.Equal(t, *m.config.Load(), c2)

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
