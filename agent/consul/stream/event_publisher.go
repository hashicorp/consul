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

	// This lock protects the topicBuffers, and snapCache
	lock sync.RWMutex

	// topicBuffers stores the head of the linked-list buffer to publish events to
	// for a topic.
	topicBuffers map[Topic]*eventBuffer

	// snapCache if a cache of EventSnapshots indexed by topic and key.
	// TODO(streaming): new snapshotCache struct for snapCache and snapCacheTTL
	snapCache map[Topic]map[string]*eventSnapshot

	subscriptions *subscriptions

	// publishCh is used to send messages from an active txn to a goroutine which
	// publishes events, so that publishing can happen asynchronously from
	// the Commit call in the FSM hot path.
	publishCh chan []Event

	snapshotHandlers SnapshotHandlers
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

// SnapshotHandlers is a mapping of Topic to a function which produces a snapshot
// of events for the SubscribeRequest. Events are appended to the snapshot using SnapshotAppender.
// The nil Topic is reserved and should not be used.
type SnapshotHandlers map[Topic]SnapshotFunc

// SnapshotFunc builds a snapshot for the subscription request, and appends the
// events to the Snapshot using SnapshotAppender.
// If err is not nil the SnapshotFunc must return a non-zero index.
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
func NewEventPublisher(handlers SnapshotHandlers, snapCacheTTL time.Duration) *EventPublisher {
	e := &EventPublisher{
		snapCacheTTL: snapCacheTTL,
		topicBuffers: make(map[Topic]*eventBuffer),
		snapCache:    make(map[Topic]map[string]*eventSnapshot),
		publishCh:    make(chan []Event, 64),
		subscriptions: &subscriptions{
			byToken: make(map[string]map[*SubscribeRequest]*Subscription),
		},
		snapshotHandlers: handlers,
	}

	return e
}

// Publish events to all subscribers of the event Topic. The events will be shared
// with all subscriptions, so the Payload used in Event.Payload must be immutable.
func (e *EventPublisher) Publish(events []Event) {
	if len(events) > 0 {
		e.publishCh <- events
	}
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
	eventsByTopic := make(map[Topic][]Event)
	for _, event := range events {
		if unsubEvent, ok := event.Payload.(closeSubscriptionPayload); ok {
			e.subscriptions.closeSubscriptionsForTokens(unsubEvent.tokensSecretIDs)
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
func (e *EventPublisher) getTopicBuffer(topic Topic) *eventBuffer {
	buf, ok := e.topicBuffers[topic]
	if !ok {
		buf = newEventBuffer()
		e.topicBuffers[topic] = buf
	}
	return buf
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
	handler, ok := e.snapshotHandlers[req.Topic]
	if !ok || req.Topic == nil {
		return nil, fmt.Errorf("unknown topic %v", req.Topic)
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	topicHead := e.getTopicBuffer(req.Topic).Head()

	// If the client view is fresh, resume the stream.
	if req.Index > 0 && topicHead.HasEventIndex(req.Index) {
		buf := newEventBuffer()
		subscriptionHead := buf.Head()
		// splice the rest of the topic buffer onto the subscription buffer so
		// the subscription will receive new events.
		next, _ := topicHead.NextNoBlock()
		buf.AppendItem(next)
		return e.subscriptions.add(req, subscriptionHead), nil
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
		return e.subscriptions.add(req, snapFromCache.First), nil
	}

	// otherwise the request has an Index, the client view is stale and must be reset
	// with a NewSnapshotToFollow event.
	result := newEventSnapshot()
	result.buffer.Append([]Event{{
		Topic:   req.Topic,
		Payload: newSnapshotToFollow{},
	}})
	result.buffer.AppendItem(snapFromCache.First)
	return e.subscriptions.add(req, result.First), nil
}

func (s *subscriptions) add(req *SubscribeRequest, head *bufferItem) *Subscription {
	sub := newSubscription(*req, head, s.unsubscribe(req))

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

// unsubscribe returns a function that the subscription will call to remove
// itself from the subsByToken.
// This function is returned as a closure so that the caller doesn't need to keep
// track of the SubscriptionRequest, and can not accidentally call unsubscribe with the
// wrong pointer.
func (s *subscriptions) unsubscribe(req *SubscribeRequest) func() {
	return func() {
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
}

func (s *subscriptions) closeAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, byRequest := range s.byToken {
		for _, sub := range byRequest {
			sub.forceClose()
		}
	}
}

// EventPublisher.lock must be held to call this method.
func (e *EventPublisher) getCachedSnapshotLocked(req *SubscribeRequest) *eventSnapshot {
	topicSnaps, ok := e.snapCache[req.Topic]
	if !ok {
		topicSnaps = make(map[string]*eventSnapshot)
		e.snapCache[req.Topic] = topicSnaps
	}

	snap, ok := topicSnaps[snapCacheKey(req)]
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
	e.snapCache[req.Topic][snapCacheKey(req)] = snap

	// Setup a cache eviction
	time.AfterFunc(e.snapCacheTTL, func() {
		e.lock.Lock()
		defer e.lock.Unlock()
		delete(e.snapCache[req.Topic], snapCacheKey(req))
	})
}

func snapCacheKey(req *SubscribeRequest) string {
	return req.Partition + "/" + req.Namespace + "/" + req.Key
}
