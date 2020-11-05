/*
Package stream provides a publish/subscribe system for events produced by changes
to the state store.
*/
package stream

import "fmt"

// Topic is an identifier that partitions events. A subscription will only receive
// events which match the Topic.
type Topic fmt.Stringer

// Event is a structure with identifiers and a payload. Events are Published to
// EventPublisher and returned to Subscribers.
type Event struct {
	Topic   Topic
	Index   uint64
	Payload Payload
}

type Payload interface {
	// FilterByKey must return true if the Payload should be included in a subscription
	// requested with the key and namespace.
	// Generally this means that the payload matches the key and namespace or
	// the payload is a special framing event that should be returned to every
	// subscription.
	// TODO: rename to MatchesKey
	FilterByKey(key, namespace string) bool
}

// PayloadEvents is an Payload which contains multiple Events.
type PayloadEvents struct {
	Items []Event
}

func NewPayloadEvents(items ...Event) *PayloadEvents {
	return &PayloadEvents{Items: items}
}

func (p *PayloadEvents) filter(f func(Event) bool) bool {
	items := p.Items

	// To avoid extra allocations, iterate over the list of events first and
	// get a count of the total desired size. This trades off some extra cpu
	// time in the worse case (when not all items match the filter), for
	// fewer memory allocations.
	var size int
	for idx := range items {
		if f(items[idx]) {
			size++
		}
	}
	if len(items) == size || size == 0 {
		return size != 0
	}

	filtered := make([]Event, 0, size)
	for idx := range items {
		event := items[idx]
		if f(event) {
			filtered = append(filtered, event)
		}
	}
	p.Items = filtered
	return true
}

func (p *PayloadEvents) FilterByKey(key, namespace string) bool {
	return p.filter(func(event Event) bool {
		return event.Payload.FilterByKey(key, namespace)
	})
}

func (p *PayloadEvents) Len() int {
	return len(p.Items)
}

// IsEndOfSnapshot returns true if this is a framing event that indicates the
// snapshot has completed. Subsequent events from Subscription.Next will be
// streamed as they occur.
func (e Event) IsEndOfSnapshot() bool {
	return e.Payload == endOfSnapshot{}
}

// IsNewSnapshotToFollow returns true if this is a framing event that indicates
// that the clients view is stale, and must be reset. Subsequent events from
// Subscription.Next will be a new snapshot, followed by an EndOfSnapshot event.
func (e Event) IsNewSnapshotToFollow() bool {
	return e.Payload == newSnapshotToFollow{}
}

type endOfSnapshot struct{}

func (endOfSnapshot) FilterByKey(string, string) bool {
	return true
}

type newSnapshotToFollow struct{}

func (newSnapshotToFollow) FilterByKey(string, string) bool {
	return true
}

type closeSubscriptionPayload struct {
	tokensSecretIDs []string
}

func (closeSubscriptionPayload) FilterByKey(string, string) bool {
	return true
}

// NewCloseSubscriptionEvent returns a special Event that is handled by the
// stream package, and is never sent to subscribers. EventProcessor handles
// these events, and closes any subscriptions which were created using a token
// which matches any of the tokenSecretIDs.
//
// tokenSecretIDs may contain duplicate IDs.
func NewCloseSubscriptionEvent(tokenSecretIDs []string) Event {
	return Event{Payload: closeSubscriptionPayload{tokensSecretIDs: tokenSecretIDs}}
}
