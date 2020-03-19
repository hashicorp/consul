package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-memdb"
	"golang.org/x/crypto/blake2b"

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
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
	topicBuffers map[agentpb.Topic]*stream.EventBuffer

	// snapCache stores the head of any snapshot buffers still in cache if caching
	// is enabled.
	snapCache map[agentpb.Topic]map[string]*stream.EventSnapshot

	// snapFns is the set of snapshot functions that were registered bound to the
	// state store.
	snapFns map[agentpb.Topic]stream.SnapFn

	// subsByToken stores a list of Subscription objects outstanding indexed by a
	// hash of the ACL token they used to subscribe so we can reload them if their
	// ACL permissions change.
	subsByToken map[string]map[*agentpb.SubscribeRequest]*stream.Subscription

	// commitCh decouples the Commit call in the FSM hot path from distributing
	// the resulting events.
	commitCh chan commitUpdate

	// staged stores the events to be published when the transaction is committed.
	staged     []agentpb.Event
	stagedLock sync.RWMutex
}

type commitUpdate struct {
	tx     *txnWrapper
	events []agentpb.Event
}

func NewEventPublisher(store *Store, topicBufferSize int, snapCacheTTL time.Duration) *EventPublisher {
	e := &EventPublisher{
		store:           store,
		topicBufferSize: topicBufferSize,
		snapCacheTTL:    snapCacheTTL,
		topicBuffers:    make(map[agentpb.Topic]*stream.EventBuffer),
		snapCache:       make(map[agentpb.Topic]map[string]*stream.EventSnapshot),
		snapFns:         make(map[agentpb.Topic]stream.SnapFn),
		subsByToken:     make(map[string]map[*agentpb.SubscribeRequest]*stream.Subscription),
		commitCh:        make(chan commitUpdate, 64),
	}

	// create a local handler table
	for topic, handlers := range topicRegistry {
		fnCopy := handlers.Snapshot
		e.snapFns[topic] = func(req *agentpb.SubscribeRequest, buf *stream.EventBuffer) (uint64, error) {
			return fnCopy(e.store, req, buf)
		}
	}

	go e.handleUpdates()

	return e
}

// // PreparePublish accepts a set of events to be published. All events must have
// // the same index which must be higher than any previously Committed events.
// // Events will not be delivered to subscribers until Commit is called. If
// // PreparePublish is called a second time before Commit, and the events have a
// // higher index, it is assumed the previous set of events were aborted and they
// // will be dropped. If the events in a subsequent call have the same index, they
// // will be staged in addition to those already prepared and all will be
// // committed on next Commit.
// func (e *EventPublisher) PreparePublish(events []agentpb.Event) error {
// 	if len(events) == 0 {
// 		// no-op
// 		return nil
// 	}

// 	e.stagedLock.Lock()
// 	defer e.stagedLock.Unlock()

// 	if len(e.staged) > 0 {
// 		if e.staged[0].Index != events[0].Index {
// 			// implicit abort - these events are from a different index, replace
// 			// current staged events.
// 			e.staged = events
// 		} else {
// 			// Merge events at same index
// 			e.staged = append(e.staged, events...)
// 		}
// 	} else {
// 		e.staged = events
// 	}
// 	return nil
// }

// // Commit triggers any staged events to be sent to the relevant listeners. This
// // is called via txn.Defer to delay it from running until the transaction has
// // been finalized.
// func (e *EventPublisher) Commit() {
// 	e.stagedLock.Lock()
// 	defer e.stagedLock.Unlock()

// 	if len(e.staged) == 0 {
// 		return
// 	}

// 	e.commitCh <- commitUpdate{
// 		tx:     e.store.db.Txn(false),
// 		events: e.staged,
// 	}
// 	e.staged = nil
// }

func (e *EventPublisher) publishChanges(tx *txnWrapper, changes memdb.Changes) error {
	var events []agentpb.Event
	for topic, th := range topicRegistry {
		if th.ProcessChanges != nil {
			es, err := th.ProcessChanges(e.store, tx, changes)
			if err != nil {
				return fmt.Errorf("failed generating events for topic %q: %s", topic, err)
			}
			events = append(events, es...)
		}
	}
	e.commitCh <- commitUpdate{
		// Create a new transaction since it's going to be used from a different
		// thread. Transactions aren't thread safe but it's OK to create it here
		// since we won't try to use it in this thread and pass it straight to the
		// handler which will own it exclusively.
		tx:     e.store.db.Txn(false),
		events: events,
	}
	return nil
}

func (e *EventPublisher) handleUpdates() {
	for {
		update := <-e.commitCh
		e.sendEvents(update)
	}
}

