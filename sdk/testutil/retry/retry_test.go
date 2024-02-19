// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestBasics(t *testing.T) {
	t.Run("Error allows retry", func(t *testing.T) {
		i := 0
		Run(t, func(r *R) {
			i++
			t.Logf("i: %d; r: %#v", i, r)
			if i == 1 {
				r.Errorf("Errorf, i: %d", i)
				return
			}
		})
		assert.Equal(t, i, 2)
	})

	t.Run("Fatal returns from func, but does not fail test", func(t *testing.T) {
		i := 0
		gotHere := false
		ft := &fakeT{T: t}
		Run(ft, func(r *R) {
			i++
			t.Logf("i: %d; r: %#v", i, r)
			if i == 1 {
				r.Fatalf("Fatalf, i: %d", i)
				gotHere = true
			}
		})

		assert.False(t, gotHere)
		assert.Equal(t, i, 2)
		// surprisingly, r.FailNow() *does not* trigger ft.FailNow()!
		assert.Equal(t, ft.fails, 0)
	})

	t.Run("Func being run can panic with struct{}{}", func(t *testing.T) {
		gotPanic := false
		func() {
			defer func() {
				if p := recover(); p != nil {
					gotPanic = true
				}
			}()
			Run(t, func(r *R) {
				panic(struct{}{})
			})
		}()

		assert.True(t, gotPanic)
	})
}

func TestRunWith(t *testing.T) {
	t.Run("calls FailNow after exceeding retries", func(t *testing.T) {
		ft := &fakeT{T: t}
		iter := 0
		RunWith(&Counter{Count: 3, Wait: time.Millisecond}, ft, func(r *R) {
			iter++
			r.FailNow()
		})

		require.Equal(t, 3, iter)
		require.Equal(t, 1, ft.fails)
	})

	t.Run("Stop ends the retrying", func(t *testing.T) {
		ft := &fakeT{T: t}
		iter := 0
		RunWith(&Counter{Count: 5, Wait: time.Millisecond}, ft, func(r *R) {
			iter++
			if iter == 2 {
				r.Stop(fmt.Errorf("do not proceed"))
			}
			r.Fatalf("not yet")
		})

		// TODO: these should all be assert
		require.Equal(t, 2, iter)
		require.Equal(t, 1, ft.fails)
		require.Len(t, ft.out, 1)
		require.Contains(t, ft.out[0], "not yet\n")
		require.Contains(t, ft.out[0], "do not proceed\n")
	})
}

func TestCleanup_Passthrough(t *testing.T) {

}

func TestCleanup(t *testing.T) {

	t.Run("basic", func(t *testing.T) {
		ft := &fakeT{T: t}
		cleanupsExecuted := 0

		Run(
			ft,
			func(r *R) {
				r.Cleanup(func() {
					cleanupsExecuted += 1
				})
			},
			WithImmediateCleanup(),
			WithRetryer(&Counter{Count: 2, Wait: time.Millisecond}),
		)

		require.Equal(t, 0, ft.fails)
		require.Equal(t, 1, cleanupsExecuted)
	})
	t.Run("cleanup-panic-recovery", func(t *testing.T) {
		ft := &fakeT{T: t}
		cleanupsExecuted := 0
		Run(
			ft,
			func(r *R) {
				r.Cleanup(func() {
					cleanupsExecuted += 1
				})

				r.Cleanup(func() {
					cleanupsExecuted += 1
					panic(fmt.Errorf("fake test error"))
				})

				r.Cleanup(func() {
					cleanupsExecuted += 1
				})

				// test is successful but should fail due to the cleanup panicing
			},
			WithRetryer(&Counter{Count: 2, Wait: time.Millisecond}),
			WithImmediateCleanup(),
		)

		require.Equal(t, 3, cleanupsExecuted)
		require.Equal(t, 1, ft.fails)
		require.Contains(t, ft.out[0], "fake test error")
	})

	t.Run("cleanup-per-retry", func(t *testing.T) {
		ft := &fakeT{T: t}
		iter := 0
		cleanupsExecuted := 0
		Run(
			ft,
			func(r *R) {
				if cleanupsExecuted != iter {
					r.Stop(fmt.Errorf("cleanups not executed between retries"))
					return
				}
				iter += 1

				r.Cleanup(func() {
					cleanupsExecuted += 1
				})

				r.FailNow()
			},
			WithRetryer(&Counter{Count: 3, Wait: time.Millisecond}),
			WithImmediateCleanup(),
		)

		require.Equal(t, 3, cleanupsExecuted)
		// ensure that r.Stop hadn't been called. If it was then we would
		// have log output
		require.Len(t, ft.out, 0)
	})

	t.Run("passthrough-to-t", func(t *testing.T) {
		cleanupsExecuted := 0

		require.True(t, t.Run("internal", func(t *testing.T) {
			iter := 0
			Run(
				t,
				func(r *R) {
					iter++

					r.Cleanup(func() {
						cleanupsExecuted += 1
					})

					// fail all but the last one to ensure the right number of cleanups
					// are eventually executed
					if iter < 3 {
						r.FailNow()
					}
				},
				WithRetryer(&Counter{Count: 3, Wait: time.Millisecond}),
			)

			// at this point nothing should be cleaned up
			require.Equal(t, 0, cleanupsExecuted)
		}))

		// now since the subtest finished the test cleanup funcs
		// should have been executed.
		require.Equal(t, 3, cleanupsExecuted)
	})
}

type fakeT struct {
	fails int
	out   []string
	*testing.T
}

func (f *fakeT) Helper() {}

func (f *fakeT) Log(args ...interface{}) {
	f.out = append(f.out, fmt.Sprint(args...))
}

func (f *fakeT) FailNow() {
	f.fails++
}

var _ TestingTB = &fakeT{}
