// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package retry provides support for repeating operations in tests.
//
// A sample retry operation looks like this:
//
//	func TestX(t *testing.T) {
//	    retry.Run(t, func(r *retry.R) {
//	        if err := foo(); err != nil {
//				r.Errorf("foo: %s", err)
//				return
//	        }
//	    })
//	}
//
// Run uses the DefaultFailer, which is a Timer with a Timeout of 7s,
// and a Wait of 25ms. To customize, use RunWith.
//
// WARNING: unlike *testing.T, *retry.R#Fatal and FailNow *do not*
// fail the test function entirely, only the current run the retry func
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
//
// Logs from Logf, (Error|Fatal)(f) are gathered in an internal buffer
// and printed only if the retryer fails. Printed logs are deduped and
// prefixed with source code line numbers
type R struct {
	// fail is set by FailNow and (Fatal|Error)(f). It indicates the pass
	// did not succeed, and should be retried
	fail bool
	// done is set by Stop. It indicates the entire run was a failure,
	// and triggers t.FailNow()
	done   bool
	output []string

	cleanups []func()
}

func (r *R) Logf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
}

func (r *R) Log(args ...interface{}) {
	r.log(fmt.Sprintln(args...))
}

func (r *R) Helper() {}

// Cleanup register a function to be run to cleanup resources that
// were allocated during the retry attempt. These functions are executed
// after a retry attempt. If they panic, it will not stop further retry
// attempts but will be cause for the overall test failure.
func (r *R) Cleanup(fn func()) {
	r.cleanups = append(r.cleanups, fn)
}

func (r *R) runCleanup() {

	// Make sure that if a cleanup function panics,
	// we still run the remaining cleanup functions.
	defer func() {
		err := recover()
		if err != nil {
			r.Stop(fmt.Errorf("error when performing test cleanup: %v", err))
		}
		if len(r.cleanups) > 0 {
			r.runCleanup()
		}
	}()

	for len(r.cleanups) > 0 {
		var cleanup func()
		if len(r.cleanups) > 0 {
			last := len(r.cleanups) - 1
			cleanup = r.cleanups[last]
			r.cleanups = r.cleanups[:last]
		}
		if cleanup != nil {
			cleanup()
		}
	}
}

// runFailed is a sentinel value to indicate that the func itself
// didn't panic, rather that `FailNow` was called.
type runFailed struct{}

// FailNow stops run execution. It is roughly equivalent to:
//
//	r.Error("")
//	return
//
// inside the function being run.
func (r *R) FailNow() {
	r.fail = true
	panic(runFailed{})
}

// Fatal is equivalent to r.Logf(args) followed by r.FailNow(), i.e. the run
// function should be exited. Retries on the next run are allowed. Fatal is
// equivalent to
//
//	r.Error(args)
//	return
//
// inside the function being run.
func (r *R) Fatal(args ...interface{}) {
	r.log(fmt.Sprint(args...))
	r.FailNow()
}

// Fatalf is like Fatal but allows a format string
func (r *R) Fatalf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
	r.FailNow()
}

// Error indicates the current run encountered an error and should be retried.
// It *does not* stop execution of the rest of the run function.
func (r *R) Error(args ...interface{}) {
	r.log(fmt.Sprint(args...))
	r.fail = true
}

// Errorf is like Error but allows a format string
func (r *R) Errorf(format string, args ...interface{}) {
	r.log(fmt.Sprintf(format, args...))
	r.fail = true
}

// If err is non-nil, equivalent to r.Fatal(err.Error()) followed by
// r.FailNow(). Otherwise a no-op.
func (r *R) Check(err error) {
	if err != nil {
		r.log(err.Error())
		r.FailNow()
	}
}

func (r *R) log(s string) {
	r.output = append(r.output, decorate(s))
}

// Stop retrying, and fail the test, logging the specified error.
// Does not stop execution, so return should be called after.
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
		// run f(rr), but if recover yields a runFailed value, we know
		// FailNow was called.
		func() {
			defer rr.runCleanup()
			defer func() {
				if p := recover(); p != nil && p != (runFailed{}) {
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

// DefaultFailer provides default retry.Run() behavior for unit tests, namely
// 7s timeout with a wait of 25ms
func DefaultFailer() *Timer {
	return &Timer{Timeout: 7 * time.Second, Wait: 25 * time.Millisecond}
}

// Retryer provides an interface for repeating operations
// until they succeed or an exit condition is met.
type Retryer interface {
	// Continue returns true if the operation should be repeated, otherwise it
	// returns false to indicate retrying should stop.
	Continue() bool
}