// sendEvents sends the given events to any applicable topic listeners, as well
// as any ACL update events to cause affected listeners to reset their stream.
func (e *EventPublisher) sendEvents(update commitUpdate) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Always abort the transaction. This is not strictly necessary with memDB
	// because once we drop the reference to the Txn object, the radix nodes will
	// be GCed anyway but it's hygienic incase memDB ever has a different
	// implementation.
	defer update.tx.Abort()

	eventsByTopic := make(map[agentpb.Topic][]agentpb.Event)

	for _, event := range update.events {
		// If the event is an ACL update, treat it as a special case. Currently
		// ACL update events are only used internally to recognize when a subscriber
		// should reload its subscription.
		if event.Topic == agentpb.Topic_ACLTokens ||
			event.Topic == agentpb.Topic_ACLPolicies ||
			event.Topic == agentpb.Topic_ACLRoles {

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

// handleACLUpdate handles an ACL token/policy/role update. This method assumes
// the lock is held.
func (e *EventPublisher) handleACLUpdate(tx *txnWrapper, event agentpb.Event) error {
	switch event.Topic {
	case agentpb.Topic_ACLTokens:
		token := event.GetACLToken()
		subs := e.subsByToken[secretHash(token.Token.SecretID)]
		for _, sub := range subs {
			sub.CloseReload()
		}
	case agentpb.Topic_ACLPolicies:
		policy := event.GetACLPolicy()
		// TODO(streaming) figure out how to thread method/ent meta here for
		// namespace support in Ent. Probably need wildcard here?
		tokens, err := e.store.aclTokenListByPolicy(tx, policy.PolicyID, nil)
		if err != nil {
			return err
		}

		// Loop through the tokens used by the policy.
		for token := tokens.Next(); token != nil; token = tokens.Next() {
			token := token.(*structs.ACLToken)
			if subs, ok := e.subsByToken[secretHash(token.SecretID)]; ok {
				for _, sub := range subs {
					sub.CloseReload()
				}
			}
		}

		// Find any roles using this policy so tokens with those roles can be reloaded.
		roles, err := e.store.aclRoleListByPolicy(tx, policy.PolicyID, nil)
		if err != nil {
			return err
		}
		for role := roles.Next(); role != nil; role = roles.Next() {
			role := role.(*structs.ACLRole)

			// TODO(streaming) figure out how to thread method/ent meta here for
			// namespace support in Ent.
			tokens, err := e.store.aclTokenListByRole(tx, role.ID, nil)
			if err != nil {
				return err
			}
			for token := tokens.Next(); token != nil; token = tokens.Next() {
				token := token.(*structs.ACLToken)
				if subs, ok := e.subsByToken[secretHash(token.SecretID)]; ok {
					for _, sub := range subs {
						sub.CloseReload()
					}
				}
			}
		}

	case agentpb.Topic_ACLRoles:
		role := event.GetACLRole()
		// TODO(streaming) figure out how to thread method/ent meta here for
		// namespace support in Ent.
		tokens, err := e.store.aclTokenListByRole(tx, role.RoleID, nil)
		if err != nil {
			return err
		}
		for token := tokens.Next(); token != nil; token = tokens.Next() {
			token := token.(*structs.ACLToken)
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

// Subscribe returns a new stream.Subscription for the given request. A
// subscription will stream an initial snapshot of events matching the request
// if required and then block until new events that modify the request occur, or
// the context is cancelled. Subscriptions may be forced to reset if the server
// decides it can no longer maintain correct operation for example if ACL
// policies changed or the state store was restored.
//
// When the called is finished with the subscription for any reason, it must
// call Unsubscribe to free ACL tracking resources.
func (e *EventPublisher) Subscribe(ctx context.Context,
	req *agentpb.SubscribeRequest) (*stream.Subscription, error) {
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
		e := agentpb.Event{
			Index:   req.Index,
			Topic:   req.Topic,
			Key:     req.Key,
			Payload: &agentpb.Event_ResumeStream{ResumeStream: true},
		}
		// Make a new buffer to send to the client containing the resume.
		buf := stream.NewEventBuffer()

		// Store the head of that buffer before we append to it to give as the
		// starting point for the subscription.
		subHead := buf.Head()

		buf.Append([]agentpb.Event{e})

		// Now splice the rest of the topic buffer on so the subscription will
		// continue to see future updates in the topic buffer.
		follow, err := topicHead.FollowAfter()
		if err != nil {
			return nil, err
		}
		buf.AppendBuffer(follow)

		sub = stream.NewSubscription(ctx, req, subHead)
	} else {
		snap, err := e.getSnapshotLocked(req, topicHead)
		if err != nil {
			return nil, err
		}
		sub = stream.NewSubscription(ctx, req, snap.Snap)
	}

	// Add the subscription to the ACL token map.
	tokenHash := secretHash(req.Token)
	subsByToken, ok := e.subsByToken[tokenHash]
	if !ok {
		subsByToken = make(map[*agentpb.SubscribeRequest]*stream.Subscription)
		e.subsByToken[tokenHash] = subsByToken
	}
	subsByToken[req] = sub

	return sub, nil
}

// Unsubscribe must be called when a client is no longer interested in a
// subscription to free resources monitoring changes in it's ACL token. The same
// request object passed to Subscribe must be used.
func (e *EventPublisher) Unsubscribe(req *agentpb.SubscribeRequest) {
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

func (e *EventPublisher) getSnapshotLocked(req *agentpb.SubscribeRequest, topicHead *stream.BufferItem) (*stream.EventSnapshot, error) {
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
