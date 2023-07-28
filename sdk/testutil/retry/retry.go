// Package retry provides support for repeating operations in tests.
//
// A sample retry operation looks like this:
//
//	func TestX(t *testing.T) {
//	    retry.Run(t, func(r *retry.R) {
//	        if err := foo(); err != nil {
//	            r.Fatal("f: ", err)
//	        }
//	    })
//	}
package retry

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Failer is an interface compatible with testing.T.
type Failer interface {
	Helper()

	// Log is called for the final test output
	Log(args ...interface{})

	// FailNow is called when the retrying is abandoned.
	FailNow()
}

// R provides context for the retryer.
type R struct {
	fail   bool
	done   bool
	output []string
}

func (r *R) Helper() {}

var runFailed = struct{}{}

func (r *R) FailNow() {
	r.fail = true
	panic(runFailed)
}

func (r *R) Fatal(args ...interface{}) {
	r.log(fmt.Sprint(args...))
	r.FailNow()
}

func (r *R) Fatalf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
	r.FailNow()
}

func (r *R) Error(args ...interface{}) {
	r.log(fmt.Sprint(args...))
	r.fail = true
}

func (r *R) Errorf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
	r.fail = true
}

func (r *R) Check(err error) {
	if err != nil {
		r.log(err.Error())
		r.FailNow()
	}
}

func (r *R) log(s string) {
	r.output = append(r.output, decorate(s))
}

// Stop retrying, and fail the test with the specified error.
func (r *R) Stop(err error) {
	r.log(err.Error())
	r.done = true
}

func decorate(s string) string {
	_, file, line, ok := runtime.Caller(3)
	if ok {
		n := strings.LastIndex(file, "/")
		if n >= 0 {
			file = file[n+1:]
		}
	} else {
		file = "???"
		line = 1
	}
	return fmt.Sprintf("%s:%d: %s", file, line, s)
}

func Run(t Failer, f func(r *R)) {
	t.Helper()
	run(DefaultFailer(), t, f)
}

func RunWith(r Retryer, t Failer, f func(r *R)) {
	t.Helper()
	run(r, t, f)
}

func dedup(a []string) string {
	if len(a) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	var b bytes.Buffer
	for _, s := range a {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		b.WriteString(s)
		b.WriteRune('\n')
	}
	return b.String()
}

func run(r Retryer, t Failer, f func(r *R)) {
	t.Helper()
	rr := &R{}

	fail := func() {
		t.Helper()
		out := dedup(rr.output)
		if out != "" {
			t.Log(out)
		}
		t.FailNow()
	}

	for r.Continue() {
		func() {
			defer func() {
				if p := recover(); p != nil && p != runFailed {
					panic(p)
				}
			}()
			f(rr)
		}()

		switch {
		case rr.done:
			fail()
			return
		case !rr.fail:
			return
		}
		rr.fail = false
	}
	fail()
}

// DefaultFailer provides default retry.Run() behavior for unit tests.
func DefaultFailer() *Timer {
	return &Timer{Timeout: 7 * time.Second, Wait: 25 * time.Millisecond}
}

// TwoSeconds repeats an operation for two seconds and waits 25ms in between.
func TwoSeconds() *Timer {
	return &Timer{Timeout: 2 * time.Second, Wait: 25 * time.Millisecond}
}

// ThreeTimes repeats an operation three times and waits 25ms in between.
func ThreeTimes() *Counter {
	return &Counter{Count: 3, Wait: 25 * time.Millisecond}
}

// Retryer provides an interface for repeating operations
// until they succeed or an exit condition is met.
type Retryer interface {
	// Continue returns true if the operation should be repeated, otherwise it
	// returns false to indicate retrying should stop.
	Continue() bool
}

// Counter repeats an operation a given number of
// times and waits between subsequent operations.
type Counter struct {
	Count int
	Wait  time.Duration

	count int
}

func (r *Counter) Continue() bool {
	if r.count == r.Count {
		return false
	}
	if r.count > 0 {
		time.Sleep(r.Wait)
	}
	r.count++
	return true
}

// Timer repeats an operation for a given amount
// of time and waits between subsequent operations.
type Timer struct {
	Timeout time.Duration
	Wait    time.Duration

	// stop is the timeout deadline.
	// Set on the first invocation of Next().
	stop time.Time
}

func (r *Timer) Continue() bool {
	if r.stop.IsZero() {
		r.stop = time.Now().Add(r.Timeout)
		return true
	}
	if time.Now().After(r.stop) {
		return false
	}
	time.Sleep(r.Wait)
	return true
}
