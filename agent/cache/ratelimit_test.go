package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/lib"
	"github.com/stretchr/testify/require"
)

// TestRateLimiter test between max value (auto-computed) and minValue or iterations.TestRateLimiter
// minValue is not exact as we have to deal with performance of hardware the test is running on.
// We keep some small tolerance to avoid unstable tests
func TestRateLimiter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		frequency string
		duration  string
		minVal    int64
	}{
		// We put some tolerance here for slow CI (minVal: 100 instead of 200)
		{frequency: "1/1ms", duration: "200ms", minVal: 100},
		{frequency: "1/10ms", duration: "250ms", minVal: 24},
		// Should be always 5 or 6 iterations
		{frequency: "5/s", duration: "1s", minVal: 5},
		// should be 1, because first call starts immediatly
		{frequency: "1/s", duration: "500ms", minVal: 1},
		{frequency: "1/1h", duration: "25ms", minVal: 1},
	}

	for _, tc := range cases {
		testCaseName := fmt.Sprintf("frequency: %s for %s", tc.frequency, tc.duration)
		t.Run(testCaseName, func(t *testing.T) {
			rateLimitSpec, err := lib.ParseRateLimit(tc.frequency)
			require.NoError(t, err, "RateLimitSpec should be correct")
			duration, err := time.ParseDuration(tc.duration)
			require.NoError(t, err, "Duration should be correct")
			// maxTargetCount is rate + 1 because we allow first request to start directly
			maxTargetCount := int64(duration/rateLimitSpec.Period) + 1
			rateLimiter := NewRateLimiter(rateLimitSpec)
			defer rateLimiter.Stop()
			go func() {
				time.Sleep(duration)
				rateLimiter.Stop()
			}()
			count := int64(0)
			for {
				err := rateLimiter.Take()
				if err != nil {
					break
				}
				count++
			}
			require.GreaterOrEqual(t, maxTargetCount, count)
			require.GreaterOrEqual(t, count, tc.minVal)
		})
	}
}
