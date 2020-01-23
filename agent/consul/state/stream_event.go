package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-memdb"
	"golang.org/x/crypto/blake2b"
)

type EventPublisher struct {
	store *Store

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
	// seconds. TODO
	snapCacheTTL time.Duration

	// This lock protects the topicBuffers, snapCache and subsByToken maps.
	lock sync.RWMutex

	// topicBuffers stores the head of the linked-list buffer to publish events to
	// for a topic.
	topicBuffers map[stream.Topic]*stream.EventBuffer

	// snapCache stores the head of any snapshot buffers still in cache if caching
	// is enabled.
	snapCache map[stream.Topic]map[string]*stream.EventSnapshot

	// snapFns is the set of snapshot functions that were registered bound to the
	// state store.
	snapFns map[stream.Topic]stream.SnapFn

	// subsByToken stores a list of Subscription objects outstanding indexed by a
	// hash of the ACL token they used to subscribe so we can reload them if their
	// ACL permissions change.
	subsByToken map[string]map[*stream.SubscribeRequest]*stream.Subscription

	// commitCh decouples the Commit call in the FSM hot path from distributing
	// the resulting events.
	commitCh chan commitUpdate

	// staged stores the events to be published when the transaction is committed.
	staged     []stream.Event
	stagedLock sync.RWMutex
}

type commitUpdate struct {
	tx     *memdb.Txn
	events []stream.Event
}

func NewEventPublisher(store *Store, topicBufferSize int, snapCacheTTL time.Duration) *EventPublisher {
	e := &EventPublisher{
		store:           store,
		topicBufferSize: topicBufferSize,
		snapCacheTTL:    snapCacheTTL,
		topicBuffers:    make(map[stream.Topic]*stream.EventBuffer),
		snapCache:       make(map[stream.Topic]map[string]*stream.EventSnapshot),
		snapFns:         make(map[stream.Topic]stream.SnapFn),
		subsByToken:     make(map[string]map[*stream.SubscribeRequest]*stream.Subscription),
		commitCh:        make(chan commitUpdate, 64),
	}

	// create a local handler table
	for topic, handlers := range topicRegistry {
		fnCopy := handlers.Snapshot
		e.snapFns[topic] = func(req *stream.SubscribeRequest, buf *stream.EventBuffer) (uint64, error) {
			return fnCopy(e.store, req, buf)
		}
	}

	go e.handleUpdates()

	return e
}

// PreparePublish accepts a set of events to be published. All events must have
// the same index which must be higher than any previously Committed events.
// Events will not be delivered to subscribers until Commit is called. If
// PreparePublish is called a second time before Commit, and the events have a
// higher index, it is assumed the previous set of events were aborted and they
// will be dropped. If the events in a subsequent call have the same index, they
// will be staged in addition to those already prepared and all will be
// committed on next Commit.
func (e *EventPublisher) PreparePublish(events []stream.Event) error {
	if len(events) == 0 {
		// no-op
		return nil
	}

	e.stagedLock.Lock()
	defer e.stagedLock.Unlock()

	if len(e.staged) > 0 {
		if e.staged[0].Index != events[0].Index {
			// implicit abort - these events are from a different index, replace
			// current staged events.
			e.staged = events
		} else {
			// Merge events at same index
			e.staged = append(e.staged, events...)
		}
	} else {
		e.staged = events
	}
	return nil
}

// Commit triggers any staged events to be sent to the relevant listeners. This is called
// via txn.Defer to delay it from running until the transaction has been finalized.
func (e *EventPublisher) Commit() {
	e.stagedLock.Lock()
	defer e.stagedLock.Unlock()

	if len(e.staged) == 0 {
		return
	}

	e.commitCh <- commitUpdate{
		tx:     e.store.db.Txn(false),
		events: e.staged,
	}
	e.staged = nil
}

func (e *EventPublisher) handleUpdates() {
	for {
		update := <-e.commitCh
		e.sendEvents(update)
	}
}

// sendEvents sends the given events to any applicable topic listeners, as well as
// any ACL update events to cause affected listeners to reset their stream.
func (e *EventPublisher) sendEvents(update commitUpdate) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Always abort the transaction. This is not strictly necessary with memDB
	// because once we drop the reference to the Txn object, the radix nodes will
	// be GCed anyway but it's hygienic incase memDB ever has a different
	// implementation.
	defer update.tx.Abort()

	eventsByTopic := make(map[stream.Topic][]stream.Event)

	for _, event := range update.events {
		// If the event is an ACL update, treat it as a special case. Currently
		// ACL update events are only used internally to recognize when a subscriber
		// should reload its subscription.
		if event.Topic == stream.Topic_ACLTokens ||
			event.Topic == stream.Topic_ACLPolicies ||
			event.Topic == stream.Topic_ACLRoles {
			if err := e.handleACLUpdate(update.tx, event); err != nil {
				// This seems pretty drastic? What would be better. It's not super safe
				// to continue since we might have missed some ACL update and so leak
				// data to unauthorized clients but crashing whole server also seems
				// bad. I wonder if we could send a "reset" to all subscribers instead
				// and effectively re-start all subscriptions to be on the safe side
				// without just crashing?
				// TODO(banks): reset all instead of panic?
				panic(err)
			}

			continue
		}

		// If the event isn't an ACL update, send it to the relevant subscribers.
		event.SetACLRules()

		// Split events by topic to deliver.
		eventsByTopic[event.Topic] = append(eventsByTopic[event.Topic], event)
	}

	// Deliver events
	for topic, events := range eventsByTopic {
		buf, ok := e.topicBuffers[topic]
		if !ok {
			buf = stream.NewEventBuffer()
			e.topicBuffers[topic] = buf
		}
		buf.Append(events)
	}
}

