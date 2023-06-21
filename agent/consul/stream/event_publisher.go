// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventPublisher receives change events from Publish, and sends the events to
// all subscribers of the event Topic.
type EventPublisher struct {
	// snapCacheTTL controls how long we keep snapshots in our cache before
	// allowing them to be garbage collected and a new one made for subsequent
	// requests for that topic and key. In general this should be pretty short to
	// keep memory overhead of duplicated event data low - snapshots are typically
	// not that expensive, but having a cache for a few seconds can help
	// de-duplicate building the same snapshot over and over again when a
	// thundering herd of watchers all subscribe to the same topic within a few
	// seconds.
	snapCacheTTL time.Duration

	// This lock protects the snapCache, topicBuffers and topicBuffer.refs.
	lock sync.RWMutex

	// topicBuffers stores the head of the linked-list buffers to publish events to
	// for a topic.
	topicBuffers map[topicSubject]*topicBuffer

	// snapCache if a cache of EventSnapshots indexed by topic and subject.
	// TODO(streaming): new snapshotCache struct for snapCache and snapCacheTTL
	snapCache map[topicSubject]*eventSnapshot

	subscriptions *subscriptions

	// publishCh is used to send messages from an active txn to a goroutine which
	// publishes events, so that publishing can happen asynchronously from
	// the Commit call in the FSM hot path.
	publishCh chan []Event

	snapshotHandlers SnapshotHandlers

	// wildcards contains map keys used to access the buffer for a topic's wildcard
	// subject — it is used to track which topics support wildcard subscriptions.
	wildcards map[Topic]topicSubject
}

// topicSubject is used as a map key when accessing topic buffers and cached
// snapshots.
type topicSubject struct {
	Topic   string
	Subject string
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

// topicBuffer augments the eventBuffer with a reference counter, enabling
// clean up of unused buffers once there are no longer any subscribers for
// the given topic and key.
type topicBuffer struct {
	refs int // refs is guarded by EventPublisher.lock.
	buf  *eventBuffer
}

// SnapshotHandlers is a mapping of Topic to a function which produces a snapshot
// of events for the SubscribeRequest. Events are appended to the snapshot using SnapshotAppender.
// The nil Topic is reserved and should not be used.
type SnapshotHandlers map[Topic]SnapshotFunc

// SnapshotFunc builds a snapshot for the subscription request, and appends the
// events to the Snapshot using SnapshotAppender.
//
// Note: index MUST NOT be zero if any events were appended.
type SnapshotFunc func(SubscribeRequest, SnapshotAppender) (index uint64, err error)

// SnapshotAppender appends groups of events to create a Snapshot of state.
type SnapshotAppender interface {
	// Append events to the snapshot. Every event in the slice must have the same
	// Index, indicating that it is part of the same raft transaction.
	Append(events []Event)
}

// NewEventPublisher returns an EventPublisher for publishing change events.
// Handlers are used to convert the memDB changes into events.
// A goroutine is run in the background to publish events to all subscribes.
// Cancelling the context will shutdown the goroutine, to free resources,
// and stop all publishing.
func NewEventPublisher(snapCacheTTL time.Duration) *EventPublisher {
	e := &EventPublisher{
		snapCacheTTL: snapCacheTTL,
		topicBuffers: make(map[topicSubject]*topicBuffer),
		snapCache:    make(map[topicSubject]*eventSnapshot),
		publishCh:    make(chan []Event, 64),
		subscriptions: &subscriptions{
			byToken: make(map[string]map[*SubscribeRequest]*Subscription),
		},
		snapshotHandlers: make(map[Topic]SnapshotFunc),
		wildcards:        make(map[Topic]topicSubject),
	}

	return e
}

// RegisterHandler will register a new snapshot handler function. The expectation is
// that all handlers get registered prior to the event publisher being Run. Handler
// registration is therefore not concurrency safe and access to handlers is internally
// not synchronized. Passing supportsWildcard allows consumers to subscribe to events
// on this topic with *any* subject (by requesting SubjectWildcard) but this must be
// supported by the handler function.
func (e *EventPublisher) RegisterHandler(topic Topic, handler SnapshotFunc, supportsWildcard bool) error {
	if topic.String() == "" {
		return fmt.Errorf("the topic cannnot be empty")
	}

	if _, found := e.snapshotHandlers[topic]; found {
		return fmt.Errorf("a handler is already registered for the topic: %s", topic.String())
	}

	e.snapshotHandlers[topic] = handler

	if supportsWildcard {
		e.wildcards[topic] = topicSubject{
			Topic:   topic.String(),
			Subject: SubjectWildcard.String(),
		}
	}

	return nil
}

func (e *EventPublisher) RefreshTopic(topic Topic) error {
	if _, found := e.snapshotHandlers[topic]; !found {
		return fmt.Errorf("topic %s is not registered", topic)
	}

	e.forceEvictByTopic(topic)
	e.subscriptions.closeAllByTopic(topic)

	return nil
}

// Publish events to all subscribers of the event Topic. The events will be shared
// with all subscriptions, so the Payload used in Event.Payload must be immutable.
func (e *EventPublisher) Publish(events []Event) {
	if len(events) == 0 {
		return
	}

	for idx, event := range events {
		if _, ok := event.Payload.(closeSubscriptionPayload); ok {
			continue
		}

		if event.Payload.Subject() == SubjectWildcard {
			panic(fmt.Sprintf("SubjectWildcard can only be used for subscription, not for publishing (topic: %s, index: %d)", event.Topic, idx))
		}
	}

	e.publishCh <- events
}

// Run the event publisher until ctx is cancelled. Run should be called from a
// goroutine to forward events from Publish to all the appropriate subscribers.
func (e *EventPublisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			e.subscriptions.closeAll()
			return
		case update := <-e.publishCh:
			e.publishEvent(update)
		}
	}
}

