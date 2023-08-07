// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controller

import (
	"context"
	"time"

	"github.com/hashicorp/consul/lib/retry"
)

// flapThreshold is the minimum amount of time between restarts for us *not* to
// consider a controller to be stuck in a crash-loop.
const flapThreshold = 2 * time.Second

// supervisor keeps a task running, restarting it on-error, for as long as the
// given lease is held. When the lease is lost, the context given to the task
// will be canceled. If the task persistently fails (i.e. the controller is in
// a crash-loop) supervisor will use exponential backoff to delay restarts.
type supervisor struct {
	task  task
	lease Lease

	running    bool
	startedAt  time.Time
	errCh      chan error
	cancelTask context.CancelFunc

	backoff        *retry.Waiter
	backoffUntil   time.Time
	backoffTimerCh <-chan time.Time
}

func newSupervisor(task task, lease Lease) *supervisor {
	return &supervisor{
		task:  task,
		lease: lease,
		errCh: make(chan error),
		backoff: &retry.Waiter{
			MinFailures: 1,
			MinWait:     500 * time.Millisecond,
			MaxWait:     time.Minute,
			Jitter:      retry.NewJitter(25),
		},
	}
}

type task func(context.Context) error

func (s *supervisor) run(ctx context.Context) {
	for {
		if s.shouldStart() {
			s.startTask(ctx)
		} else if s.shouldStop() {
			s.stopTask()
		}

		select {
		// Outer context canceled.
		case <-ctx.Done():
			if s.cancelTask != nil {
				s.cancelTask()
			}
			return

		// Task stopped running.
		case <-s.errCh:
			stopBackoffTimer := s.handleError()
			if stopBackoffTimer != nil {
				defer stopBackoffTimer()
			}

		// Unblock when the lease is acquired/lost, or the backoff timer fires.
		case <-s.lease.Changed():
		case <-s.backoffTimerCh:
		}
	}
}

func (s *supervisor) shouldStart() bool {
	if s.running {
		return false
	}

	if !s.lease.Held() {
		return false
	}

	if time.Now().Before(s.backoffUntil) {
		return false
	}

	return true
}

func (s *supervisor) startTask(ctx context.Context) {
	if s.cancelTask != nil {
		s.cancelTask()
	}

	taskCtx, cancelTask := context.WithCancel(ctx)
	s.cancelTask = cancelTask
	s.startedAt = time.Now()
	s.running = true

	go func() {
		err := s.task(taskCtx)

		select {
		case s.errCh <- err:
		case <-ctx.Done():
		}
	}()
}

func (s *supervisor) shouldStop() bool {
	return s.running && !s.lease.Held()
}

func (s *supervisor) stopTask() {
	s.cancelTask()
	s.backoff.Reset()
	s.running = false
}

func (s *supervisor) handleError() func() bool {
	s.running = false

	if time.Since(s.startedAt) > flapThreshold {
		s.backoff.Reset()
		s.backoffUntil = time.Time{}
	} else {
		delay := s.backoff.WaitDuration()
		s.backoffUntil = time.Now().Add(delay)

		timer := time.NewTimer(delay)
		s.backoffTimerCh = timer.C
		return timer.Stop
	}

	return nil
}
