package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Backoff(t *testing.T) {
	t.Parallel()

	limiter := NewRateLimiter(1*time.Millisecond, 1*time.Second)

	request := Request{Kind: "one"}
	require.Equal(t, 1*time.Millisecond, limiter.NextRetry(request))
	require.Equal(t, 2*time.Millisecond, limiter.NextRetry(request))
	require.Equal(t, 4*time.Millisecond, limiter.NextRetry(request))
	require.Equal(t, 8*time.Millisecond, limiter.NextRetry(request))
	require.Equal(t, 16*time.Millisecond, limiter.NextRetry(request))

	requestTwo := Request{Kind: "two"}
	require.Equal(t, 1*time.Millisecond, limiter.NextRetry(requestTwo))
	require.Equal(t, 2*time.Millisecond, limiter.NextRetry(requestTwo))

	limiter.Forget(request)
	require.Equal(t, 1*time.Millisecond, limiter.NextRetry(request))
}

func TestRateLimiter_Overflow(t *testing.T) {
	t.Parallel()

	limiter := NewRateLimiter(1*time.Millisecond, 1000*time.Second)

	request := Request{Kind: "one"}
	for i := 0; i < 5; i++ {
		limiter.NextRetry(request)
	}
	// ensure we have a normally incrementing exponential backoff
	require.Equal(t, 32*time.Millisecond, limiter.NextRetry(request))

	overflow := Request{Kind: "overflow"}
	for i := 0; i < 1000; i++ {
		limiter.NextRetry(overflow)
	}
	// make sure we're capped at the passed in max backoff
	require.Equal(t, 1000*time.Second, limiter.NextRetry(overflow))

	limiter = NewRateLimiter(1*time.Minute, 1000*time.Hour)

	for i := 0; i < 2; i++ {
		limiter.NextRetry(request)
	}
	// ensure we have a normally incrementing exponential backoff
	require.Equal(t, 4*time.Minute, limiter.NextRetry(request))

	for i := 0; i < 1000; i++ {
		limiter.NextRetry(overflow)
	}
	// make sure we're capped at the passed in max backoff
	require.Equal(t, 1000*time.Hour, limiter.NextRetry(overflow))
}
