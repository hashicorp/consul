// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inmem

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// Watch implements the storage.Watch interface using a stream.Subscription.
type Watch struct {
	sub   *stream.Subscription
	query query

	// events holds excess events when they are bundled in a stream.PayloadEvents,
	// until Next is called again.
	events []stream.Event
}

// Next returns the next WatchEvent, blocking until one is available.
func (w *Watch) Next(ctx context.Context) (*pbresource.WatchEvent, error) {
	for {
		e, err := w.nextEvent(ctx)
		if err == stream.ErrSubForceClosed {
			return nil, storage.ErrWatchClosed
		}
		if err != nil {
			return nil, err
		}

		event := e.Payload.(eventPayload).event
		if w.query.matches(event.Resource) {
			return event, nil
		}
	}
}

func (w *Watch) nextEvent(ctx context.Context) (*stream.Event, error) {
	if len(w.events) != 0 {
		event := w.events[0]
		w.events = w.events[1:]
		return &event, nil
	}

	var idx uint64
	for {
		e, err := w.sub.Next(ctx)
		if err != nil {
			return nil, err
		}

		if e.IsFramingEvent() {
			continue
		}

		// This works around a *very* rare race-condition in the EventPublisher where
		// it's possible to see duplicate events when events are published at the same
		// time as the first subscription is created on a {topic, subject} pair.
		//
		// We see this problem when a call to WriteCAS is happening in parallel with
		// a call to WatchList. It happens because our snapshot handler returns events
		// that have not yet been published (in the gap between us committing changes
		// to MemDB and the EventPublisher dispatching events onto its event buffers).
		//
		// An intuitive solution to this problem would be to take eventLock in the
		// snapshot handler to avoid it racing with publishing, but this does not
		// work because publishing is asynchronous.
		//
		// We should fix this problem at the root, but it's complicated, so for now
		// we'll work around it.
		if e.Index <= idx {
			continue
		}
		idx = e.Index

		switch t := e.Payload.(type) {
		case eventPayload:
			return &e, nil
		case *stream.PayloadEvents:
			if len(t.Items) == 0 {
				continue
			}

			event, rest := t.Items[0], t.Items[1:]
			w.events = rest
			return &event, nil
		}
	}
}

// Close the watch and free its associated resources.
func (w *Watch) Close() { w.sub.Unsubscribe() }

var eventTopic = stream.StringTopic("resources")

type eventPayload struct {
	subject stream.Subject
	event   *pbresource.WatchEvent
}

func (p eventPayload) Subject() stream.Subject { return p.subject }

// These methods are required by the stream.Payload interface, but we don't use them.
func (eventPayload) HasReadPermission(acl.Authorizer) bool         { return false }
func (eventPayload) ToSubscriptionEvent(uint64) *pbsubscribe.Event { return nil }

type wildcardSubject struct {
	resourceType storage.UnversionedType
}

func (s wildcardSubject) String() string {
	return s.resourceType.Group + indexSeparator +
		s.resourceType.Kind + indexSeparator +
		storage.Wildcard
}

type tenancySubject struct {
	resourceType storage.UnversionedType
	tenancy      *pbresource.Tenancy
}

func (s tenancySubject) String() string {
	return s.resourceType.Group + indexSeparator +
		s.resourceType.Kind + indexSeparator +
		s.tenancy.Partition + indexSeparator +
		s.tenancy.PeerName + indexSeparator +
		s.tenancy.Namespace
}

// publishEvent sends the event to the relevant Watches.
func (s *Store) publishEvent(idx uint64, op pbresource.WatchEvent_Operation, res *pbresource.Resource) {
	id := res.Id
	resourceType := storage.UnversionedTypeFrom(id.Type)
	event := &pbresource.WatchEvent{Operation: op, Resource: res}

	// We publish two copies of the event: one to the tenancy-specific subject and
	// another to a wildcard subject. Ideally, we'd be able to put the type in the
	// topic instead and use stream.SubjectWildcard, but this requires knowing all
	// types up-front (to register the snapshot handlers).
	s.pub.Publish([]stream.Event{
		{
			Topic: eventTopic,
			Index: idx,
			Payload: eventPayload{
				subject: wildcardSubject{resourceType},
				event:   event,
			},
		},
		{
			Topic: eventTopic,
			Index: idx,
			Payload: eventPayload{
				subject: tenancySubject{
					resourceType: resourceType,
					tenancy:      id.Tenancy,
				},
				event: event,
			},
		},
	})
}

// watchSnapshot implements a stream.SnapshotFunc to provide upsert events for
// the initial state of the world.
func (s *Store) watchSnapshot(req stream.SubscribeRequest, snap stream.SnapshotAppender) (uint64, error) {
	var q query
	switch t := req.Subject.(type) {
	case tenancySubject:
		q.resourceType = t.resourceType
		q.tenancy = t.tenancy
	case wildcardSubject:
		q.resourceType = t.resourceType
		q.tenancy = &pbresource.Tenancy{
			Partition: storage.Wildcard,
			PeerName:  storage.Wildcard,
			Namespace: storage.Wildcard,
		}
	default:
		return 0, fmt.Errorf("unhandled subject type: %T", req.Subject)
	}

	tx := s.txn(false)
	defer tx.Abort()

	idx, err := currentEventIndex(tx)
	if err != nil {
		return 0, err
	}

	results, err := listTxn(tx, q)
	if err != nil {
		return 0, nil
	}

	events := make([]stream.Event, len(results))
	for i, r := range results {
		events[i] = stream.Event{
			Topic: eventTopic,
			Index: idx,
			Payload: eventPayload{
				subject: req.Subject,
				event: &pbresource.WatchEvent{
					Operation: pbresource.WatchEvent_OPERATION_UPSERT,
					Resource:  r,
				},
			},
		}
	}
	snap.Append(events)

	return idx, nil
}
