// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/consul/lifecycle"
)

func next(t *testing.T, s *lifecycle.Subscription) lifecycle.Update {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	u, err := s.Next(ctx)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	return u
}

func TestSubscribe_SnapshotThenOrderedEvents(t *testing.T) {
	ex := newExchange(t)
	store := mustRegister(t, ex, lifecycle.Restorable("server/state-store"))
	mustRegister(t, ex, lifecycle.Simple("server/acl-resolver"))
	mustTransition(t, store, lifecycle.StateServing)

	sub, err := ex.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	snap := next(t, sub)
	if snap.Kind != lifecycle.UpdateSnapshot || snap.Resync || len(snap.Statuses) != 2 {
		t.Fatalf("snapshot %+v", snap)
	}
	if snap.Statuses[0].Component != "server/acl-resolver" || snap.Statuses[1].State != lifecycle.StateServing {
		t.Fatalf("snapshot statuses %+v", snap.Statuses)
	}

	done, err := store.BeginTransition(t.Context(), lifecycle.StateRestoring)
	if err != nil {
		t.Fatal(err)
	}
	if err := done(lifecycle.StateServing); err != nil {
		t.Fatal(err)
	}

	u1, u2 := next(t, sub), next(t, sub)
	if u1.Kind != lifecycle.UpdateEvent || u1.Event.To.State != lifecycle.StateRestoring || u1.Seq <= snap.Seq {
		t.Fatalf("first event %+v (snapshot seq %d)", u1, snap.Seq)
	}
	if u2.Event.To.State != lifecycle.StateServing || u2.Seq != u1.Seq+1 || u2.Event.To.Generation != 2 {
		t.Fatalf("second event %+v", u2)
	}
}

func TestSubscribe_Filters(t *testing.T) {
	ex := newExchange(t)
	srv := mustRegister(t, ex, lifecycle.Simple("server/a"))
	agent := mustRegister(t, ex, lifecycle.Simple("agent/a"))

	byPrefix, _ := ex.Subscribe(t.Context(), lifecycle.WithPrefix("agent/"))
	byName, _ := ex.Subscribe(t.Context(), lifecycle.WithComponents("server/a"))
	if s := next(t, byPrefix); len(s.Statuses) != 1 || s.Statuses[0].Component != "agent/a" {
		t.Fatalf("prefix snapshot %+v", s.Statuses)
	}
	if s := next(t, byName); len(s.Statuses) != 1 || s.Statuses[0].Component != "server/a" {
		t.Fatalf("name snapshot %+v", s.Statuses)
	}
	mustTransition(t, srv, lifecycle.StateReady)
	mustTransition(t, agent, lifecycle.StateReady)
	if u := next(t, byPrefix); u.Event.Component() != "agent/a" {
		t.Fatalf("prefix event %+v", u)
	}
	if u := next(t, byName); u.Event.Component() != "server/a" {
		t.Fatalf("name event %+v", u)
	}
}

func TestSubscribe_SlowReaderGetsResyncNotUnboundedQueue(t *testing.T) {
	ex := newExchange(t)
	h := mustRegister(t, ex, lifecycle.Simple("a"))
	sub, err := ex.Subscribe(t.Context(), lifecycle.WithBuffer(4))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	// The writer never blocks, however far behind the reader is.
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		_ = h.Transition(lifecycle.StateReady)
		for i := 0; i < 1000; i++ {
			_ = h.Transition(lifecycle.StateDegraded)
			_ = h.Transition(lifecycle.StateReady)
		}
	}()
	select {
	case <-writeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("writer blocked on a slow subscriber")
	}

	u := next(t, sub)
	if u.Kind != lifecycle.UpdateSnapshot || !u.Resync {
		t.Fatalf("want a resync snapshot, got %+v", u)
	}
	if u.Statuses[0].State != lifecycle.StateReady {
		t.Fatalf("resync snapshot is not current: %+v", u.Statuses)
	}
	resyncSeq := u.Seq

	if err := h.Transition(lifecycle.StateDegraded); err != nil {
		t.Fatal(err)
	}
	if u := next(t, sub); u.Kind != lifecycle.UpdateEvent || u.Seq != resyncSeq+1 {
		t.Fatalf("event after resync %+v (resync seq %d)", u, resyncSeq)
	}
}

func TestSubscribe_EndsWithContext(t *testing.T) {
	ex := newExchange(t)
	ctx, cancel := context.WithCancel(t.Context())
	sub, err := ex.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next(t, sub)
	cancel()
	_, err = sub.Next(t.Context())
	if !errors.Is(err, lifecycle.ErrClosed) {
		t.Fatalf("after cancel: %v", err)
	}
}
