package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/consul/state/db"
)

// EventPublisher receives changes events from Publish, and sends them to all
// registered subscribers.
type EventPublisher struct {
	// topicBufferSize controls how many trailing events we keep in memory for
	// each topic to avoid needing to snapshot again for re-connecting clients
	// that may have missed some events. It may be zero for no buffering (the most
	// recent event is always kept though). TODO
	topicBufferSize int

	// snapCacheTTL controls how long we keep snapshots in our cache before
	// allowing them to be garbage collected and a new one made for subsequent
	// requests for that topic and key. In general this should be pretty short to
	// keep memory overhead of duplicated event data low - snapshots are typically
	// not that expensive, but having a cache for a few seconds can help
	// de-duplicate building the same snapshot over and over again when a
	// thundering herd of watchers all subscribe to the same topic within a few
	// seconds.
	snapCacheTTL time.Duration

	// This lock protects the topicBuffers, and snapCache
	lock sync.RWMutex

	// topicBuffers stores the head of the linked-list buffer to publish events to
	// for a topic.
	topicBuffers map[Topic]*EventBuffer

	// snapCache if a cache of EventSnapshots indexed by topic and key.
	// TODO: new struct for snapCache and snapFns and snapCacheTTL
	snapCache map[Topic]map[string]*EventSnapshot

	subscriptions *subscriptions

	// publishCh is used to send messages from an active txn to a goroutine which
	// publishes events, so that publishing can happen asynchronously from
	// the Commit call in the FSM hot path.
	publishCh chan changeEvents

	handlers map[Topic]TopicHandler
}

type subscriptions struct {
	// lock for byToken. If both subscription.lock and EventPublisher.lock need
	// to be held, EventPublisher.lock MUST always be acquired first.
	lock sync.RWMutex

	// byToken is an mapping of active Subscriptions indexed by a the token and
	// a pointer to the request.
	// When the token is modified all subscriptions under that token will be
	// reloaded.
	// A subscription may be unsubscribed by using the pointer to the request.
	byToken map[string]map[*SubscribeRequest]*Subscription
}

type changeEvents struct {
	events []Event
}

// TopicHandler provides functions which create stream.Events for a topic.
type TopicHandler struct {
	// Snapshot creates the necessary events to reproduce the current state and
	// appends them to the EventBuffer.
	Snapshot func(*SubscribeRequest, *EventBuffer) (index uint64, err error)
	// ProcessChanges accepts a slice of Changes, and builds a slice of events for
	// those changes.
	ProcessChanges func(db.ReadTxn, db.Changes) ([]Event, error)
}

// NewEventPublisher returns an EventPublisher for publishing change events.
// Handlers are used to convert the memDB changes into events.
// A goroutine is run in the background to publish events to all subscribes.
// Cancelling the context will shutdown the goroutine, to free resources,
// and stop all publishing.
func NewEventPublisher(ctx context.Context, handlers map[Topic]TopicHandler, snapCacheTTL time.Duration) *EventPublisher {
	e := &EventPublisher{
		snapCacheTTL: snapCacheTTL,
		topicBuffers: make(map[Topic]*EventBuffer),
		snapCache:    make(map[Topic]map[string]*EventSnapshot),
		publishCh:    make(chan changeEvents, 64),
		subscriptions: &subscriptions{
			byToken: make(map[string]map[*SubscribeRequest]*Subscription),
		},
		handlers: handlers,
	}

	go e.handleUpdates(ctx)

	return e
}

// PublishChanges to all subscribers. tx is a read-only transaction that captures
// the state at the time the change happened. The caller must never use the tx once
// it has been passed to PublishChanged.
func (e *EventPublisher) PublishChanges(tx db.ReadTxn, changes db.Changes) error {
	defer tx.Abort()

	var events []Event
	for topic, handler := range e.handlers {
		if handler.ProcessChanges != nil {
			es, err := handler.ProcessChanges(tx, changes)
			if err != nil {
				return fmt.Errorf("failed generating events for topic %q: %s", topic, err)
			}
			events = append(events, es...)
		}
	}

	e.publishCh <- changeEvents{events: events}
	return nil
}

func (e *EventPublisher) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// TODO: also close all subscriptions so the subscribers are moved
			// to the new publisher?
			return
		case update := <-e.publishCh:
			e.sendEvents(update)
		}
	}
}

// sendEvents sends the given events to any applicable topic listeners, as well
// as any ACL update events to cause affected listeners to reset their stream.
func (e *EventPublisher) sendEvents(update changeEvents) {
	for _, event := range update.events {
		if unsubEvent, ok := event.Payload.(UnsubscribePayload); ok {
			e.subscriptions.closeSubscriptionsForTokens(unsubEvent.TokensSecretIDs)
		}
	}

	eventsByTopic := make(map[Topic][]Event)
	for _, event := range update.events {
		if event.Topic == TopicInternal {
			continue
		}
		eventsByTopic[event.Topic] = append(eventsByTopic[event.Topic], event)
	}

	e.lock.Lock()
	defer e.lock.Unlock()
	for topic, events := range eventsByTopic {
		e.getTopicBuffer(topic).Append(events)
	}
}

