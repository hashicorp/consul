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

	// ctx is the Subscription context that wraps the context of the streaming RPC
	// handler call.
	ctx context.Context

	// cancelFn stores the context cancel function that will wake up the
	// in-progress Next call on a server-initiated state change e.g. Reload.
	cancelFn func()
}

type SubscribeRequest struct {
	Topic Topic
	Key   string
	Token string
	Index uint64
}

// NewSubscription return a new subscription.
func NewSubscription(ctx context.Context, req *SubscribeRequest, item *BufferItem) *Subscription {
	subCtx, cancel := context.WithCancel(ctx)
	return &Subscription{
		ctx:         subCtx,
		cancelFn:    cancel,
		req:         req,
		currentItem: item,
	}
}

// Next returns the next set of events to deliver. It must only be called from a
// single goroutine concurrently as it mutates the Subscription.
func (s *Subscription) Next() ([]Event, error) {
	state := atomic.LoadUint32(&s.state)
	if state == SubscriptionStateCloseReload {
		return nil, ErrSubscriptionReload
	}

	for {
		next, err := s.currentItem.Next(s.ctx)
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

		// Assume happy path where all events (or none) are relevant.
		allMatch := true

		// If there is a specific key, see if we need to filter any events
		if s.req.Key != "" {
			for _, e := range next.Events {
				if s.req.Key != e.Key {
					allMatch = false
					break
				}
			}
		}

		// Only if we need to filter events should we bother allocating a new slice
		// as this is a hot loop.
		events := next.Events
		if !allMatch {
			events = make([]Event, 0, len(next.Events))
			for _, e := range next.Events {
				// Only return it if the key matches.
				if s.req.Key == "" || s.req.Key == e.Key {
					events = append(events, e)
				}
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
	swapped := atomic.CompareAndSwapUint32(&s.state, SubscriptionStateOpen,
		SubscriptionStateCloseReload)

	if swapped {
		s.cancelFn()
	}
}

// Request returns the request object that started the subscription.
func (s *Subscription) Request() *SubscribeRequest {
	return s.req
}
