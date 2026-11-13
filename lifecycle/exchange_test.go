// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/lifecycle"
	"github.com/hashicorp/consul/lifecycle/lifecycletest"
)

const (
	stateIdle    lifecycle.StateName = "Idle"
	stateSyncing lifecycle.StateName = "Syncing"
	stateLive    lifecycle.StateName = "Live"
	stateLagging lifecycle.StateName = "Lagging"
	stateBroken  lifecycle.StateName = "Broken"
)

// customSpec is a component-defined state machine that uses none of the preset
// state names, to show that states are owned by the component.
func customSpec(name lifecycle.ComponentName) lifecycle.ComponentSpec {
	return lifecycle.ComponentSpec{
		Name:    name,
		Initial: stateIdle,
		States: map[lifecycle.StateName]lifecycle.StateSpec{
			stateIdle:    {Phase: lifecycle.PhaseStarting, Next: []lifecycle.StateName{stateSyncing}},
			stateSyncing: {Phase: lifecycle.PhaseTransitioning, Next: []lifecycle.StateName{stateLive, stateBroken}},
			stateLive:    {Phase: lifecycle.PhaseReady, Next: []lifecycle.StateName{stateLagging, stateSyncing, stateBroken}},
			stateLagging: {Phase: lifecycle.PhaseDegraded, Next: []lifecycle.StateName{stateLive, stateBroken}},
			stateBroken:  {Phase: lifecycle.PhaseFailed, Next: []lifecycle.StateName{stateSyncing}},
		},
	}
}

func newExchange(t *testing.T, opts ...lifecycle.Option) *lifecycle.Exchange {
	t.Helper()
	ex := lifecycle.NewExchange(opts...)
	t.Cleanup(ex.Close)
	return ex
}

func mustRegister(t *testing.T, ex *lifecycle.Exchange, spec lifecycle.ComponentSpec) *lifecycle.Handle {
	t.Helper()
	h, err := ex.Register(spec)
	if err != nil {
		t.Fatalf("register %s: %v", spec.Name, err)
	}
	return h
}

func mustTransition(t *testing.T, h *lifecycle.Handle, to lifecycle.StateName, opts ...lifecycle.TransitionOption) {
	t.Helper()
	if err := h.Transition(to, opts...); err != nil {
		t.Fatalf("transition %s -> %s: %v", h.Name(), to, err)
	}
}

func TestRegister_ValidatesSpec(t *testing.T) {
	ex := newExchange(t)
	bad := []lifecycle.ComponentSpec{
		{},
		{Name: "a"},
		{Name: "a", Initial: "X", States: map[lifecycle.StateName]lifecycle.StateSpec{"Y": {Phase: lifecycle.PhaseReady}}},
		{Name: "a", Initial: "X", States: map[lifecycle.StateName]lifecycle.StateSpec{"X": {}}},
		{Name: "a", Initial: "X", States: map[lifecycle.StateName]lifecycle.StateSpec{"X": {Phase: lifecycle.PhaseReady, Next: []lifecycle.StateName{"Z"}}}},
		{Name: "a", Initial: "X", States: map[lifecycle.StateName]lifecycle.StateSpec{"X": {Phase: lifecycle.PhaseReady, MaxDuration: -1}}},
	}
	for i, spec := range bad {
		if _, err := ex.Register(spec); !errors.Is(err, lifecycle.ErrInvalidSpec) {
			t.Errorf("spec %d: got %v, want ErrInvalidSpec", i, err)
		}
	}

	for _, spec := range []lifecycle.ComponentSpec{
		lifecycle.Simple("simple"), lifecycle.Restorable("restorable"), lifecycle.Reloadable("reloadable"), customSpec("custom"),
	} {
		if err := spec.Validate(); err != nil {
			t.Errorf("%s: %v", spec.Name, err)
		}
		mustRegister(t, ex, spec)
	}
	if _, err := ex.Register(lifecycle.Simple("simple")); !errors.Is(err, lifecycle.ErrAlreadyRegistered) {
		t.Fatalf("duplicate: got %v", err)
	}
}

func TestRegister_SpecIsCopied(t *testing.T) {
	ex := newExchange(t)
	spec := customSpec("c")
	h := mustRegister(t, ex, spec)
	// Mutating the caller's spec must not change the registered machine.
	delete(spec.States, stateSyncing)
	mustTransition(t, h, stateSyncing)
}

