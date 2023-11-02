// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervise(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	runCh := make(chan struct{})
	stopCh := make(chan struct{})
	errCh := make(chan error)

	task := func(taskCtx context.Context) error {
		runCh <- struct{}{}

		select {
		case err := <-errCh:
			return err
		case <-taskCtx.Done():
			stopCh <- struct{}{}
			return taskCtx.Err()
		}
	}

	lease := newTestLease()

	go newSupervisor(task, lease).run(ctx)

	select {
	case <-runCh:
		t.Fatal("task should not be running before lease is held")
	case <-time.After(500 * time.Millisecond):
	}

	lease.acquired()

	select {
	case <-runCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task not running after lease is acquired")
	}

	select {
	case <-stopCh:
		t.Fatal("task should not have stopped before lease is lost")
	case <-time.After(500 * time.Millisecond):
	}

	lease.lost()

	select {
	case <-stopCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task still running after lease was lost")
	}

	select {
	case <-runCh:
		t.Fatal("task should not be run again before lease is re-acquired")
	case <-time.After(500 * time.Millisecond):
	}

	lease.acquired()

	select {
	case <-runCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task not running after lease is re-acquired")
	}

	errCh <- errors.New("KABOOM")

	select {
	case <-runCh:
	case <-time.After(2 * time.Second):
		t.Fatal("task was not restarted")
	}

	cancel()

	select {
	case <-stopCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task still running after parent context was canceled")
	}
}

func newTestLease() *testLease {
	return &testLease{ch: make(chan struct{}, 1)}
}

type testLease struct {
	held atomic.Bool
	ch   chan struct{}
}

func (l *testLease) Held() bool               { return l.held.Load() }
func (l *testLease) Changed() <-chan struct{} { return l.ch }

func (l *testLease) acquired() { l.setHeld(true) }
func (l *testLease) lost()     { l.setHeld(false) }

func (l *testLease) setHeld(held bool) {
	l.held.Store(held)

	select {
	case l.ch <- struct{}{}:
	default:
	}
}
