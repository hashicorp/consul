package stream

import (
	"context"
	"errors"
	"sync/atomic"
)

const (
	// subscriptionStateOpen is the default state of a subscription. An open
	// subscription may receive new events.
	subscriptionStateOpen uint32 = 0

	// subscriptionStateClosed indicates that the subscription was closed, possibly
	// as a result of a change to an ACL token, and will not receive new events.
	// The subscriber must issue a new Subscribe request.
	subscriptionStateClosed uint32 = 1
)

// ErrSubscriptionClosed is a error signalling the subscription has been
// closed. The client should Unsubscribe, then re-Subscribe.
var ErrSubscriptionClosed = errors.New("subscription closed by server, client should unsub and retry")

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
	currentItem *bufferItem

	// ctx is the Subscription context that wraps the context of the streaming RPC
	// handler call.
	ctx context.Context

	// cancelFn stores the context cancel function that will wake up the
	// in-progress Next call on a server-initiated state change e.g. Reload.
	cancelFn func()

	// Unsubscribe is a function set by EventPublisher that is called to
	// free resources when the subscription is no longer needed.
	Unsubscribe func()
}

// SubscribeRequest identifies the types of events the subscriber would like to
// receiver. Topic and Token are required.
type SubscribeRequest struct {
	Topic Topic
	Key   string
	Token string
	Index uint64
}

// newSubscription return a new subscription. The caller is responsible for
// calling Unsubscribe when it is done with the subscription, to free resources.
func newSubscription(ctx context.Context, req *SubscribeRequest, item *bufferItem) *Subscription {
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
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return nil, ErrSubscriptionClosed
	}

	for {
		next, err := s.currentItem.Next(s.ctx)
		if err != nil {
			// Check we didn't return because of a state change cancelling the context
			if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
				return nil, ErrSubscriptionClosed
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

// Close the subscription. Subscribers will receive an error when they call Next,
// and will need to perform a new Subscribe request.
// It is safe to call from any goroutine.
func (s *Subscription) Close() {
	swapped := atomic.CompareAndSwapUint32(&s.state, subscriptionStateOpen, subscriptionStateClosed)
	if swapped {
		s.cancelFn()
	}
}
