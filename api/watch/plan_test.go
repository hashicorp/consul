// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
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

func TestSetCancelFunc_StoppedPlan(t *testing.T) {
	t.Parallel()
	plan := mustParse(t, `{"type":"noop"}`)
	plan.Stop()

	called := false
	plan.setCancelFunc(func() { called = true })

	if plan.cancelFunc == nil {
		t.Fatalf("cancelFunc should be set even after stop")
	}
	if !called {
		t.Fatalf("cancel should be called immediately when stopped")
	}
}

func TestSetCancelFunc_ConcurrentStop(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		plan := mustParse(t, `{"type":"noop"}`)

		stopDone := make(chan struct{})
		watchDone := make(chan struct{})

		go func() {
			defer close(stopDone)
			plan.Stop()
		}()

		go func() {
			defer close(watchDone)
			plan.setCancelFunc(func() {})
			plan.cancelFunc()
		}()

		select {
		case <-stopDone:
		case <-time.After(1 * time.Second):
			t.Fatalf("iteration %d: timed out waiting for stop", i)
		}

		select {
		case <-watchDone:
		case <-time.After(1 * time.Second):
			t.Fatalf("iteration %d: timed out waiting for watch", i)
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
