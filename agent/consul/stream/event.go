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
	Key     string
	Index   uint64
	Payload interface{}
}

// Len returns the number of events contained within this event. If the Payload
// is a []Event, the length of that slice is returned. Otherwise 1 is returned.
func (e Event) Len() int {
	if batch, ok := e.Payload.([]Event); ok {
		return len(batch)
	}
	return 1
}

// Filter returns an Event filtered to only those Events where f returns true.
// If the second return value is false, every Event was removed by the filter.
func (e Event) Filter(f func(Event) bool) (Event, bool) {
	batch, ok := e.Payload.([]Event)
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

	filtered := make([]Event, 0, size)
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

type newSnapshotToFollow struct{}

type closeSubscriptionPayload struct {
	tokensSecretIDs []string
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