// handleACLUpdate handles an ACL token/policy/role update. This method assumes the lock is held.
func (e *EventPublisher) handleACLUpdate(tx *memdb.Txn, event stream.Event) error {
	switch event.Topic {
	case stream.Topic_ACLTokens:
		token := event.GetACLToken()
		subs := e.subsByToken[secretHash(token.Token.SecretID)]

		for _, sub := range subs {
			sub.CloseReload()
		}
	case stream.Topic_ACLPolicies:
		policy := event.GetACLPolicy()
		tokens, err := e.store.aclTokenListTxn(tx, nil, true, true, policy.PolicyID, "", "", nil)
		if err != nil {
			return err
		}

		// Loop through the tokens used by the policy.
		for _, token := range tokens {
			if subs, ok := e.subsByToken[secretHash(token.SecretID)]; ok {
				for _, sub := range subs {
					sub.CloseReload()
				}
			}
		}

		// Find any roles using this policy so tokens with those roles can be reloaded.
		roles, err := e.store.aclRoleListTxn(tx, nil, policy.PolicyID, nil)
		if err != nil {
			return err
		}
		for _, role := range roles {
			tokens, err := e.store.aclTokenListTxn(tx, nil, true, true, "", role.ID, "", nil)
			if err != nil {
				return err
			}
			for _, token := range tokens {
				if subs, ok := e.subsByToken[secretHash(token.SecretID)]; ok {
					for _, sub := range subs {
						sub.CloseReload()
					}
				}
			}
		}

	case stream.Topic_ACLRoles:
		role := event.GetACLRole()
		tokens, err := e.store.aclTokenListTxn(tx, nil, true, true, "", role.RoleID, "", nil)
		if err != nil {
			return err
		}

		// Loop through the tokens used by the role and add the affected subscribers to the list.
		for _, token := range tokens {
			if subs, ok := e.subsByToken[secretHash(token.SecretID)]; ok {
				for _, sub := range subs {
					sub.CloseReload()
				}
			}
		}
	}

	return nil
}

// secretHash returns a 256-bit Blake2 hash of the given string.
func secretHash(token string) string {
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	hash.Write([]byte(token))
	return string(hash.Sum(nil))
}

func (e *EventPublisher) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	// Ensure we know how to make a snapshot for this topic
	_, ok := topicRegistry[req.Topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic %s", req.Topic)
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	// Ensure there is a topic buffer for that topic so we start capturing any
	// future published events.
	buf, ok := e.topicBuffers[req.Topic]
	if !ok {
		buf = stream.NewEventBuffer()
		e.topicBuffers[req.Topic] = buf
	}

	// See if we need a snapshot
	topicHead := buf.Head()
	var sub *stream.Subscription
	if req.Index > 0 && len(topicHead.Events) > 0 && topicHead.Events[0].Index == req.Index {
		// No need for a snapshot just send the "end snapshot" message to signal to
		// client it's cache is still good. (note that this can be distinguished
		// from a legitimate empty snapshot due to the index matching the one the
		// client sent), then follow along from here in the topic.
		e := stream.Event{
			Index:   req.Index,
			Topic:   req.Topic,
			Key:     req.Key,
			Payload: &stream.Event_ResumeStream{ResumeStream: true},
		}
		// Make a new buffer to send to the client containing the resume.
		buf := stream.NewEventBuffer()

		// Store the head of that buffer before we append to it to give as the
		// starting point for the subscription.
		subHead := buf.Head()

		buf.Append([]stream.Event{e})

		// Now splice the rest of the topic buffer on so the subscription will
		// continue to see future updates in the topic buffer.
		follow, err := topicHead.FollowAfter()
		if err != nil {
			return nil, err
		}
		buf.AppendBuffer(follow)

		sub = stream.NewSubscription(req, subHead)
	} else {
		snap, err := e.getSnapshotLocked(req, topicHead)
		if err != nil {
			return nil, err
		}
		sub = stream.NewSubscription(req, snap.Snap)
	}

	// Add the subscription to the ACL token map.
	tokenHash := secretHash(req.Token)
	subsByToken, ok := e.subsByToken[tokenHash]
	if !ok {
		subsByToken = make(map[*stream.SubscribeRequest]*stream.Subscription)
		e.subsByToken[tokenHash] = subsByToken
	}
	subsByToken[req] = sub

	return sub, nil
}

func (e *EventPublisher) Unsubscribe(req *stream.SubscribeRequest) {
	e.lock.Lock()
	defer e.lock.Unlock()

	tokenHash := secretHash(req.Token)
	subsByToken, ok := e.subsByToken[tokenHash]
	if !ok {
		return
	}
	delete(subsByToken, req)
	if len(subsByToken) == 0 {
		delete(e.subsByToken, tokenHash)
	}
}

func (e *EventPublisher) getSnapshotLocked(req *stream.SubscribeRequest, topicHead *stream.BufferItem) (*stream.EventSnapshot, error) {
	// See if there is a cached snapshot
	topicSnaps, ok := e.snapCache[req.Topic]
	if !ok {
		topicSnaps = make(map[string]*stream.EventSnapshot)
		e.snapCache[req.Topic] = topicSnaps
	}

	snap, ok := topicSnaps[req.Key]
	if ok && snap.Err() == nil {
		return snap, nil
	}

	// No snap or errored snap in cache, create a new one
	snapFn, ok := e.snapFns[req.Topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic %s", req.Topic)
	}

	snap = stream.NewEventSnapshot(req, topicHead, snapFn)
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