// publishEvent appends the events to any applicable topic buffers. It handles
// any closeSubscriptionPayload events by closing associated subscriptions.
func (e *EventPublisher) publishEvent(events []Event) {
	groupedEvents := make(map[topicSubject][]Event)
	for _, event := range events {
		if unsubEvent, ok := event.Payload.(closeSubscriptionPayload); ok {
			e.subscriptions.closeSubscriptionsForTokens(unsubEvent.tokensSecretIDs)
			continue
		}

		groupKey := topicSubject{
			Topic:   event.Topic.String(),
			Subject: event.Payload.Subject().String(),
		}
		groupedEvents[groupKey] = append(groupedEvents[groupKey], event)

		// If the topic supports wildcard subscribers, copy the events to a wildcard
		// buffer too.
		e.lock.Lock()
		wildcard, ok := e.wildcards[event.Topic]
		e.lock.Unlock()
		if ok {
			groupedEvents[wildcard] = append(groupedEvents[wildcard], event)
		}
	}

	e.lock.Lock()
	defer e.lock.Unlock()
	for groupKey, events := range groupedEvents {
		// Note: bufferForPublishing returns nil if there are no subscribers for the
		// given topic and subject, in which case events will be dropped on the floor and
		// future subscribers will catch up by consuming the snapshot.
		if buf := e.bufferForPublishing(groupKey); buf != nil {
			buf.Append(events)
		}
	}
}

// bufferForSubscription returns the topic event buffer to which events for the
// given topic and key will be appended. If no such buffer exists, a new buffer
// will be created.
//
// Warning: e.lock MUST be held when calling this function.
func (e *EventPublisher) bufferForSubscription(key topicSubject) *topicBuffer {
	buf, ok := e.topicBuffers[key]
	if !ok {
		buf = &topicBuffer{
			buf: newEventBuffer(),
		}
		e.topicBuffers[key] = buf
	}

	return buf
}

// bufferForPublishing returns the event buffer to which events for the given
// topic and key should be appended. nil will be returned if there are no
// subscribers for the given topic and key.
//
// Warning: e.lock MUST be held when calling this function.
func (e *EventPublisher) bufferForPublishing(key topicSubject) *eventBuffer {
	buf, ok := e.topicBuffers[key]
	if !ok {
		return nil
	}
	return buf.buf
}

