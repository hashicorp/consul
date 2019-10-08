package state

import (
	"errors"
	"sync"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-memdb"
)

type EventPublisher struct {
	store *Store

	listeners map[stream.Topic]map[*stream.SubscribeRequest]chan stream.Event

	// These maps store the relations of ACL tokens, roles and policies.
	tokenSubs map[string]map[*stream.SubscribeRequest]chan stream.Event

	// lastIndex stores the index of the last event sent on each topic.
	lastIndex map[stream.Topic]uint64

	commitCh chan commitUpdate

	// staged stores the events to be published when the transaction is committed.
	staged     []stream.Event
	stagedLock sync.RWMutex

	// This lock protects the listeners and tokenSubs maps of active subscriptions.
	lock sync.RWMutex
}

type commitUpdate struct {
	tx     *memdb.Txn
	events []stream.Event
}

func NewEventPublisher(store *Store) *EventPublisher {
	e := &EventPublisher{
		store:     store,
		listeners: make(map[stream.Topic]map[*stream.SubscribeRequest]chan stream.Event),
		tokenSubs: make(map[string]map[*stream.SubscribeRequest]chan stream.Event),
		lastIndex: make(map[stream.Topic]uint64),
		commitCh:  make(chan commitUpdate, 64),
	}
	go e.handleUpdates()

	return e
}

// PreparePublish gets an event ready to publish to any listeners on the relevant
// topics. This doesn't do the send, which happens when the memdb transaction has
// been committed.
func (e *EventPublisher) PreparePublish(events []stream.Event) error {
	e.stagedLock.Lock()
	defer e.stagedLock.Unlock()

	if e.staged != nil {
		return errors.New("event already staged for commit")
	}

	e.staged = events

	return nil
}

// Commit triggers any staged events to be sent to the relevant listeners. This is called
// via txn.Defer to delay it from running until the transaction has been finalized.
func (e *EventPublisher) Commit() {
	e.stagedLock.Lock()
	defer e.stagedLock.Unlock()

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

	for _, event := range update.events {
		// If the event is an ACL update, treat it as a special case. Currently
		// ACL update events are only used internally to recognize when a subscriber
		// should reload its subscription.
		if event.Topic == stream.Topic_ACLTokens ||
			event.Topic == stream.Topic_ACLPolicies ||
			event.Topic == stream.Topic_ACLRoles {
			if err := e.handleACLUpdate(update.tx, event); err != nil {
				panic(err)
			}

			continue
		}

		// If the event isn't an ACL update, send it to the relevant subscribers.
		event.SetACLRules()
		for subscription, listener := range e.listeners[event.Topic] {
			// If this event doesn't pertain to the subset this subscription is listening for,
			// skip sending it. We'll probably need more nuanced logic here later.
			if subscription.Key != event.Key && subscription.Key != "" {
				continue
			}

			select {
			case listener <- event:
			default:
				close(listener)
				delete(e.listeners[subscription.Topic], subscription)
			}
		}

		// Update the last published index for the topic.
		if event.Index > e.lastIndex[event.Topic] {
			e.lastIndex[event.Topic] = event.Index
		}
	}
}

