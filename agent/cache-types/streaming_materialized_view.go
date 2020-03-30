package cachetype

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
)

const (
	// SubscribeBackoffMax controls the range of exponential backoff when errors
	// are returned from subscriptions.
	SubscribeBackoffMax = 60 * time.Second
)

// StreamingClient is the interface we need from the gRPC client stub. Separate
// interface simplifies testing.
type StreamingClient interface {
	Subscribe(ctx context.Context, in *agentpb.SubscribeRequest, opts ...grpc.CallOption) (agentpb.Consul_SubscribeClient, error)
}

// MaterializedViewState is the interface used to manage they type-specific
// materialized view logic.
type MaterializedViewState interface {
	// InitFilter is called once when the view is constructed if the subscription
	// has a non-empty Filter argument. The implementor is expected to create a
	// *bexpr.Filter and store it locally so it can be used to filter events
	// and/or results. Ideally filtering should occur inside `Update` calls such
	// that we don't store objects in the view state that are just filtered when
	// the result is returned, however in some cases it might not be possible and
	// the type may choose to store the whole view and only apply filtering in the
	// Result method just before returning a result.
	InitFilter(expression string) error

	// Update is called when one or more events are received. The first call will
	// include _all_ events in the initial snapshot which may be an empty set.
	// Subsequent calls will contain one or more update events in the order they
	// are received.
	Update(events []*agentpb.Event) error

	// Result returns the type-specific cache result based on the state. When no
	// events have been delivered yet the result should be an empty value type
	// suitable to return to clients in case there is an empty result on the
	// servers. The index the materialized view represents is maintained
	// separately and passed in in case the return type needs an Index field
	// populating. This allows implementations to not worry about maintaining
	// indexes seen during Update.
	Result(index uint64) (interface{}, error)
}

// StreamingCacheType is the interface a cache-type needs to implement to make
// use of streaming as the transport for updates from the server.
type StreamingCacheType interface {
	NewMaterializedViewState() MaterializedViewState
	StreamingClient() StreamingClient
	Logger() hclog.Logger
}

