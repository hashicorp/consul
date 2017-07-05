package retry

import (
	"testing"
	"time"
)

// delta defines the time band a test run should complete in.
var delta = 25 * time.Millisecond

func TestRetryer(t *testing.T) {
	tests := []struct {
		desc string
		r    Retryer
	}{
		{"counter", &Counter{Count: 3, Wait: 100 * time.Millisecond}},
		{"timer", &Timer{Timeout: 200 * time.Millisecond, Wait: 100 * time.Millisecond}},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var iters, fails int
			fail := func() { fails++ }
			start := time.Now()
			for tt.r.NextOr(fail) {
				iters++
			}
			dur := time.Since(start)
			if got, want := iters, 3; got != want {
				t.Fatalf("got %d retries want %d", got, want)
			}
			if got, want := fails, 1; got != want {
				t.Fatalf("got %d FailNow calls want %d", got, want)
			}
			// since the first iteration happens immediately
			// the retryer waits only twice for three iterations.
			// order of events: (true, (wait) true, (wait) true, false)
			if got, want := dur, 200*time.Millisecond; got < (want-delta) || got > (want+delta) {
				t.Fatalf("loop took %v want %v (+/- %v)", got, want, delta)
			}
		})
	}
}