func TestTransition_ComponentStatesPhasesAndGeneration(t *testing.T) {
	rec := &lifecycletest.Recorder{}
	ex := newExchange(t, lifecycle.WithObserver(rec))
	h := mustRegister(t, ex, customSpec("agent/sync"))

	st := h.Status()
	if st.State != stateIdle || st.Phase != lifecycle.PhaseStarting || st.Generation != 0 {
		t.Fatalf("initial status %+v", st)
	}

	err := h.Transition(stateLive)
	if !errors.Is(err, lifecycle.ErrInvalidTransition) {
		t.Fatalf("Idle -> Live: got %v, want ErrInvalidTransition", err)
	}

	mustTransition(t, h, stateSyncing, lifecycle.WithReason("first sync"))
	mustTransition(t, h, stateLive)
	if g := h.Status().Generation; g != 1 {
		t.Fatalf("generation after first Ready = %d, want 1", g)
	}
	// Degraded and back is the same generation: nothing was replaced.
	mustTransition(t, h, stateLagging)
	mustTransition(t, h, stateLive)
	if g := h.Status().Generation; g != 1 {
		t.Fatalf("generation after Degraded -> Ready = %d, want 1", g)
	}
	// A resync replaces the data: new generation.
	mustTransition(t, h, stateSyncing)
	mustTransition(t, h, stateLive)
	if g := h.Status().Generation; g != 2 {
		t.Fatalf("generation after resync = %d, want 2", g)
	}

	lifecycletest.AssertTransitions(t, rec, "agent/sync",
		stateIdle, stateSyncing, stateLive, stateLagging, stateLive, stateSyncing, stateLive)
	lifecycletest.Eventually(t, time.Second, func() bool { return len(rec.Invalid()) == 1 })
	if got := rec.Invalid()[0]; got != "agent/sync:Idle->Live" {
		t.Fatalf("invalid transition recorded as %q", got)
	}

	var last uint64
	for _, ev := range rec.Events() {
		if ev.Seq != last+1 {
			t.Fatalf("seq %d after %d", ev.Seq, last)
		}
		if ev.SchemaVersion != lifecycle.SchemaVersion {
			t.Fatalf("schema version %d", ev.SchemaVersion)
		}
		last = ev.Seq
	}
}

