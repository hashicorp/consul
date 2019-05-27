package fsm

import (
	"sync"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

type EventPublisher struct {
	listeners map[stream.Topic]map[*stream.SubscribeRequest]chan interface{}
	lock      sync.RWMutex
}

func NewEventPublisher() *EventPublisher {
	return &EventPublisher{
		listeners: make(map[stream.Topic]map[*stream.SubscribeRequest]chan interface{}),
	}
}

// PublishEvent sends a stream.Event based on the fsm operation to any
// listeners registered on the relevant topics.
func (e *EventPublisher) PublishEvent(req interface{}, index uint64) {
	event, topic := convertToEvent(req)

	e.lock.RLock()
	defer e.lock.RUnlock()
	for subscription, listener := range e.listeners[topic] {
		// If this event doesn't pertain to the subset this subscription is listening for,
		// skip sending it. We'll probably need more nuanced logic here later.
		if subscription.Key != event.Key && subscription.Key != "" {
			continue
		}

		select {
		case listener <- event:
		default:
		}
	}
}

// Convert the fsm request into the relevant protobuf structs and figure out
// which topic(s) it applies to.
func convertToEvent(req interface{}) (stream.Event, stream.Topic) {
	switch req.(type) {
	case structs.RegisterRequest:
	case structs.DeregisterRequest:
	}
}

func (e *EventPublisher) Subscribe(subscription *stream.SubscribeRequest) <-chan interface{} {
	ch := make(chan interface{}, 10)

	e.lock.Lock()
	defer e.lock.Unlock()
	if topicListeners, ok := e.listeners[subscription.Topic]; ok {
		topicListeners[subscription] = ch
	} else {
		e.listeners[subscription.Topic] = map[*stream.SubscribeRequest]chan interface{}{
			subscription: ch,
		}
	}

	return ch
}

func (e *EventPublisher) Unsubscribe(subscription *stream.SubscribeRequest) {
	e.lock.Lock()
	defer e.lock.Unlock()
	delete(e.listeners[subscription.Topic], subscription)
}
