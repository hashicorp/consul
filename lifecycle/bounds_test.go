// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/lifecycle"
	"github.com/hashicorp/consul/lifecycle/lifecycletest"
)

// blockingObserver blocks the dispatcher until released, like a stuck sink.
type blockingObserver struct {
	lifecycle.NoopObserver
	release chan struct{}
}

func (o blockingObserver) OnEvent(lifecycle.ControlEvent) { <-o.release }

func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// TestBounds_WriteStorm checks that a storm of transitions with a subscriber
// that never reads and an observer that never returns keeps memory flat, never
// blocks the writer, and ends in a resync and a dropped-notification count.
func TestBounds_WriteStorm(t *testing.T) {
	m := &recordingMetrics{}
	stuck := blockingObserver{release: make(chan struct{})}
	ex := lifecycle.NewExchange(lifecycle.WithMetrics(m), lifecycle.WithObserver(stuck), lifecycle.WithObserverQueue(64))
	defer ex.Close()
	defer close(stuck.release)

	h := mustRegister(t, ex, lifecycle.Simple("storm"))
	mustTransition(t, h, lifecycle.StateReady)
	sub, err := ex.Subscribe(t.Context(), lifecycle.WithBuffer(32))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	const n = 100000
	write := func() {
		for i := 0; i < n/2; i++ {
			_ = h.Transition(lifecycle.StateDegraded, lifecycle.WithAttributes(map[string]string{"i": fmt.Sprint(i)}))
			_ = h.Transition(lifecycle.StateReady)
		}
	}
	write() // warm up allocations
	before := heapInUse()
	start := time.Now()
	write()
	elapsed := time.Since(start)
	after := heapInUse()

	if after > before && after-before > 4<<20 {
		t.Fatalf("heap grew by %d bytes over %d transitions", after-before, n)
	}
	if elapsed > 20*time.Second {
		t.Fatalf("writer took %s; it should never block on readers", elapsed)
	}

	// Overflow dropped everything queued, including the unread initial snapshot,
	// so the first read is the resync snapshot.
	u := next(t, sub)
	if u.Kind != lifecycle.UpdateSnapshot || !u.Resync {
		t.Fatalf("expected resync after overflow, got %+v", u)
	}
	t.Logf("heap before=%d after=%d, %d transitions in %s", before, after, n, elapsed)
}

func TestBounds_DroppedObserverNotificationsAreCounted(t *testing.T) {
	m := &recordingMetrics{}
	gate := blockingObserver{release: make(chan struct{})}
	ex := lifecycle.NewExchange(lifecycle.WithMetrics(m), lifecycle.WithObserver(gate), lifecycle.WithObserverQueue(4))
	defer ex.Close()

	h := mustRegister(t, ex, lifecycle.Simple("a"))
	for i := 0; i < 50; i++ {
		_ = h.Transition(lifecycle.StateReady)
		_ = h.Transition(lifecycle.StateDegraded)
	}
	close(gate.release)
	_ = h.Transition(lifecycle.StateReady)
	lifecycletest.Eventually(t, 2*time.Second, func() bool { return m.counter("lifecycle.observer_dropped") > 0 })
}

func TestBounds_NoGoroutineLeak(t *testing.T) {
	base := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		clock := lifecycletest.NewClock()
		ex := lifecycle.NewExchange(lifecycle.WithClock(clock), lifecycle.WithObserver(&lifecycletest.Recorder{}))
		h, err := ex.Register(lifecycle.Simple("a").WithMaxDuration(lifecycle.StateStarting, time.Second))
		if err != nil {
			t.Fatal(err)
		}
		sub, _ := ex.Subscribe(t.Context())
		_ = h.Deregister()
		sub.Close()
		ex.Close()
	}
	lifecycletest.Eventually(t, 2*time.Second, func() bool { return runtime.NumGoroutine() <= base },
		"goroutines %d > baseline %d", runtime.NumGoroutine(), base)
}