// getTopicBuffer for the topic. Creates a new event buffer if one does not
// already exist.
//
// EventPublisher.lock must be held to call this method.
func (e *EventPublisher) getTopicBuffer(topic Topic) *EventBuffer {
	buf, ok := e.topicBuffers[topic]
	if !ok {
		buf = NewEventBuffer()
		e.topicBuffers[topic] = buf
	}
	return buf
}

// Subscribe returns a new stream.Subscription for the given request. A
// subscription will stream an initial snapshot of events matching the request
// if required and then block until new events that modify the request occur, or
// the context is cancelled. Subscriptions may be forced to reset if the server
// decides it can no longer maintain correct operation for example if ACL
// policies changed or the state store was restored.
//
// When the caller is finished with the subscription for any reason, it must
// call Subscription.Unsubscribe to free ACL tracking resources.
func (e *EventPublisher) Subscribe(
	ctx context.Context,
	req *SubscribeRequest,
) (*Subscription, error) {
	// Ensure we know how to make a snapshot for this topic
	_, ok := e.handlers[req.Topic]
	if !ok || req.Topic == TopicInternal {
		return nil, fmt.Errorf("unknown topic %d", req.Topic)
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	// Ensure there is a topic buffer for that topic so we start capturing any
	// future published events.
	buf := e.getTopicBuffer(req.Topic)

	// See if we need a snapshot
	topicHead := buf.Head()
	var sub *Subscription
	if req.Index > 0 && len(topicHead.Events) > 0 && topicHead.Events[0].Index == req.Index {
		// No need for a snapshot, send the "resume stream" message to signal to
		// client it's cache is still good. (note that this can be distinguished
		// from a legitimate empty snapshot due to the index matching the one the
		// client sent), then follow along from here in the topic.
		e := Event{
			Index:   req.Index,
			Topic:   req.Topic,
			Key:     req.Key,
			Payload: ResumeStream{},
		}
		// Make a new buffer to send to the client containing the resume.
		buf := NewEventBuffer()

		// Store the head of that buffer before we append to it to give as the
		// starting point for the subscription.
		subHead := buf.Head()

		buf.Append([]Event{e})

		// Now splice the rest of the topic buffer on so the subscription will
		// continue to see future updates in the topic buffer.
		follow, err := topicHead.FollowAfter()
		if err != nil {
			return nil, err
		}
		buf.AppendBuffer(follow)

		sub = NewSubscription(ctx, req, subHead)
	} else {
		snap, err := e.getSnapshotLocked(req, topicHead)
		if err != nil {
			return nil, err
		}
		sub = NewSubscription(ctx, req, snap.Snap)
	}

	e.subscriptions.add(req, sub)
	// Set unsubscribe so that the caller doesn't need to keep track of the
	// SubscriptionRequest, and can not accidentally call unsubscribe with the
	// wrong value.
	sub.Unsubscribe = func() {
		e.subscriptions.unsubscribe(req)
	}
	return sub, nil
}

func (s *subscriptions) add(req *SubscribeRequest, sub *Subscription) {
	s.lock.Lock()
	defer s.lock.Unlock()

	subsByToken, ok := s.byToken[req.Token]
	if !ok {
		subsByToken = make(map[*SubscribeRequest]*Subscription)
		s.byToken[req.Token] = subsByToken
	}
	subsByToken[req] = sub
}

func (s *subscriptions) closeSubscriptionsForTokens(tokenSecretIDs []string) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, secretID := range tokenSecretIDs {
		if subs, ok := s.byToken[secretID]; ok {
			for _, sub := range subs {
				sub.Close()
			}
		}
	}
}

// unsubscribe must be called when a client is no longer interested in a
// subscription to free resources monitoring changes in it's ACL token.
//
// req MUST be the same pointer that was used to register the subscription.
func (s *subscriptions) unsubscribe(req *SubscribeRequest) {
	s.lock.Lock()
	defer s.lock.Unlock()

	subsByToken, ok := s.byToken[req.Token]
	if !ok {
		return
	}
	delete(subsByToken, req)
	if len(subsByToken) == 0 {
		delete(s.byToken, req.Token)
	}
}

func (e *EventPublisher) getSnapshotLocked(req *SubscribeRequest, topicHead *BufferItem) (*EventSnapshot, error) {
	// See if there is a cached snapshot
	topicSnaps, ok := e.snapCache[req.Topic]
	if !ok {
		topicSnaps = make(map[string]*EventSnapshot)
		e.snapCache[req.Topic] = topicSnaps
	}

	snap, ok := topicSnaps[req.Key]
	if ok && snap.Err() == nil {
		return snap, nil
	}

	// No snap or errored snap in cache, create a new one
	handler, ok := e.handlers[req.Topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic %d", req.Topic)
	}

	snap = NewEventSnapshot(req, topicHead, handler.Snapshot)
	if e.snapCacheTTL > 0 {
		topicSnaps[req.Key] = snap

		// Trigger a clearout after TTL
		time.AfterFunc(e.snapCacheTTL, func() {
			e.lock.Lock()
			defer e.lock.Unlock()
			delete(topicSnaps, req.Key)
		})
	}

	return snap, nil
}
