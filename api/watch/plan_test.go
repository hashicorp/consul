// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"context"
	"sync"
	"testing"
	"time"
)

func init() {
	watchFuncFactory["noop"] = noopWatch
}

func noopWatch(params map[string]interface{}) (WatcherFunc, error) {
	fn := func(p *Plan) (BlockingParamVal, interface{}, error) {
		idx := WaitIndexVal(0)
		if i, ok := p.lastParamVal.(WaitIndexVal); ok {
			idx = i
		}
		return idx + 1, uint64(idx + 1), nil
	}
	return fn, nil
}

func mustParse(t *testing.T, q string) *Plan {
	params := makeParams(t, q)
	plan, err := Parse(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return plan
}

func TestRun_Stop(t *testing.T) {
	t.Parallel()
	plan := mustParse(t, `{"type":"noop"}`)

	var expect uint64 = 1
	doneCh := make(chan struct{})
	plan.Handler = func(idx uint64, val interface{}) {
		if idx != expect {
			t.Fatalf("Bad: %d %d", expect, idx)
		}
		if val != expect {
			t.Fatalf("Bad: %d %d", expect, val)
		}
		if expect == 1 {
			close(doneCh)
		}
		expect++
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- plan.Run("127.0.0.1:8500")
	}()

	select {
	case <-doneCh:
		plan.Stop()

	case <-time.After(1 * time.Second):
		t.Fatalf("handler never ran")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("err: %v", err)
		}

	case <-time.After(1 * time.Second):
		t.Fatalf("watcher didn't exit")
	}

	if expect == 1 {
		t.Fatalf("Bad: %d", expect)
	}
}

func TestRun_Stop_Hybrid(t *testing.T) {
	t.Parallel()
	plan := mustParse(t, `{"type":"noop"}`)

	var expect uint64 = 1
	doneCh := make(chan struct{})
	plan.HybridHandler = func(blockParamVal BlockingParamVal, val interface{}) {
		idxVal, ok := blockParamVal.(WaitIndexVal)
		if !ok {
			t.Fatalf("expected index-based watch")
		}
		idx := uint64(idxVal)
		if idx != expect {
			t.Fatalf("Bad: %d %d", expect, idx)
		}
		if val != expect {
			t.Fatalf("Bad: %d %d", expect, val)
		}
		if expect == 1 {
			close(doneCh)
		}
		expect++
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- plan.Run("127.0.0.1:8500")
	}()

	select {
	case <-doneCh:
		plan.Stop()

	case <-time.After(1 * time.Second):
		t.Fatalf("handler never ran")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("err: %v", err)
		}

	case <-time.After(1 * time.Second):
		t.Fatalf("watcher didn't exit")
	}

	if expect == 1 {
		t.Fatalf("Bad: %d", expect)
	}
}

// TestSetCancelFunc_AlwaysSetsCancelFunc verifies that setCancelFunc always
// assigns p.cancelFunc even when the plan is already stopped. This prevents a
// nil pointer panic when the deferred cancelFunc() is called in watch functions.
// See https://github.com/hashicorp/consul/issues/19020
func TestSetCancelFunc_AlwaysSetsCancelFunc(t *testing.T) {
	t.Parallel()
	plan := mustParse(t, `{"type":"noop"}`)

	// Stop the plan first, then call setCancelFunc
	plan.Stop()

	called := false
	cancel := context.CancelFunc(func() { called = true })
	plan.setCancelFunc(cancel)

	// cancelFunc should be set even though the plan is stopped
	if plan.cancelFunc == nil {
		t.Fatalf("expected cancelFunc to be set, got nil")
	}

	// cancel should have been called immediately since the plan is stopped
	if !called {
		t.Fatalf("expected cancel to be called when plan is stopped")
	}
}

// TestSetCancelFunc_ConcurrentStopAndSet verifies there is no race condition
// or nil pointer panic when Stop and setCancelFunc are called concurrently.
// See https://github.com/hashicorp/consul/issues/19020
func TestSetCancelFunc_ConcurrentStopAndSet(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		plan := mustParse(t, `{"type":"noop"}`)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			plan.Stop()
		}()

		go func() {
			defer wg.Done()
			cancel := context.CancelFunc(func() {})
			plan.setCancelFunc(cancel)
		}()

		wg.Wait()

		if plan.cancelFunc == nil {
			t.Fatalf("iteration %d: expected cancelFunc to be set, got nil", i)
		}
	}
}

func TestRunWithClientAndLogger_NilLogger(t *testing.T) {
	t.Parallel()
	plan := mustParse(t, `{"type":"noop"}`)

	errCh := make(chan error, 1)
	go func() {
		errCh <- plan.RunWithClientAndHclog(nil, nil)
	}()

	plan.Stop()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("watcher didn't exit")
	}
}