func TestAttributes_MergeDeleteReplaceAndLimits(t *testing.T) {
	ex := newExchange(t, lifecycle.WithAttributeLimits(3, 8, 8))
	h, err := ex.Register(lifecycle.Simple("s"), lifecycle.WithAttributes(map[string]string{"a": "1"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Update(lifecycle.WithAttributes(map[string]string{"b": "2"})); err != nil {
		t.Fatal(err)
	}
	if got := h.Status().Attributes; len(got) != 2 || got["a"] != "1" || got["b"] != "2" {
		t.Fatalf("merge: %v", got)
	}
	if err := h.Update(lifecycle.WithAttributes(map[string]string{"a": ""})); err != nil {
		t.Fatal(err)
	}
	if got := h.Status().Attributes; len(got) != 1 || got["b"] != "2" {
		t.Fatalf("delete: %v", got)
	}
	mustTransition(t, h, lifecycle.StateReady, lifecycle.WithReplaceAttributes(), lifecycle.WithAttributes(map[string]string{"c": "3"}))
	if got := h.Status().Attributes; len(got) != 1 || got["c"] != "3" {
		t.Fatalf("replace: %v", got)
	}

	// The returned status is a copy.
	st := h.Status()
	st.Attributes["c"] = "mutated"
	if h.Status().Attributes["c"] != "3" {
		t.Fatal("Status leaked internal attributes map")
	}

	for name, attrs := range map[string]map[string]string{
		"too many keys": {"k1": "v", "k2": "v", "k3": "v"},
		"long key":      {"0123456789": "v"},
		"long value":    {"k": "0123456789"},
	} {
		if err := h.Update(lifecycle.WithAttributes(attrs)); !errors.Is(err, lifecycle.ErrAttributesTooLarge) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if got := h.Status().Attributes; len(got) != 1 || got["c"] != "3" {
		t.Fatalf("rejected update changed attributes: %v", got)
	}
}

func TestLimits_MaxComponents(t *testing.T) {
	ex := newExchange(t, lifecycle.WithMaxComponents(2))
	mustRegister(t, ex, lifecycle.Simple("a"))
	h := mustRegister(t, ex, lifecycle.Simple("b"))
	if _, err := ex.Register(lifecycle.Simple("c")); !errors.Is(err, lifecycle.ErrTooManyComponents) {
		t.Fatalf("got %v", err)
	}
	if err := h.Deregister(); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, ex, lifecycle.Simple("c"))
}

func TestNilExchangeAndHandleAreNoops(t *testing.T) {
	var ex *lifecycle.Exchange
	h, err := ex.Register(lifecycle.Simple("a"))
	if err != nil || h != nil {
		t.Fatalf("Register on nil: %v %v", h, err)
	}
	if err := h.Transition(lifecycle.StateReady); err != nil {
		t.Fatal(err)
	}
	done, err := h.BeginTransition(t.Context(), lifecycle.StateStopping)
	if err != nil || done(lifecycle.StateStopped) != nil {
		t.Fatalf("BeginTransition on nil: %v", err)
	}
	lease, err := ex.Require("a", lifecycle.PhaseReady).Acquire(t.Context(), lifecycle.FailFast)
	if err != nil {
		t.Fatalf("gate on nil exchange: %v", err)
	}
	lease.Release()
	if _, ok := ex.Status("a"); ok {
		t.Fatal("status on nil exchange")
	}
	ex.Close()
}

func TestDeregister(t *testing.T) {
	rec := &lifecycletest.Recorder{}
	ex := newExchange(t, lifecycle.WithObserver(rec))
	h := mustRegister(t, ex, lifecycle.Simple("server/peer/1"))
	mustTransition(t, h, lifecycle.StateReady)
	if err := h.Deregister(); err != nil {
		t.Fatal(err)
	}
	if err := h.Deregister(); !errors.Is(err, lifecycle.ErrNotRegistered) {
		t.Fatalf("second deregister: %v", err)
	}
	if err := h.Transition(lifecycle.StateDegraded); !errors.Is(err, lifecycle.ErrNotRegistered) {
		t.Fatalf("transition after deregister: %v", err)
	}
	_, err := ex.Require("server/peer/1", lifecycle.PhaseReady).Acquire(t.Context(), lifecycle.FailFast)
	if !errors.Is(err, lifecycle.ErrNotRegistered) || !lifecycle.IsNotReady(err) {
		t.Fatalf("gate on removed component: %v", err)
	}
	lifecycletest.Eventually(t, time.Second, func() bool {
		evs := rec.Events()
		return len(evs) > 0 && evs[len(evs)-1].Kind == lifecycle.EventRemoved
	})
	// The name can be reused.
	mustRegister(t, ex, lifecycle.Simple("server/peer/1"))
}

func TestStuckWatchdog(t *testing.T) {
	clock := lifecycletest.NewClock()
	rec := &lifecycletest.Recorder{}
	ex := newExchange(t, lifecycle.WithClock(clock), lifecycle.WithObserver(rec))
	spec := lifecycle.Restorable("server/state-store").WithMaxDuration(lifecycle.StateRestoring, time.Minute)
	h := mustRegister(t, ex, spec)
	mustTransition(t, h, lifecycle.StateServing)

	// Left in time: no report.
	mustTransition(t, h, lifecycle.StateRestoring)
	clock.Advance(30 * time.Second)
	mustTransition(t, h, lifecycle.StateServing)
	clock.Advance(time.Hour)
	lifecycletest.AssertNeverStuck(t, rec, "server/state-store")

	// Stuck: exactly one report.
	mustTransition(t, h, lifecycle.StateRestoring, lifecycle.WithReason("snapshot install"))
	clock.Advance(2 * time.Minute)
	clock.Advance(2 * time.Minute)
	lifecycletest.Eventually(t, time.Second, func() bool { return len(rec.Stuck()) == 1 })
	time.Sleep(20 * time.Millisecond)
	stuck := rec.Stuck()
	if len(stuck) != 1 || stuck[0].State != lifecycle.StateRestoring || stuck[0].Reason != "snapshot install" {
		t.Fatalf("stuck reports %+v", stuck)
	}
}

type panicObserver struct{ lifecycle.NoopObserver }

func (panicObserver) OnEvent(lifecycle.ControlEvent) { panic("boom") }

func TestObservers_PanicIsRecoveredAndOthersStillCalled(t *testing.T) {
	rec := &lifecycletest.Recorder{}
	log := &recordingLogger{}
	ex := newExchange(t, lifecycle.WithLogger(log), lifecycle.WithObserver(panicObserver{}), lifecycle.WithObserver(rec))
	h := mustRegister(t, ex, lifecycle.Simple("a"))
	mustTransition(t, h, lifecycle.StateReady)
	lifecycletest.AssertTransitions(t, rec, "a", lifecycle.StateStarting, lifecycle.StateReady)
	lifecycletest.Eventually(t, time.Second, func() bool { return log.count("lifecycle observer panicked") == 2 })
}

func TestClose(t *testing.T) {
	ex := lifecycle.NewExchange(lifecycle.WithObserver(&lifecycletest.Recorder{}))
	h := mustRegister(t, ex, lifecycle.Simple("a"))
	sub, err := ex.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ex.Close()
	ex.Close()
	if _, err := ex.Register(lifecycle.Simple("b")); !errors.Is(err, lifecycle.ErrClosed) {
		t.Fatalf("register after close: %v", err)
	}
	if _, err := ex.Subscribe(t.Context()); !errors.Is(err, lifecycle.ErrClosed) {
		t.Fatalf("subscribe after close: %v", err)
	}
	// Components keep working after Close (shutdown order is not guaranteed).
	mustTransition(t, h, lifecycle.StateReady)
	if _, err := sub.Next(t.Context()); err != nil {
		t.Fatalf("queued snapshot should still be readable: %v", err)
	}
	if _, err := sub.Next(t.Context()); !errors.Is(err, lifecycle.ErrClosed) {
		t.Fatalf("next after close: %v", err)
	}
}

type recordingLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (l *recordingLogger) add(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, level+" "+msg)
}
func (l *recordingLogger) Debug(msg string, _ ...interface{}) { l.add("DEBUG", msg) }
func (l *recordingLogger) Info(msg string, _ ...interface{})  { l.add("INFO", msg) }
func (l *recordingLogger) Warn(msg string, _ ...interface{})  { l.add("WARN", msg) }
func (l *recordingLogger) Error(msg string, _ ...interface{}) { l.add("ERROR", msg) }
func (l *recordingLogger) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, m := range l.msgs {
		if strings.Contains(m, substr) {
			n++
		}
	}
	return n
}

type recordingMetrics struct {
	mu       sync.Mutex
	gauges   map[string]float32
	counters map[string]float32
}

func key(name []string, labels map[string]string) string {
	k := strings.Join(name, ".")
	for _, l := range []string{"component", "state", "from", "to"} {
		if v, ok := labels[l]; ok {
			k += ";" + l + "=" + v
		}
	}
	return k
}

func (m *recordingMetrics) SetGauge(name []string, v float32, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = map[string]float32{}
	}
	m.gauges[key(name, labels)] = v
}

