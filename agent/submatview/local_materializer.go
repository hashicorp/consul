package submatview

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// LocalMaterializer is a materializer for a stream of events
// and manages the local subscription to the event publisher
// until the cache result is discarded when its TTL expires.
type LocalMaterializer struct {
	deps        Deps
	backend     LocalBackend
	retryWaiter *retry.Waiter
	handler     eventHandler

	mat *materializer
}

var _ Materializer = (*LocalMaterializer)(nil)

type LocalBackend interface {
	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)
}

func NewLocalMaterializer(backend LocalBackend, deps Deps) *LocalMaterializer {
	m := LocalMaterializer{
		backend: backend,
		deps:    deps,
		mat:     newMaterializer(deps.Logger, deps.View, deps.Waiter),
	}
	return &m
}

// Query implements Materializer
func (m *LocalMaterializer) Query(ctx context.Context, minIndex uint64) (Result, error) {
	return m.mat.query(ctx, minIndex)
}

// Run receives events from a local subscription backend and sends them to the View.
// It runs until ctx is cancelled, so it is expected to be run in a goroutine.
// Mirrors implementation of RPCMaterializer.
//
// Run implements Materializer
func (m *LocalMaterializer) Run(ctx context.Context) {
	for {
		req := m.deps.Request(m.mat.currentIndex())
		err := m.subscribeOnce(ctx, req)
		if ctx.Err() != nil {
			return
		}
		m.mat.handleError(req, err)

		if err := m.mat.retryWaiter.Wait(ctx); err != nil {
			return
		}
	}
}

// subscribeOnce opens a new subscription to a local backend and runs
// for its lifetime or until the view is closed.
func (m *LocalMaterializer) subscribeOnce(ctx context.Context, req *pbsubscribe.SubscribeRequest) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.handler = initialHandler(req.Index)

	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)
	sub, err := m.backend.Subscribe(state.PBToStreamSubscribeRequest(req, entMeta))
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	for {
		event, err := sub.Next(ctx)
		switch {
		case errors.Is(err, stream.ErrSubForceClosed):
			m.deps.Logger.Trace("subscription reset by server")
			return err

		case err != nil:
			return err
		}

		e := event.Payload.ToSubscriptionEvent(event.Index)
		m.handler, err = m.handler(m, e)
		if err != nil {
			m.mat.reset()
			return err
		}
	}
}

// updateView implements viewState
func (m *LocalMaterializer) updateView(events []*pbsubscribe.Event, index uint64) error {
	return m.mat.updateView(events, index)
}

// reset implements viewState
func (m *LocalMaterializer) reset() {
	m.mat.reset()
}
