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
var ErrSubscriptionClosed = errors.New("subscription closed by server, client must reset state and resubscribe")

// Subscription provides events on a Topic. Events may be filtered by Key.
// Events are returned by Next(), and may start with a Snapshot of events.
type Subscription struct {
	// state is accessed atomically 0 means open, 1 means closed with reload
	state uint32

	// req is the requests that we are responding to
	req SubscribeRequest

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
func newSubscription(req SubscribeRequest, item *bufferItem, unsub func()) *Subscription {
	return &Subscription{
		forceClosed: make(chan struct{}),
		req:         req,
		currentItem: item,
		unsub:       unsub,
	}
}

// Next returns the next Event to deliver. It must only be called from a
// single goroutine concurrently as it mutates the Subscription.
func (s *Subscription) Next(ctx context.Context) (Event, error) {
	if atomic.LoadUint32(&s.state) == subscriptionStateClosed {
		return Event{}, ErrSubscriptionClosed
	}

	for {
		next, err := s.currentItem.Next(ctx, s.forceClosed)
		switch {
		case err != nil && atomic.LoadUint32(&s.state) == subscriptionStateClosed:
			return Event{}, ErrSubscriptionClosed
		case err != nil:
			return Event{}, err
		}
		s.currentItem = next
		if len(next.Events) == 0 {
			continue
		}
		event, ok := filterByKey(s.req, next.Events)
		if !ok {
			continue
		}
		return event, nil
	}
}

func newEventFromBatch(req SubscribeRequest, events []Event) Event {
	first := events[0]
	if len(events) == 1 {
		return first
	}
	return Event{
		Topic:   req.Topic,
		Key:     req.Key,
		Index:   first.Index,
		Payload: events,
	}
}

func filterByKey(req SubscribeRequest, events []Event) (Event, bool) {
	event := newEventFromBatch(req, events)
	if req.Key == "" {
		return event, true
	}

	fn := func(e Event) bool {
		return req.Key == e.Key
	}
	return event.Filter(fn)
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