// handleACLUpdate handles an ACL token/policy/role update. This method assumes the lock is held.
func (e *EventPublisher) handleACLUpdate(tx *memdb.Txn, event stream.Event) error {
	switch event.Topic {
	case stream.Topic_ACLTokens:
		token := event.GetACLToken()
		subscribers, ok := e.tokenSubs[token.Token.AccessorID]

		// If there are subscribers using the updated/deleted token, signal them
		// to reload their connection.
		if ok {
			e.reloadSubscribers(subscribers)
		}
	case stream.Topic_ACLPolicies:
		policy := event.GetACLPolicy()
		affectedSubs := make(map[*stream.SubscribeRequest]chan stream.Event)
		for token, subscribers := range e.tokenSubs {
			token, err := e.store.aclTokenGetTxn(tx, nil, token, "id", nil)
			if err != nil {
				return err
			}
			if token == nil {
				continue
			}

			// If the updated policy was used in this token, add any subscribers using it
			// to the affected set.
			for _, p := range token.Policies {
				if p.ID == policy.PolicyID {
					for sub, ch := range subscribers {
						affectedSubs[sub] = ch
					}
					break
				}
			}
		}

		// Send a reload to the affected subscribers.
		e.reloadSubscribers(affectedSubs)
	case stream.Topic_ACLRoles:
		role := event.GetACLRole()
		affectedSubs := make(map[*stream.SubscribeRequest]chan stream.Event)
		for token, subscribers := range e.tokenSubs {
			token, err := e.store.aclTokenGetTxn(tx, nil, token, "id", nil)
			if err != nil {
				return err
			}
			if token == nil {
				continue
			}

			// If the updated role was used in this token, add any subscribers using it
			// to the affected set.
			for _, r := range token.Roles {
				if r.ID == role.RoleID {
					for sub, ch := range subscribers {
						affectedSubs[sub] = ch
					}
					break
				}
			}
		}

		// Send a reload to the affected subscribers.
		e.reloadSubscribers(affectedSubs)
	}

	return nil
}

// reloadSubscribers sends a reload signal to all subscribers in the given map. This
// method assumes the lock is held.
func (e *EventPublisher) reloadSubscribers(subscribers map[*stream.SubscribeRequest]chan stream.Event) {
	for subscription, listener := range subscribers {
		// Send a reload event and deregister the subscriber.
		reloadEvent := stream.Event{
			Payload: &stream.Event_ReloadStream{ReloadStream: true},
		}
		e.nonBlockingListenerSend(listener, subscription, reloadEvent)
		e.unsubscribeLocked(subscription)
	}
}

// nonBlockingListenerSend sends an event to a listener in a non-blocking way, closing the
// subscription if the send fails. This method assumes the lock is held.
func (e *EventPublisher) nonBlockingListenerSend(listener chan stream.Event, subscription *stream.SubscribeRequest, event stream.Event) {
	select {
	case listener <- event:
	default:
		close(listener)
		e.unsubscribeLocked(subscription)
	}
}

// LastTopicIndex returns the index of the last event published for the topic.
func (e *EventPublisher) LastTopicIndex(topic stream.Topic) uint64 {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.lastIndex[topic]
}

func (e *EventPublisher) Subscribe(subscription *stream.SubscribeRequest) (<-chan stream.Event, error) {
	ch := make(chan stream.Event, 32)

	e.lock.Lock()
	defer e.lock.Unlock()
	if topicListeners, ok := e.listeners[subscription.Topic]; ok {
		topicListeners[subscription] = ch
	} else {
		e.listeners[subscription.Topic] = map[*stream.SubscribeRequest]chan stream.Event{
			subscription: ch,
		}
	}

	// Add the subscription to the ACL token map.
	var accessor string
	_, token, err := e.store.ACLTokenGetBySecret(nil, subscription.Token)
	if err != nil {
		return nil, err
	}
	if token != nil {
		accessor = token.AccessorID
	}

	if subs, ok := e.tokenSubs[accessor]; ok {
		subs[subscription] = ch
	} else {
		e.tokenSubs[accessor] = map[*stream.SubscribeRequest]chan stream.Event{
			subscription: ch,
		}
	}

	return ch, nil
}

func (e *EventPublisher) Unsubscribe(subscription *stream.SubscribeRequest) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.unsubscribeLocked(subscription)
}

func (e *EventPublisher) unsubscribeLocked(subscription *stream.SubscribeRequest) {
	// Clean up the topic -> subscribers map.
	delete(e.listeners[subscription.Topic], subscription)
	if len(e.listeners[subscription.Topic]) == 0 {
		delete(e.listeners, subscription.Topic)
	}

	// Clean up the token -> subscribers map.
	delete(e.tokenSubs[subscription.Token], subscription)
	if len(e.tokenSubs[subscription.Token]) == 0 {
		delete(e.tokenSubs, subscription.Token)
	}
}
