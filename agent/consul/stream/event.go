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
	FilterByKey(key, namespace string) bool
}

// Len returns the number of events contained within this event. If the Payload
// is a []Event, the length of that slice is returned. Otherwise 1 is returned.
func (e Event) Len() int {
	if batch, ok := e.Payload.(PayloadEvents); ok {
		return len(batch)
	}
	return 1
}

// Filter returns an Event filtered to only those Events where f returns true.
// If the second return value is false, every Event was removed by the filter.
func (e Event) Filter(f func(Event) bool) (Event, bool) {
	batch, ok := e.Payload.(PayloadEvents)
	if !ok {
		return e, f(e)
	}

	// To avoid extra allocations, iterate over the list of events first and
	// get a count of the total desired size. This trades off some extra cpu
	// time in the worse case (when not all items match the filter), for
	// fewer memory allocations.
	var size int
	for idx := range batch {
		if f(batch[idx]) {
			size++
		}
	}
	if len(batch) == size || size == 0 {
		return e, size != 0
	}

	filtered := make(PayloadEvents, 0, size)
	for idx := range batch {
		event := batch[idx]
		if f(event) {
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == 0 {
		return e, false
	}
	e.Payload = filtered
	return e, true
}

// PayloadEvents is an Payload which contains multiple Events.
type PayloadEvents []Event

// TODO: this method is not called, but needs to exist so that we can store
// a slice of events as a payload. In the future we should be able to refactor
// Event.Filter so that this FilterByKey includes the re-slicing.
func (e PayloadEvents) FilterByKey(_, _ string) bool {
	return true
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
