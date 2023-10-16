// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// View receives events from, and return results to, Materializer. A view is
// responsible for converting the pbsubscribe.Event.Payload into the local
// type, and storing it so that it can be returned by Result().
type View interface {
	// Update is called when one or more events are received. The first call will
	// include _all_ events in the initial snapshot which may be an empty set.
	// Subsequent calls will contain one or more update events in the order they
	// are received.
	Update(events []*pbsubscribe.Event) error

	// Result returns the type-specific cache result based on the state. When no
	// events have been delivered yet the result should be an empty value type
	// suitable to return to clients in case there is an empty result on the
	// servers. The index the materialized view represents is maintained
	// separately and passed in in case the return type needs an Index field
	// populating. This allows implementations to not worry about maintaining
	// indexes seen during Update.
	Result(index uint64) interface{}

	// Reset the view to the zero state, done in preparation for receiving a new
	// snapshot.
	Reset()
}

// Result returned from the View.
type Result struct {
	Index uint64
	Value interface{}
	// Cached is true if the requested value was already available locally. If
	// the value is false, it indicates that GetFromView had to wait for an update,
	Cached bool
}

type Deps struct {
	View    View
	Logger  hclog.Logger
	Waiter  *retry.Waiter
	Request func(index uint64) *pbsubscribe.SubscribeRequest
}

// materializer consumes the event stream, handling any framing events, and
// allows for querying the materialized view.
type materializer struct {
	retryWaiter *retry.Waiter
	logger      hclog.Logger

	// lock protects the mutable state - all fields below it must only be accessed
	// while holding lock.
	lock     sync.Mutex
	index    uint64
	view     View
	updateCh chan struct{}
	err      error
}

func newMaterializer(logger hclog.Logger, view View, waiter *retry.Waiter) *materializer {
	m := materializer{
		view:        view,
		retryWaiter: waiter,
		logger:      logger,
		updateCh:    make(chan struct{}),
	}
	if m.retryWaiter == nil {
		m.retryWaiter = defaultWaiter()
	}
	return &m
}

// Query blocks until the index of the View is greater than opts.MinIndex,
// or the context is cancelled.
func (m *materializer) query(ctx context.Context, minIndex uint64) (Result, error) {
	m.lock.Lock()

	result := Result{
		Index: m.index,
		Value: m.view.Result(m.index),
	}

	updateCh := m.updateCh
	m.lock.Unlock()

	// If our index is > req.Index return right away. If index is zero then we
	// haven't loaded a snapshot at all yet which means we should wait for one on
	// the update chan.
	if result.Index > 0 && result.Index > minIndex {
		result.Cached = true
		return result, nil
	}

	for {
		select {
		case <-updateCh:
			// View updated, return the new result
			m.lock.Lock()
			result.Index = m.index

			switch {
			case m.err != nil:
				err := m.err
				m.lock.Unlock()
				return result, err
			case result.Index <= minIndex:
				// get a reference to the new updateCh, the previous one was closed
				updateCh = m.updateCh
				m.lock.Unlock()
				continue
			}

			result.Value = m.view.Result(m.index)
			m.lock.Unlock()
			return result, nil

		case <-ctx.Done():
			// Update the result value to the latest because callers may still
			// use the value when the error is context.DeadlineExceeded
			m.lock.Lock()
			result.Value = m.view.Result(m.index)
			m.lock.Unlock()
			return result, ctx.Err()
		}
	}
}

func (m *materializer) currentIndex() uint64 {
	var resp uint64

	m.lock.Lock()
	resp = m.index
	m.lock.Unlock()

	return resp
}

// notifyUpdateLocked closes the current update channel and recreates a new
// one. It must be called while holding the m.lock lock.
func (m *materializer) notifyUpdateLocked(err error) {
	m.err = err
	close(m.updateCh)
	m.updateCh = make(chan struct{})
}

// reset clears the state ready to start a new stream from scratch.
func (m *materializer) reset() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.view.Reset()
	m.index = 0
}

// updateView updates the view from a sequence of events and stores
// the corresponding Raft index.
func (m *materializer) updateView(events []*pbsubscribe.Event, index uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if err := m.view.Update(events); err != nil {
		return err
	}

	m.index = index
	m.notifyUpdateLocked(nil)
	m.retryWaiter.Reset()
	return nil
}

func (m *materializer) handleError(req *pbsubscribe.SubscribeRequest, err error) {
	failures := m.retryWaiter.Failures()
	if isNonTemporaryOrConsecutiveFailure(err, failures) {
		m.lock.Lock()
		m.notifyUpdateLocked(err)
		m.lock.Unlock()
	}

	logger := m.logger.With(
		"err", err,
		"topic", req.Topic,
		"failure_count", failures+1,
	)

	if req.GetWildcardSubject() {
		logger = logger.With("wildcard_subject", true)
	} else if sub := req.GetNamedSubject(); sub != nil {
		logger = logger.With("key", sub.Key)
	} else {
		logger = logger.With("key", req.Key) // nolint:staticcheck // SA1019 intentional use of deprecated field
	}

	logger.Error("subscribe call failed")
}

// isNonTemporaryOrConsecutiveFailure returns true if the error is not a
// temporary error or if failures > 0.
func isNonTemporaryOrConsecutiveFailure(err error, failures int) bool {
	// temporary is an interface used by net and other std lib packages to
	// show error types represent temporary/recoverable errors.
	temp, ok := err.(interface {
		Temporary() bool
	})
	return !ok || !temp.Temporary() || failures > 0
}

func defaultWaiter() *retry.Waiter {
	return &retry.Waiter{
		MinFailures: 1,
		// Start backing off with small increments (200-400ms) which will double
		// each attempt. (200-400, 400-800, 800-1600, 1600-3200, 3200-6000, 6000
		// after that). (retry.Wait applies Max limit after jitter right now).
		Factor:  200 * time.Millisecond,
		MinWait: 0,
		MaxWait: 60 * time.Second,
		Jitter:  retry.NewJitter(100),
	}
}
