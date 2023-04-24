/*
Package stream provides a publish/subscribe system for events produced by changes
to the state store.
*/
package stream

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// Topic is an identifier that partitions events. A subscription will only receive
// events which match the Topic.
type Topic fmt.Stringer

// Subject identifies a portion of a topic for which a subscriber wishes to
// receive events (e.g. health events for a particular service) usually the
// normalized resource name (including partition and namespace if applicable).
type Subject fmt.Stringer

const (
	// SubjectNone is used when all events on a given topic are "global" and not
	// further partitioned by subject. For example: the "CA Roots" topic which is
	// used to notify subscribers when the global set CA root certificates changes.
	SubjectNone StringSubject = "none"

	// SubjectWildcard is used to subscribe to all events on a topic, regardless
	// of their subject. For example: mesh gateways need to consume *all* service
	// resolver config entries.
	//
	// Note: not all topics support wildcard subscriptions.
	SubjectWildcard StringSubject = "♣"
)

// Event is a structure with identifiers and a payload. Events are Published to
// EventPublisher and returned to Subscribers.
type Event struct {
	Topic   Topic
	Index   uint64
	Payload Payload
}

// A Payload contains the topic-specific data in an event. The payload methods
// should not modify the state of the payload if the Event is being submitted to
// EventPublisher.Publish.
type Payload interface {
	// HasReadPermission uses the acl.Authorizer to determine if the items in the
	// Payload are visible to the request. It returns true if the payload is
	// authorized for Read, otherwise returns false.
	HasReadPermission(authz acl.Authorizer) bool

	// Subject is used to identify which subscribers should be notified of this
	// event - e.g. those subscribing to health events for a particular service.
	// it is usually the normalized resource name (including the partition and
	// namespace if applicable).
	Subject() Subject

	// ToSubscriptionEvent is used to convert streaming events to their
	// serializable equivalent.
	ToSubscriptionEvent(idx uint64) *pbsubscribe.Event
}

// PayloadEvents is a Payload that may be returned by Subscription.Next when
// there are multiple events at an index.
//
// Note that unlike most other Payload, PayloadEvents is mutable and it is NOT
// safe to send to EventPublisher.Publish.
type PayloadEvents struct {
	Items []Event
}

func newPayloadEvents(items ...Event) *PayloadEvents {
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

func (p *PayloadEvents) Len() int {
	return len(p.Items)
}

// HasReadPermission filters the PayloadEvents to those which are authorized
// for reading by authz.
func (p *PayloadEvents) HasReadPermission(authz acl.Authorizer) bool {
	return p.filter(func(event Event) bool {
		return event.Payload.HasReadPermission(authz)
	})
}

// Subject is required to satisfy the Payload interface but is not implemented
// by PayloadEvents. PayloadEvents structs are constructed by Subscription.Next
// *after* Subject has been used to dispatch the enclosed events to the correct
// buffer.
func (PayloadEvents) Subject() Subject {
	panic("PayloadEvents does not implement Subject")
}

func (p PayloadEvents) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index: idx,
		Payload: &pbsubscribe.Event_EventBatch{
			EventBatch: &pbsubscribe.EventBatch{
				Events: batchEventsFromEventSlice(p.Items),
			},
		},
	}
}

func batchEventsFromEventSlice(events []Event) []*pbsubscribe.Event {
	result := make([]*pbsubscribe.Event, len(events))
	for i := range events {
		event := events[i]
		result[i] = event.Payload.ToSubscriptionEvent(event.Index)
	}
	return result
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

// IsFramingEvent returns true if this is a framing event (e.g. EndOfSnapshot
// or NewSnapshotToFollow).
func (e Event) IsFramingEvent() bool {
	return e.IsEndOfSnapshot() || e.IsNewSnapshotToFollow()
}

type framingEvent struct{}

func (framingEvent) HasReadPermission(acl.Authorizer) bool {
	return true
}

// Subject is required by the Payload interface but is not implemented by
// framing events, as they are typically *manually* appended to the correct
// buffer and do not need to be routed using a Subject.
func (framingEvent) Subject() Subject {
	panic("framing events do not implement Subject")
}

func (framingEvent) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	panic("framingEvent does not implement ToSubscriptionEvent")
}

type endOfSnapshot struct {
	framingEvent
}

func (s endOfSnapshot) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index:   idx,
		Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
}

type newSnapshotToFollow struct {
	framingEvent
}

func (s newSnapshotToFollow) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index:   idx,
		Payload: &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true},
	}
}

type closeSubscriptionPayload struct {
	tokensSecretIDs []string
}

// closeSubscriptionPayload is only used internally and does not correspond to
// a subscription event that would be sent to clients.
func (s closeSubscriptionPayload) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	panic("closeSubscriptionPayload does not implement ToSubscriptionEvent")
}

func (closeSubscriptionPayload) HasReadPermission(acl.Authorizer) bool {
	return false
}

// Subject is required by the Payload interface but it is not implemented by
// closeSubscriptionPayload, as this event type is handled separately and not
// actually appended to the buffer.
func (closeSubscriptionPayload) Subject() Subject {
	panic("closeSubscriptionPayload does not implement Subject")
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