// Subscribe returns a new Subscription for the given request. A subscription
// will receive an initial snapshot of events matching the request if req.Index > 0.
// After the snapshot, events will be streamed as they are created.
// Subscriptions may be closed, forcing the client to resubscribe (for example if
// ACL policies changed or the state store is abandoned).
//
// When the caller is finished with the subscription for any reason, it must
// call Subscription.Unsubscribe to free ACL tracking resources.
func (e *EventPublisher) Subscribe(req *SubscribeRequest) (*Subscription, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	handler, ok := e.snapshotHandlers[req.Topic]
	if !ok || req.Topic == nil {
		return nil, fmt.Errorf("unknown topic %v", req.Topic)
	}

	if req.Subject == SubjectWildcard {
		if _, supportsWildcard := e.wildcards[req.Topic]; !supportsWildcard {
			return nil, fmt.Errorf("topic %s does not support wildcard subscriptions", req.Topic)
		}
	}

	topicBuf := e.bufferForSubscription(req.topicSubject())
	topicBuf.refs++

	// freeBuf is used to free the topic buffer once there are no remaining
	// subscribers for the given topic and key.
	//
	// Note: it's called by Subcription.Unsubscribe which has its own side-effects
	// that are made without holding e.lock (so there's a moment where the ref
	// counter is inconsistent with the subscription map) — in practice this is
	// fine, we don't need these things to be strongly consistent. The alternative
	// would be to hold both locks, which introduces the risk of deadlocks.
	freeBuf := func() {
		e.lock.Lock()
		defer e.lock.Unlock()

		topicBuf.refs--

		if topicBuf.refs == 0 {
			delete(e.topicBuffers, req.topicSubject())

			// Evict cached snapshot too because the topic buffer will have been spliced
			// onto it. If we don't do this, any new subscribers started before the cache
			// TTL is reached will get "stuck" waiting on the old buffer.
			delete(e.snapCache, req.topicSubject())
		}
	}

	topicHead := topicBuf.buf.Head()

	// If the client view is fresh, resume the stream.
	if req.Index > 0 && topicHead.HasEventIndex(req.Index) {
		buf := newEventBuffer()
		subscriptionHead := buf.Head()
		// splice the rest of the topic buffer onto the subscription buffer so
		// the subscription will receive new events.
		next, _ := topicHead.NextNoBlock()
		buf.AppendItem(next)
		return e.subscriptions.add(req, subscriptionHead, freeBuf), nil
	}

	snapFromCache := e.getCachedSnapshotLocked(req)
	if snapFromCache == nil {
		snap := newEventSnapshot()
		snap.appendAndSplice(*req, handler, topicHead)
		e.setCachedSnapshotLocked(req, snap)
		snapFromCache = snap
	}

	// If the request.Index is 0 the client has no view, send a full snapshot.
	if req.Index == 0 {
		return e.subscriptions.add(req, snapFromCache.First, freeBuf), nil
	}

	// otherwise the request has an Index, the client view is stale and must be reset
	// with a NewSnapshotToFollow event.
	result := newEventSnapshot()
	result.buffer.Append([]Event{{
		Topic:   req.Topic,
		Payload: newSnapshotToFollow{},
	}})
	result.buffer.AppendItem(snapFromCache.First)
	return e.subscriptions.add(req, result.First, freeBuf), nil
}

func (s *subscriptions) add(req *SubscribeRequest, head *bufferItem, freeBuf func()) *Subscription {
	// We wrap freeBuf in a sync.Once as it's expected that Subscription.unsub is
	// idempotent, but freeBuf decrements the reference counter on every call.
	var once sync.Once
	sub := newSubscription(*req, head, func() {
		s.unsubscribe(req)
		once.Do(freeBuf)
	})

	s.lock.Lock()
	defer s.lock.Unlock()

	subsByToken, ok := s.byToken[req.Token]
	if !ok {
		subsByToken = make(map[*SubscribeRequest]*Subscription)
		s.byToken[req.Token] = subsByToken
	}
	subsByToken[req] = sub
	return sub
}

func (s *subscriptions) closeSubscriptionsForTokens(tokenSecretIDs []string) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, secretID := range tokenSecretIDs {
		if subs, ok := s.byToken[secretID]; ok {
			for _, sub := range subs {
				sub.forceClose()
			}
		}
	}
}

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

func (s *subscriptions) closeAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, byRequest := range s.byToken {
		for _, sub := range byRequest {
			sub.shutDown()
		}
	}
}

func (s *subscriptions) closeAllByTopic(topic Topic) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, byRequest := range s.byToken {
		for _, sub := range byRequest {
			if sub.req.Topic == topic {
				sub.forceClose()
			}
		}
	}
}

// EventPublisher.lock must be held to call this method.
func (e *EventPublisher) getCachedSnapshotLocked(req *SubscribeRequest) *eventSnapshot {
	snap, ok := e.snapCache[req.topicSubject()]
	if ok && snap.err() == nil {
		return snap
	}
	return nil
}

// EventPublisher.lock must be held to call this method.
func (e *EventPublisher) setCachedSnapshotLocked(req *SubscribeRequest, snap *eventSnapshot) {
	if e.snapCacheTTL == 0 {
		return
	}
	e.snapCache[req.topicSubject()] = snap

	// Setup a cache eviction
	time.AfterFunc(e.snapCacheTTL, func() {
		e.lock.Lock()
		defer e.lock.Unlock()
		delete(e.snapCache, req.topicSubject())
	})
}

// forceEvictByTopic will remove all entries from the snapshot cache for a given topic.
// This method should be called while holding the publishers lock.
func (e *EventPublisher) forceEvictByTopic(topic Topic) {
	e.lock.Lock()
	for key := range e.snapCache {
		if key.Topic == topic.String() {
			delete(e.snapCache, key)
		}
	}
	e.lock.Unlock()
}
