// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// RPCMaterializer is a materializer for a streaming cache type
// and manages the actual streaming RPC call to the servers behind
// the scenes until the cache result is discarded when its TTL expires.
type RPCMaterializer struct {
	deps    Deps
	client  StreamClient
	handler eventHandler

	mat *materializer
}

var _ Materializer = (*RPCMaterializer)(nil)

// StreamClient provides a subscription to state change events.
type StreamClient interface {
	Subscribe(ctx context.Context, in *pbsubscribe.SubscribeRequest, opts ...grpc.CallOption) (pbsubscribe.StateChangeSubscription_SubscribeClient, error)
}

// NewRPCMaterializer returns a new Materializer. Run must be called to start it.
func NewRPCMaterializer(client StreamClient, deps Deps) *RPCMaterializer {
	m := RPCMaterializer{
		deps:   deps,
		client: client,
		mat:    newMaterializer(deps.Logger, deps.View, deps.Waiter),
	}
	return &m
}

// Query implements Materializer
func (m *RPCMaterializer) Query(ctx context.Context, minIndex uint64) (Result, error) {
	return m.mat.query(ctx, minIndex)
}

// Run receives events from the StreamClient and sends them to the View. It runs
// until ctx is cancelled, so it is expected to be run in a goroutine.
// Mirrors implementation of LocalMaterializer
//
// Run implements Materializer
func (m *RPCMaterializer) Run(ctx context.Context) {
	for {
		req := m.deps.Request(m.mat.currentIndex())
		err := m.subscribeOnce(ctx, req)
		if ctx.Err() != nil {
			return
		}

		var idle idleStreamErr
		if errors.As(err, &idle) {
			// A silent stream is not a failure: keep serving the current view and
			// resubscribe from its index, without waking callers with an error or
			// backing off. The idle timeout already paced this attempt.
			m.logIdle(req, idle)
			continue
		}
		m.mat.handleError(req, err)

		if err := m.mat.retryWaiter.Wait(ctx); err != nil {
			return
		}
	}
}

// subscribeOnce opens a new subscribe streaming call to the servers and runs
// for its lifetime or until the view is closed.
func (m *RPCMaterializer) subscribeOnce(ctx context.Context, req *pbsubscribe.SubscribeRequest) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The watchdog covers the whole stream, including a Subscribe call or a
	// snapshot that never arrives.
	watchdog := newIdleWatchdog(m.deps.IdleTimeout, cancel)
	defer watchdog.stop()

	m.handler = initialHandler(req.Index)

	s, err := m.client.Subscribe(ctx, req)
	if watchdog.fired() {
		return watchdog.err()
	}
	if err != nil {
		return err
	}

	for {
		event, err := s.Recv()
		if watchdog.fired() {
			return watchdog.err()
		}
		watchdog.reset()
		switch {
		case isGrpcStatus(err, codes.Aborted):
			m.mat.reset()
			return resetErr("stream reset requested")
		case err != nil:
			return err
		}

		m.handler, err = m.handler(m, event)
		if err != nil {
			m.mat.reset()
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

// updateView implements viewState
func (m *RPCMaterializer) updateView(events []*pbsubscribe.Event, index uint64) error {
	return m.mat.updateView(events, index)
}

// reset implements viewState
func (m *RPCMaterializer) reset() {
	m.mat.reset()
}

// idleStreamErr is returned when a subscription received nothing for longer
// than Deps.IdleTimeout.
type idleStreamErr struct {
	idle time.Duration
}

// Error implements error
func (e idleStreamErr) Error() string {
	return fmt.Sprintf("no events received on the subscription for %s", e.idle)
}

// idleWatchdog cancels a stream that stays silent. The zero value, returned
// when the timeout is not positive, never fires.
type idleWatchdog struct {
	timeout time.Duration
	timer   *time.Timer
	idle    atomic.Bool
}

// idleJitterFraction spreads the timeouts of subscriptions that were opened
// together, so a server does not have to serve all their resubscribes at once.
const idleJitterFraction = 0.1

func newIdleWatchdog(timeout time.Duration, cancel context.CancelFunc) *idleWatchdog {
	w := &idleWatchdog{}
	if timeout <= 0 {
		return w
	}
	w.timeout = timeout + time.Duration(rand.Float64()*idleJitterFraction*float64(timeout))
	w.timer = time.AfterFunc(w.timeout, func() {
		w.idle.Store(true)
		cancel()
	})
	return w
}

// reset restarts the countdown after a message was received.
func (w *idleWatchdog) reset() {
	if w.timer != nil {
		w.timer.Reset(w.timeout)
	}
}

func (w *idleWatchdog) stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}

func (w *idleWatchdog) fired() bool {
	return w.idle.Load()
}

func (w *idleWatchdog) err() error {
	return idleStreamErr{idle: w.timeout}
}

func (m *RPCMaterializer) logIdle(req *pbsubscribe.SubscribeRequest, err idleStreamErr) {
	logger := m.deps.Logger.With(
		"topic", req.Topic,
		"index", m.mat.currentIndex(),
		"idle_timeout", err.idle,
	)
	if sub := req.GetNamedSubject(); sub != nil {
		logger = logger.With("key", sub.Key)
	}
	logger.Warn("streaming subscription stalled; resubscribing")
	metrics.IncrCounter([]string{"streaming", "idle_resubscribe"}, 1)
}
