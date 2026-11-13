// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/lifecycle"
	"github.com/hashicorp/consul/lifecycle/lifecycletest"
)

func TestGate_FailFast(t *testing.T) {
	ex := newExchange(t)
	gate := ex.Require("server/state-store", lifecycle.PhaseReady)

	_, err := gate.Acquire(t.Context(), lifecycle.FailFast)
	if !errors.Is(err, lifecycle.ErrNotRegistered) {
		t.Fatalf("unregistered: %v", err)
	}

	h := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	_, err = gate.Acquire(t.Context(), lifecycle.FailFast)
	var nr *lifecycle.NotReadyError
	if !errors.As(err, &nr) || nr.State != lifecycle.StateBooting || nr.Phase != lifecycle.PhaseStarting {
		t.Fatalf("booting: %v", err)
	}
	if err := gate.Check(); !lifecycle.IsNotReady(err) {
		t.Fatalf("Check while booting: %v", err)
	}

	mustTransition(t, h, lifecycle.StateServing)
	lease, err := gate.Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatal(err)
	}
	if n := ex.Leases("server/state-store"); n != 1 {
		t.Fatalf("leases %d", n)
	}
	lease.Release()
	lease.Release()
	lifecycletest.AssertDrained(t, ex, "server/state-store")
}

func TestGate_MultipleComponentsAndPredicates(t *testing.T) {
	ex := newExchange(t)
	store := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	acl := mustRegister(t, ex, lifecycle.Simple("server/acl-resolver"))
	mustTransition(t, store, lifecycle.StateServing, lifecycle.WithAttributes(map[string]string{"index": "100"}))

	gate := ex.Require("server/state-store", lifecycle.PhaseReady).
		And("server/acl-resolver", lifecycle.PhaseReady, lifecycle.PhaseDegraded).
		AndFunc("server/state-store", "restored index >= 100", func(s lifecycle.Status) bool {
			return s.Attributes["index"] >= "100"
		})

	if _, err := gate.Acquire(t.Context(), lifecycle.FailFast); !lifecycle.IsNotReady(err) {
		t.Fatalf("acl starting: %v", err)
	}
	// A failed multi-component acquire must not leak leases.
	lifecycletest.AssertDrained(t, ex, "server/state-store")

	mustTransition(t, acl, lifecycle.StateDegraded)
	lease, err := gate.Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Leases("server/state-store") != 1 || ex.Leases("server/acl-resolver") != 1 {
		t.Fatal("expected one lease on each component")
	}
	lease.Release()

	if err := store.Update(lifecycle.WithAttributes(map[string]string{"index": "050"})); err != nil {
		t.Fatal(err)
	}
	_, err = gate.Acquire(t.Context(), lifecycle.FailFast)
	var nr *lifecycle.NotReadyError
	if !errors.As(err, &nr) || nr.Requirement != "restored index >= 100" {
		t.Fatalf("predicate: %v", err)
	}
}