func (m *recordingMetrics) IncrCounter(name []string, v float32, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = map[string]float32{}
	}
	m.counters[key(name, labels)] += v
}

func (m *recordingMetrics) gauge(k string) float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gauges[k]
}

func (m *recordingMetrics) counter(k string) float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[k]
}

func TestBuiltInMetricsAndLogging(t *testing.T) {
	m := &recordingMetrics{}
	log := &recordingLogger{}
	ex := newExchange(t, lifecycle.WithMetrics(m), lifecycle.WithLogger(log))
	h := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	mustTransition(t, h, lifecycle.StateServing)
	done, err := h.BeginTransition(t.Context(), lifecycle.StateRestoring)
	if err != nil {
		t.Fatal(err)
	}
	if err := done(lifecycle.StateServing); err != nil {
		t.Fatal(err)
	}
	_ = h.Transition(lifecycle.StateBooting) // invalid

	lifecycletest.Eventually(t, time.Second, func() bool {
		return m.counter("lifecycle.invalid_transition;component=server/state-store") == 1
	})
	if got := m.gauge("lifecycle.phase;component=server/state-store"); got != float32(lifecycle.PhaseReady) {
		t.Fatalf("phase gauge %v", got)
	}
	if m.gauge("lifecycle.state;component=server/state-store;state=Serving") != 1 ||
		m.gauge("lifecycle.state;component=server/state-store;state=Restoring") != 0 {
		t.Fatalf("state gauges %v", m.gauges)
	}
	if got := m.counter("lifecycle.transition;component=server/state-store;from=Serving;to=Restoring"); got != 1 {
		t.Fatalf("transition counter %v", got)
	}
	if log.count("INFO lifecycle transition") != 2 || log.count("WARN lifecycle transition rejected") != 1 {
		t.Fatalf("logs %v", log.msgs)
	}
}
