package retry

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
			var iters int
			start := time.Now()
			for tt.r.Continue() {
				iters++
			}
			dur := time.Since(start)
			if got, want := iters, 3; got != want {
				t.Fatalf("got %d retries want %d", got, want)
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

func TestRunWith(t *testing.T) {
	t.Run("calls FailNow after exceeding retries", func(t *testing.T) {
		ft := &fakeT{}
		iter := 0
		RunWith(&Counter{Count: 3, Wait: time.Millisecond}, ft, func(r *R) {
			iter++
			r.FailNow()
		})

		require.Equal(t, 3, iter)
		require.Equal(t, 1, ft.fails)
	})

	t.Run("Stop ends the retrying", func(t *testing.T) {
		ft := &fakeT{}
		iter := 0
		RunWith(&Counter{Count: 5, Wait: time.Millisecond}, ft, func(r *R) {
			iter++
			if iter == 2 {
				r.Stop(fmt.Errorf("do not proceed"))
			}
			r.Fatalf("not yet")
		})

		require.Equal(t, 2, iter)
		require.Equal(t, 1, ft.fails)
		require.Len(t, ft.out, 1)
		require.Contains(t, ft.out[0], "not yet\n")
		require.Contains(t, ft.out[0], "do not proceed\n")
	})
}

type fakeT struct {
	fails int
	out   []string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Log(args ...interface{}) {
	f.out = append(f.out, fmt.Sprint(args...))
}

func (f *fakeT) FailNow() {
	f.fails++
}

var _ Failer = &fakeT{}
