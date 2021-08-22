/*
Package stream provides a publish/subscribe system for events produced by changes
to the state store.
*/
package stream

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

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

// A Payload contains the topic-specific data in an event. The payload methods
// should not modify the state of the payload if the Event is being submitted to
// EventPublisher.Publish.
type Payload interface {
	// MatchesKey must return true if the Payload should be included in a
	// subscription requested with the key, namespace, and partition.
	//
	// Generally this means that the payload matches the key, namespace, and
	// partition or the payload is a special framing event that should be
	// returned to every subscription.
	MatchesKey(key, namespace, partition string) bool

	// HasReadPermission uses the acl.Authorizer to determine if the items in the
	// Payload are visible to the request. It returns true if the payload is
	// authorized for Read, otherwise returns false.
	HasReadPermission(authz acl.Authorizer) bool
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

// MatchesKey filters the PayloadEvents to those which match the key,
// namespace, and partition.
func (p *PayloadEvents) MatchesKey(key, namespace, partition string) bool {
	return p.filter(func(event Event) bool {
		return event.Payload.MatchesKey(key, namespace, partition)
	})
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

type framingEvent struct{}

func (framingEvent) MatchesKey(string, string, string) bool {
	return true
}

func (framingEvent) HasReadPermission(acl.Authorizer) bool {
	return true
}

type endOfSnapshot struct {
	framingEvent
}

type newSnapshotToFollow struct {
	framingEvent
}

type closeSubscriptionPayload struct {
	tokensSecretIDs []string
}

func (closeSubscriptionPayload) MatchesKey(string, string, string) bool {
	return false
}

func (closeSubscriptionPayload) HasReadPermission(acl.Authorizer) bool {
	return false
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
