// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

import (
	"fmt"
	"os"
)

var _ TestingTB = &R{}

type R struct {
	wrapped TestingTB
	retryer Retryer

	done             bool
	fullOutput       bool
	immediateCleanup bool

	attempts []*attempt
}

func (r *R) Cleanup(clean func()) {
	if r.immediateCleanup {
		a := r.getCurrentAttempt()
		a.cleanups = append(a.cleanups, clean)
	} else {
		r.wrapped.Cleanup(clean)
	}
}

func (r *R) Error(args ...any) {
	r.Log(args...)
	r.Fail()
}

func (r *R) Errorf(format string, args ...any) {
	r.Logf(format, args...)
	r.Fail()
}

func (r *R) Fail() {
	r.getCurrentAttempt().failed = true
}

func (r *R) FailNow() {
	r.Fail()
	panic(attemptFailed{})
}

func (r *R) Failed() bool {
	return r.getCurrentAttempt().failed
}

func (r *R) Fatal(args ...any) {
	r.Log(args...)
	r.FailNow()
}

func (r *R) Fatalf(format string, args ...any) {
	r.Logf(format, args...)
	r.FailNow()
}

func (r *R) Helper() {
	// *testing.T will just record which functions are helpers by their addresses and
	// it doesn't much matter where where we record that they are helpers
	r.wrapped.Helper()
}

func (r *R) Log(args ...any) {
	r.log(fmt.Sprintln(args...))
}

func (r *R) Logf(format string, args ...any) {
	r.log(fmt.Sprintf(format, args...))
}

// Name will return the name of the underlying TestingT.
func (r *R) Name() string {
	return r.wrapped.Name()
}

// Setenv will save the current value of the specified env var, set it to the
// specified value and then restore it to the original value in a cleanup function
// once the retry attempt has finished.
func (r *R) Setenv(key, value string) {
	prevValue, ok := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		r.wrapped.Fatalf("cannot set environment variable: %v", err)
	}

	if ok {
		r.Cleanup(func() {
			os.Setenv(key, prevValue)
		})
	} else {
		r.Cleanup(func() {
			os.Unsetenv(key)
		})
	}
}

// TempDir will use the wrapped TestingT to create a temporary directory
// that will be cleaned up when ALL RETRYING has finished.
func (r *R) TempDir() string {
	return r.wrapped.TempDir()
}

// Check will call r.Fatal(err) if err is not nil
func (r *R) Check(err error) {
	if err != nil {
		r.Fatal(err)
	}
}

func (r *R) Stop(err error) {
	r.log(err.Error())
	r.done = true
}

func (r *R) failCurrentAttempt() {
	r.getCurrentAttempt().failed = true
}

func (r *R) log(s string) {
	a := r.getCurrentAttempt()
	a.output = append(a.output, decorate(s))
}

func (r *R) getCurrentAttempt() *attempt {
	if len(r.attempts) == 0 {
		panic("no retry attempts have been started yet")
	}

	return r.attempts[len(r.attempts)-1]
}

// cleanupAttempt will perform all the register cleanup operations recorded
// during execution of the single round of the test function.
func (r *R) cleanupAttempt(a *attempt) {
	// Make sure that if a cleanup function panics,
	// we still run the remaining cleanup functions.
	defer func() {
		err := recover()
		if err != nil {
			r.Stop(fmt.Errorf("error when performing test cleanup: %v", err))
		}
		if len(a.cleanups) > 0 {
			r.cleanupAttempt(a)
		}
	}()

	for len(a.cleanups) > 0 {
		var cleanup func()
		if len(a.cleanups) > 0 {
			last := len(a.cleanups) - 1
			cleanup = a.cleanups[last]
			a.cleanups = a.cleanups[:last]
		}
		if cleanup != nil {
			cleanup()
		}
	}
}

// runAttempt will execute one round of the test function and handle cleanups and panic recovery
// of a failed attempt that should not stop retrying.
func (r *R) runAttempt(f func(r *R)) {
	r.Helper()

	a := &attempt{}
	r.attempts = append(r.attempts, a)

	defer r.cleanupAttempt(a)
	defer func() {
		if p := recover(); p != nil && p != (attemptFailed{}) {
			panic(p)
		}
	}()
	f(r)
}

func (r *R) run(f func(r *R)) {
	r.Helper()

	for r.retryer.Continue() {
		r.runAttempt(f)

		switch {
		case r.done:
			r.recordRetryFailure()
			return
		case !r.Failed():
			// the current attempt did not fail so we can go ahead and return
			return
		}
	}

	// We cannot retry any more and no attempt has succeeded yet.
	r.recordRetryFailure()
}

func (r *R) recordRetryFailure() {
	r.Helper()
	output := r.getCurrentAttempt().output
	if r.fullOutput {
		var combined []string
		for _, attempt := range r.attempts {
			combined = append(combined, attempt.output...)
		}
		output = combined
	}

	out := dedup(output)
	if out != "" {
		r.wrapped.Log(out)
	}
	r.wrapped.FailNow()
}

type attempt struct {
	failed   bool
	output   []string
	cleanups []func()
}

// attemptFailed is a sentinel value to indicate that the func itself
// didn't panic, rather that `FailNow` was called.
type attemptFailed struct{}