// temporary is a private interface as used by net and other std lib packages to
// show error types represent temporary/recoverable errors.
type temporary interface {
	Temporary() bool
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

// MaterializedView is a partial view of the state on servers, maintained via
// streaming subscriptions. It is specialized for different cache types by
// providing a MaterializedViewState that encapsulates the logic to update the
// state and format it as the correct result type.
//
// The MaterializedView object becomes the cache.Result.State for a streaming
// cache type and manages the actual streaming RPC call to the servers behind
// the scenes until the cache result is discarded when TTL expires.
type MaterializedView struct {
	// Properties above the lock are immutable after the view is constructed in
	// MaterializedViewFromFetch and must not be modified.
	typ        StreamingCacheType
	client     StreamingClient
	logger     hclog.Logger
	req        agentpb.SubscribeRequest
	ctx        context.Context
	cancelFunc func()

	// l protects the mutable state - all fields below it must only be accessed
	// while holding l.
	l            sync.Mutex
	index        uint64
	state        MaterializedViewState
	snapshotDone bool
	updateCh     chan struct{}
	retryWaiter  *lib.RetryWaiter
	err          error
	fatalErr     error
}

// MaterializedViewFromFetch retrieves an existing view from the cache result
// state if one exists, otherwise creates a new one. Note that the returned view
// MUST have Close called eventually to avoid leaking resources. Typically this
// is done automatically if the view is returned in a cache.Result.State when
// the cache evicts the result. If the view is not returned in a result state
// though Close must be called some other way to avoid leaking the goroutine and
// memory.
func MaterializedViewFromFetch(t StreamingCacheType, opts cache.FetchOptions,
	subReq agentpb.SubscribeRequest) *MaterializedView {

	if opts.LastResult == nil || opts.LastResult.State == nil {
		ctx, cancel := context.WithCancel(context.Background())
		v := &MaterializedView{
			typ:        t,
			client:     t.StreamingClient(),
			logger:     t.Logger(),
			req:        subReq,
			ctx:        ctx,
			cancelFunc: cancel,
			// Allow first retry without wait, this is important and we rely on it in
			// tests.
			retryWaiter: lib.NewRetryWaiter(1, 0, SubscribeBackoffMax,
				lib.NewJitterRandomStagger(100)),
		}
		// Run init now otherwise there is a race between run() and a call to Fetch
		// which expects a view state to exist.
		v.reset()
		go v.run()
		return v
	}
	return opts.LastResult.State.(*MaterializedView)
}

// Close implements io.Close and discards view state and stops background view
// maintenance.
func (v *MaterializedView) Close() error {
	v.l.Lock()
	defer v.l.Unlock()
	if v.cancelFunc != nil {
		v.cancelFunc()
	}
	return nil
}

func (v *MaterializedView) run() {
	if v.ctx.Err() != nil {
		return
	}

	// Loop in case stream resets and we need to start over
	for {
		// Run a subscribe call until it fails
		err := v.runSubscription()
		if err != nil {
			// Check if the view closed
			if v.ctx.Err() != nil {
				// Err doesn't matter and is likely just context cancelled
				return
			}

			v.l.Lock()
			// If this is a temporary error and it's the first consecutive failure,
			// retry to see if we can get a result without erroring back to clients.
			// If it's non-temporary or a repeated failure return to clients while we
			// retry to get back in a good state.
			if _, ok := err.(temporary); !ok || v.retryWaiter.Failures() > 0 {
				// Report error to blocked fetchers
				v.err = err
				v.notifyUpdateLocked()
			}
			waitCh := v.retryWaiter.Failed()
			failures := v.retryWaiter.Failures()
			v.l.Unlock()

			// Exponential backoff to avoid hammering servers if they are closing
			// conns because of overload or resetting based on errors.
			v.logger.Error("subscribe call failed", "err", err, "topic", v.req.Topic,
				"key", v.req.Key, "failure_count", failures)

			select {
			case <-v.ctx.Done():
				return
			case <-waitCh:
			}
		}
		// Loop and keep trying to resume subscription after error
	}

}

// runSubscription opens a new subscribe streaming call to the servers and runs
// for it's lifetime or until the view is closed.
func (v *MaterializedView) runSubscription() error {
	ctx, cancel := context.WithCancel(v.ctx)
	defer cancel()

	// Copy the request template
	req := v.req

	v.l.Lock()

	// Update request index to be the current view index in case we are resuming a
	// broken stream.
	req.Index = v.index

	// Make local copy so we don't have to read with a lock for every event. We
	// are the only goroutine that can update so we know it won't change without
	// us knowing but we do need lock to protect external readers when we update.
	snapshotDone := v.snapshotDone

	v.l.Unlock()

	s, err := v.client.Subscribe(ctx, &req)
	if err != nil {
		return err
	}

	snapshotEvents := make([]*agentpb.Event, 0)

	for {
		event, err := s.Recv()
		if err != nil {
			return err
		}

		if event.GetResetStream() {
			// Server has requested we reset the view and start with a fresh snapshot
			// - perhaps because our ACL policy changed. We reset the view state and
			// then return an error to allow the `run` method to retry after a backoff
			// if necessary.
			v.reset()
			return resetErr("stream reset requested")
		}

		if event.GetEndOfSnapshot() {
			// Hold lock while mutating view state so implementer doesn't need to
			// worry about synchronization.
			v.l.Lock()

			// Deliver snapshot events to the View state
			if err := v.state.Update(snapshotEvents); err != nil {
				v.l.Unlock()
				// This error is kinda fatal to the view - we didn't apply some events
				// the server sent us which means our view is now not in sync. The only
				// thing we can do is start over and hope for a better outcome.
				v.reset()
				return err
			}
			// Done collecting these now
			snapshotEvents = nil
			v.snapshotDone = true
			// update our local copy so we can read it without lock.
			snapshotDone = true
			v.index = event.Index
			// We have a good result, reset the error flag
			v.err = nil
			v.retryWaiter.Reset()
			// Notify watchers of the update to the view
			v.notifyUpdateLocked()
			v.l.Unlock()
			continue
		}

		if event.GetResumeStream() {
			// We've opened a new subscribe with a non-zero index to resume a
			// connection and the server confirms it's not sending a new snapshot.
			if !snapshotDone {
				// We've somehow got into a bad state here - the server thinks we have
				// an up-to-date snapshot but we don't think we do. Reset and start
				// over.
				v.reset()
				return errors.New("stream resume sent but no local snapshot")
			}
			// Just continue on as we were!
			continue
		}

		// We have an event for the topic
		events := []*agentpb.Event{event}

		// If the event is a batch, unwrap and deliver the raw events
		if batch := event.GetEventBatch(); batch != nil {
			events = batch.Events
		}

		if snapshotDone {
			// We've already got a snapshot, this is an update, deliver it right away.
			v.l.Lock()
			if err := v.state.Update(events); err != nil {
				v.l.Unlock()
				// This error is kinda fatal to the view - we didn't apply some events
				// the server sent us which means our view is now not in sync. The only
				// thing we can do is start over and hope for a better outcome.
				v.reset()
				return err
			}
			// Notify watchers of the update to the view
			v.index = event.Index
			// We have a good result, reset the error flag
			v.err = nil
			v.retryWaiter.Reset()
			v.notifyUpdateLocked()
			v.l.Unlock()
		} else {
			snapshotEvents = append(snapshotEvents, events...)
		}
	}
}

// reset clears the state ready to start a new stream from scratch.
func (v *MaterializedView) reset() {
	v.l.Lock()
	defer v.l.Unlock()

	v.state = v.typ.NewMaterializedViewState()
	if v.req.Filter != "" {
		if err := v.state.InitFilter(v.req.Filter); err != nil {
			// If this errors we are stuck - it's fatal for the whole request as it
			// means there was a bug or an invalid filter string we couldn't parse.
			// Stop the whole view by closing it and cancelling context, but also set
			// the error internally so that Fetch calls can return a meaningful error
			// and not just "context cancelled".
			v.fatalErr = err
			if v.cancelFunc != nil {
				v.cancelFunc()
			}
			return
		}
	}
	v.notifyUpdateLocked()
	// Always start from zero when we have a new state so we load a snapshot from
	// the servers.
	v.index = 0
	v.snapshotDone = false
	v.err = nil
	v.retryWaiter.Reset()
}

// notifyUpdateLocked closes the current update channel and recreates a new
// one. It must be called while holding the s.l lock.
func (v *MaterializedView) notifyUpdateLocked() {
	if v.updateCh != nil {
		close(v.updateCh)
	}
	v.updateCh = make(chan struct{})
}

// Fetch implements the logic a StreamingCacheType will need during it's Fetch
// call. Cache types that use streaming should just be able to proxy to this
// once they have a subscription object and return it's results directly.
func (v *MaterializedView) Fetch(opts cache.FetchOptions) (cache.FetchResult, error) {
	var result cache.FetchResult

	// Get current view Result and index
	v.l.Lock()
	index := v.index
	val, err := v.state.Result(v.index)
	updateCh := v.updateCh
	v.l.Unlock()

	if err != nil {
		return result, err
	}

	result.Index = index
	result.Value = val
	result.State = v

	// If our index is > req.Index return right away. If index is zero then we
	// haven't loaded a snapshot at all yet which means we should wait for one on
	// the update chan. Note it's opts.MinIndex that the cache is using here the
	// request min index might be different and from initial user request.
	if index > 0 && index > opts.MinIndex {
		return result, nil
	}

	// Watch for timeout of the Fetch. Note it's opts.Timeout not req.Timeout
	// since that is the timeout the client requested from the cache Get while the
	// options one is the internal "background refresh" timeout which is what the
	// Fetch call should be using.
	timeoutCh := time.After(opts.Timeout)
	for {
		select {
		case <-updateCh:
			// View updated, return the new result
			v.l.Lock()
			result.Index = v.index
			// Grab the new updateCh in case we need to keep waiting for the next
			// update.
			updateCh = v.updateCh
			fetchErr := v.err
			if fetchErr == nil {
				// Only generate a new result if there was no error to avoid pointless
				// work potentially shuffling the same data around.
				result.Value, err = v.state.Result(v.index)
			}
			v.l.Unlock()

			// If there was a non-transient error return it
			if fetchErr != nil {
				return result, fetchErr
			}
			if err != nil {
				return result, err
			}

			// Sanity check the update is actually later than the one the user
			// requested.
			if result.Index <= opts.MinIndex {
				// The result is still older/same as the requested index, continue to
				// wait for further updates.
				continue
			}

			// Return the updated result
			return result, nil

		case <-timeoutCh:
			// Just return whatever we got originally, might still be empty
			return result, nil

		case <-v.ctx.Done():
			v.l.Lock()
			err := v.fatalErr
			v.l.Unlock()
			if err != nil {
				return result, err
			}
			return result, v.ctx.Err()
		}
	}
}
