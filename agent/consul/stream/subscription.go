package stream

import (
	context "context"
	"errors"
	"sync/atomic"
)

const (
	// SubscriptionStateOpen is the default state of a subscription
	SubscriptionStateOpen uint32 = 0

	// SubscriptionStateCloseReload signals that the subscription was closed by
	// server and client should retry.
	SubscriptionStateCloseReload uint32 = 1
)

var (
	// ErrSubscriptionReload is a error signalling reload event should be sent to
	// the client and the server should close.
	ErrSubscriptionReload = errors.New("subscription closed by server, client should retry")
)

// Subscription holds state about a single Subscribe call. Subscribe clients
// access their next event by calling Next(). This may initially include the
// snapshot events to catch them up if they are new or behind.
type Subscription struct {
	// state is accessed atomically 0 means open, 1 means closed with reload
	state uint32

	// req is the requests that we are responding to
	req *SubscribeRequest

	// currentItem stores the current snapshot or topic buffer item we are on. It
	// is mutated by calls to Next.
	currentItem *BufferItem

	// cancelFn stores the context cancel function that will wake up the
	// in-progress Next call on a server-initiated state change e.g. Reload.
	cancelFn atomic.Value
}

// NewSubscription return a new subscription.
func NewSubscription(req *SubscribeRequest, item *BufferItem) *Subscription {
	return &Subscription{
		req:         req,
		currentItem: item,
	}
}

// Next returns the next set of events to deliver. It must only be called from a
// single goroutine concurrently as it mutates the Subscription.
func (s *Subscription) Next(ctx context.Context) ([]Event, error) {
	state := atomic.LoadUint32(&s.state)
	if state == SubscriptionStateCloseReload {
		return nil, ErrSubscriptionReload
	}

	// Create our own sub-context which gets cancelled if the stream is reloaded
	// so we notice the state change while waiting for next event.
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// Unload the cancel function, we can't store nil so a no-op func is best. I
		// guess this probably isn't strictly necessary since it's safe to call
		// cancel func multiple times but it feels gross to keep that thing around
		// after the context is gone.
		s.cancelFn.Store(context.CancelFunc(func() {}))
		cancel()
	}()
	s.cancelFn.Store(cancel)

	events := make([]Event, 0, 1)
	for {
		next, err := s.currentItem.Next(ctx)
		if err != nil {
			// Check we didn't return because of a state change cancelling the context
			state := atomic.LoadUint32(&s.state)
			if state == SubscriptionStateCloseReload {
				return nil, ErrSubscriptionReload
			}
			return nil, err
		}
		// Advance our cursor for next loop or next call
		s.currentItem = next

		for _, e := range next.Events {
			// Only return it if the key matches - the buffer is for the whole topic.
			if s.req.Key == "" || s.req.Key == e.Key {
				events = append(events, e)
			}
		}
		if len(events) > 0 {
			return events, nil
		}
		// Keep looping until we find some events we are interested in.
	}
}

// CloseReload closes the stream and signals that the subscriber should reload.
// It is safe to call from any goroutine.
func (s *Subscription) CloseReload() {
	atomic.CompareAndSwapUint32(&s.state, SubscriptionStateOpen,
		SubscriptionStateCloseReload)

	fn := s.cancelFn.Load()
	if cancel, ok := fn.(context.CancelFunc); ok {
		cancel()
	}
}

// Request returns the request object that started the subscription.
func (s *Subscription) Request() *SubscribeRequest {
	return s.req
}
