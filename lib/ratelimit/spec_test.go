package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRateLimitParsing test many possible values for rate-limit
func TestRateLimitParsing(t *testing.T) {
	t.Parallel()
	reqVal := func(toParse string, expected time.Duration) {
		t.Run(toParse, func(t *testing.T) {
			val, err := ParseRateLimit(toParse)
			require.NoErrorf(t, err, "Should not create a parsing error for: \"%s\"", toParse)
			require.Equal(t, val.Period, expected)
		})
	}
	reqVal("1/1s", time.Second)
	reqVal("1/s", time.Second)
	reqVal(" 1/ 1s", time.Second)
	reqVal(" 1   /1s", time.Second)
	reqVal("10/s", time.Second/10)
	reqVal("3/s", time.Nanosecond*333333333)
	reqVal("1/5s", time.Second*5)
	reqVal("1/10ns", 10*time.Nanosecond)
	reqVal("1/100us", 100*time.Microsecond)
	reqVal("1/100µs", 100*time.Microsecond)
	reqVal("1/ms", time.Millisecond)
	reqVal("1/2m", 2*time.Minute)
	reqVal("10/h", 6*time.Minute)
	reqVal("10/h", 6*time.Minute)

	// All remaining values should be errors in parsing
	reqError := func(toParse string) {
		t.Run(toParse, func(t *testing.T) {
			_, err := ParseRateLimit(toParse)
			require.Error(t, err, "Value \"%s\" should trigger an error", toParse)
		})
	}
	reqError("")
	reqError("h")
	reqError("/")
	reqError("//")
	reqError("///")
	reqError("1///1h")
	reqError("42")
	reqError("1/")
	reqError("1/1")
	reqError("/h")
	reqError("10/h /")
	reqError("0/h")
	reqError("-1/h")
	reqError("0/-1h")
	reqError("-1/ -1h")
	reqError("1/0")
	reqError("1/0m")
	reqError("1t/1h")
	reqError("1d/1h")
	reqError("0x01/1h")
}
