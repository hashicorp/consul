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
var ErrSubscriptionClosed = errors.New("subscription closed by server, client should resubscribe")

// Subscription provides events on a Topic. Events may be filtered by Key.
// Events are returned by Next(), and may start with a Snapshot of events.
type Subscription struct {
	// state is accessed atomically 0 means open, 1 means closed with reload
	state uint32

	// req is the requests that we are responding to
	req *SubscribeRequest

	// currentItem stores the current snapshot or topic buffer item we are on. It
	// is mutated by calls to Next.
	currentItem *bufferItem

	// forceClosed is closed when forceClose is called. It is used by
	// EventPublisher to cancel Next().
	forceClosed chan struct{}

	// unsub is a function set by EventPublisher that is called to free resources
	// when the subscription is no longer needed.
	// It must be safe to call the function from multiple goroutines and the function
	// must be idempotent.
	unsub func()
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
func newSubscription(req *SubscribeRequest, item *bufferItem, unsub func()) *Subscription {
	return &Subscription{
		forceClosed: make(chan struct{}),
		req:         req,
		currentItem: item,
		unsub:       unsub,
	}
}

// Next returns the next set of events to deliver. It must only be called from a
// single goroutine concurrently as it mutates the Subscription.
func (s *Subscription) Next(ctx context.Context) ([]Event, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return nil, ErrSubscriptionClosed
	}

	for {
		next, err := s.currentItem.Next(ctx, s.forceClosed)
		switch {
		case err != nil && atomic.LoadUint32(&s.state) == subscriptionStateClosed:
			return nil, ErrSubscriptionClosed
		case err != nil:
			return nil, err
		}
		s.currentItem = next

		events := filter(s.req.Key, next.Events)
		if len(events) == 0 {
			continue
		}
		return events, nil
	}
}

// filter events to only those that match the key exactly.
func filter(key string, events []Event) []Event {
	if key == "" || len(events) == 0 {
		return events
	}

	var count int
	for _, e := range events {
		if key == e.Key {
			count++
		}
	}

	// Only allocate a new slice if some events need to be filtered out
	switch count {
	case 0:
		return nil
	case len(events):
		return events
	}

	result := make([]Event, 0, count)
	for _, e := range events {
		if key == e.Key {
			result = append(result, e)
		}
	}
	return result
}

// Close the subscription. Subscribers will receive an error when they call Next,
// and will need to perform a new Subscribe request.
// It is safe to call from any goroutine.
func (s *Subscription) forceClose() {
	swapped := atomic.CompareAndSwapUint32(&s.state, subscriptionStateOpen, subscriptionStateClosed)
	if swapped {
		close(s.forceClosed)
	}
}

// Unsubscribe the subscription, freeing resources.
func (s *Subscription) Unsubscribe() {
	s.unsub()
}