func TestGate_WaitMode(t *testing.T) {
	ex := newExchange(t)
	h := mustRegister(t, ex, lifecycle.Simple("a"))
	if _, err := ex.Require("a", lifecycle.PhaseReady).And("b", lifecycle.PhaseReady).Acquire(t.Context(), lifecycle.Wait); !errors.Is(err, lifecycle.ErrWaitMultiple) {
		t.Fatalf("multi wait: %v", err)
	}

	got := make(chan error, 1)
	go func() {
		lease, err := ex.Require("a", lifecycle.PhaseReady).Acquire(t.Context(), lifecycle.Wait)
		lease.Release()
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("returned before Ready: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	mustTransition(t, h, lifecycle.StateReady)
	if err := <-got; err != nil {
		t.Fatal(err)
	}

	// Waiting for a component that registers later.
	go func() {
		_, err := ex.WaitFor(t.Context(), "late", lifecycle.PhaseReady)
		got <- err
	}()
	time.Sleep(20 * time.Millisecond)
	late := mustRegister(t, ex, lifecycle.Simple("late"))
	mustTransition(t, late, lifecycle.StateReady)
	if err := <-got; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := ex.Require("missing", lifecycle.PhaseReady).Acquire(ctx, lifecycle.Wait); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait timeout: %v", err)
	}
}

func TestBeginTransition_RefusesNewLeasesAndDrainsExisting(t *testing.T) {
	ex := newExchange(t)
	h := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	mustTransition(t, h, lifecycle.StateServing)
	gate := ex.Require("server/state-store", lifecycle.PhaseReady)

	held, err := gate.Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.BeginTransition(t.Context(), lifecycle.StateServing); !errors.Is(err, lifecycle.ErrInvalidTransition) {
		t.Fatalf("begin to a ready state: %v", err)
	}

	began := make(chan lifecycle.Done, 1)
	go func() {
		done, err := h.BeginTransition(t.Context(), lifecycle.StateRestoring, lifecycle.WithReason("test"))
		if err != nil {
			t.Error(err)
		}
		began <- done
	}()

	// While draining, new leases are refused even for a gate that would accept
	// the Transitioning phase.
	lifecycletest.Eventually(t, time.Second, func() bool {
		st, _ := ex.Status("server/state-store")
		return st.State == lifecycle.StateRestoring
	})
	anyPhase := ex.Require("server/state-store", lifecycle.PhaseTransitioning)
	if _, err := anyPhase.Acquire(t.Context(), lifecycle.FailFast); !lifecycle.IsNotReady(err) {
		t.Fatalf("lease during drain: %v", err)
	}
	if _, err := h.BeginTransition(t.Context(), lifecycle.StateRestoring); !errors.Is(err, lifecycle.ErrTransitionInProgress) {
		t.Fatalf("nested begin: %v", err)
	}

	select {
	case <-began:
		t.Fatal("BeginTransition returned while a lease was held")
	case <-time.After(50 * time.Millisecond):
	}
	held.Release()
	done := <-began

	if _, err := gate.Acquire(t.Context(), lifecycle.FailFast); !lifecycle.IsNotReady(err) {
		t.Fatalf("lease before Done: %v", err)
	}
	if err := done(lifecycle.StateBooting); !errors.Is(err, lifecycle.ErrInvalidTransition) {
		t.Fatalf("invalid final state: %v", err)
	}
	if err := done(lifecycle.StateServing); err != nil {
		t.Fatal(err)
	}
	if err := done(lifecycle.StateServing); err == nil {
		t.Fatal("Done twice should fail")
	}
	if st := h.Status(); st.Generation != 2 {
		t.Fatalf("generation %d, want 2", st.Generation)
	}
	lease, err := gate.Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
}

func TestBeginTransition_DrainTimeoutProceedsAndReports(t *testing.T) {
	clock := lifecycletest.NewClock()
	rec := &lifecycletest.Recorder{}
	ex := newExchange(t, lifecycle.WithClock(clock), lifecycle.WithObserver(rec), lifecycle.WithDrainTimeout(time.Second))
	h := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	mustTransition(t, h, lifecycle.StateServing)
	lease, err := ex.Require("server/state-store", lifecycle.PhaseReady).Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	began := make(chan lifecycle.Done, 1)
	go func() {
		done, _ := h.BeginTransition(t.Context(), lifecycle.StateRestoring)
		began <- done
	}()
	lifecycletest.Eventually(t, time.Second, func() bool {
		clock.Advance(500 * time.Millisecond)
		select {
		case done := <-began:
			began <- done
			return true
		default:
			return false
		}
	})
	if err := (<-began)(lifecycle.StateServing); err != nil {
		t.Fatal(err)
	}
	lifecycletest.Eventually(t, time.Second, func() bool { return len(rec.DrainTimeouts()) == 1 })
}

// TestLeases_DeadlockShape reproduces the shape of the Consul FSM bug with two
// plain mutexes taken in opposite orders:
//
//	subscriber: publisher lock, then (inside a lease) state lock
//	restorer:   BeginTransition, state lock, then publisher lock
//
// Without the lease this deadlocks; with it, the restorer only takes the state
// lock once no subscriber can be waiting for it.
func TestLeases_DeadlockShape(t *testing.T) {
	ex := newExchange(t)
	h := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	mustTransition(t, h, lifecycle.StateServing)
	gate := ex.Require("server/state-store", lifecycle.PhaseReady)

	var publisher sync.Mutex
	var state sync.RWMutex
	var refused, served, reads, swaps atomic.Int64

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				publisher.Lock()
				lease, err := gate.Acquire(ctx, lifecycle.FailFast)
				if err != nil {
					refused.Add(1)
					publisher.Unlock()
					continue
				}
				state.RLock()
				reads.Add(1) // stands in for reading the state store
				state.RUnlock()
				lease.Release()
				served.Add(1)
				publisher.Unlock()
			}
		}()
	}

	restores := 0
	for ; restores < 500 && ctx.Err() == nil; restores++ {
		done, err := h.BeginTransition(ctx, lifecycle.StateRestoring)
		if err != nil {
			t.Fatal(err)
		}
		state.Lock()
		publisher.Lock()
		swaps.Add(1) // stands in for swapping the state and refreshing the publisher
		publisher.Unlock()
		state.Unlock()
		if err := done(lifecycle.StateServing); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	wg.Wait()
	if restores < 500 {
		t.Fatalf("only %d restores completed before the deadline: deadlock", restores)
	}
	if served.Load() == 0 || reads.Load() != served.Load() || swaps.Load() != int64(restores) {
		t.Fatalf("served=%d reads=%d swaps=%d restores=%d", served.Load(), reads.Load(), swaps.Load(), restores)
	}
	t.Logf("restores=%d served=%d refused=%d", restores, served.Load(), refused.Load())
}

func TestLeases_ConcurrentStress(t *testing.T) {
	ex := newExchange(t)
	h := mustRegister(t, ex, lifecycle.Restorable("c"))
	mustTransition(t, h, lifecycle.StateServing)
	gate := ex.Require("c", lifecycle.PhaseReady)

	var inside atomic.Int64
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				lease, err := gate.Acquire(ctx, lifecycle.FailFast)
				if err != nil {
					continue
				}
				inside.Add(1)
				inside.Add(-1)
				lease.Release()
			}
		}()
	}
	for i := 0; i < 2000; i++ {
		done, err := h.BeginTransition(ctx, lifecycle.StateRestoring)
		if err != nil {
			t.Fatal(err)
		}
		if n := inside.Load(); n != 0 {
			t.Fatalf("%d lease holders inside after drain", n)
		}
		if n := ex.Leases("c"); n != 0 {
			t.Fatalf("%d leases after drain", n)
		}
		if err := done(lifecycle.StateServing); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	wg.Wait()
	lifecycletest.AssertDrained(t, ex, "c")
}
