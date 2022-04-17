package submatview

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/pbsubscribe"
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

// Materializer consumes the event stream, handling any framing events, and
// sends the events to View as they are received.
//
// Materializer is used as the cache.Result.State for a streaming
// cache type and manages the actual streaming RPC call to the servers behind
// the scenes until the cache result is discarded when TTL expires.
type Materializer struct {
	deps        Deps
	retryWaiter *retry.Waiter
	handler     eventHandler

	// lock protects the mutable state - all fields below it must only be accessed
	// while holding lock.
	lock     sync.Mutex
	index    uint64
	view     View
	updateCh chan struct{}
	err      error
}

type Deps struct {
	View    View
	Client  StreamClient
	Logger  hclog.Logger
	Waiter  *retry.Waiter
	Request func(index uint64) *pbsubscribe.SubscribeRequest
}

// StreamClient provides a subscription to state change events.
type StreamClient interface {
	Subscribe(ctx context.Context, in *pbsubscribe.SubscribeRequest, opts ...grpc.CallOption) (pbsubscribe.StateChangeSubscription_SubscribeClient, error)
}

// NewMaterializer returns a new Materializer. Run must be called to start it.
func NewMaterializer(deps Deps) *Materializer {
	v := &Materializer{
		deps:        deps,
		view:        deps.View,
		retryWaiter: deps.Waiter,
		updateCh:    make(chan struct{}),
	}
	if v.retryWaiter == nil {
		v.retryWaiter = &retry.Waiter{
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
	return v
}

// Run receives events from the StreamClient and sends them to the View. It runs
// until ctx is cancelled, so it is expected to be run in a goroutine.
func (m *Materializer) Run(ctx context.Context) {
	for {
		req := m.deps.Request(m.index)
		err := m.runSubscription(ctx, req)
		if ctx.Err() != nil {
			return
		}

		failures := m.retryWaiter.Failures()
		if isNonTemporaryOrConsecutiveFailure(err, failures) {
			m.lock.Lock()
			m.notifyUpdateLocked(err)
			m.lock.Unlock()
		}

		m.deps.Logger.Error("subscribe call failed",
			"err", err,
			"topic", req.Topic,
			"key", req.Key,
			"failure_count", failures+1)

		if err := m.retryWaiter.Wait(ctx); err != nil {
			return
		}
	}
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

// runSubscription opens a new subscribe streaming call to the servers and runs
// for it's lifetime or until the view is closed.
func (m *Materializer) runSubscription(ctx context.Context, req *pbsubscribe.SubscribeRequest) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.handler = initialHandler(req.Index)

	s, err := m.deps.Client.Subscribe(ctx, req)
	if err != nil {
		return err
	}

	for {
		event, err := s.Recv()
		switch {
		case isGrpcStatus(err, codes.Aborted):
			m.reset()
			return resetErr("stream reset requested")
		case err != nil:
			return err
		}

		m.handler, err = m.handler(m, event)
		if err != nil {
			m.reset()
			return err
		}
	}
}

func isGrpcStatus(err error, code codes.Code) bool {
	s, ok := status.FromError(err)
	return ok && s.Code() == code
}

// resetErr represents a server request to reset the subscription, it's typed so
// we can mark it as temporary and so attempt to retry first time without
// notifying clients.
type resetErr string

// Temporary Implements the internal Temporary interface
func (e resetErr) Temporary() bool {
	return true
}

// Error implements error
func (e resetErr) Error() string {
	return string(e)
}

// reset clears the state ready to start a new stream from scratch.
func (m *Materializer) reset() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.view.Reset()
	m.index = 0
}

func (m *Materializer) updateView(events []*pbsubscribe.Event, index uint64) error {
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

// notifyUpdateLocked closes the current update channel and recreates a new
// one. It must be called while holding the s.lock lock.
func (m *Materializer) notifyUpdateLocked(err error) {
	m.err = err
	close(m.updateCh)
	m.updateCh = make(chan struct{})
}

// Result returned from the View.
type Result struct {
	Index uint64
	Value interface{}
	// Cached is true if the requested value was already available locally. If
	// the value is false, it indicates that getFromView had to wait for an update,
	Cached bool
}

// getFromView blocks until the index of the View is greater than opts.MinIndex,
//or the context is cancelled.
func (m *Materializer) getFromView(ctx context.Context, minIndex uint64) (Result, error) {
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
