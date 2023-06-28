// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stream

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

// eventBuffer is a single-writer, multiple-reader, unlimited length concurrent
// buffer of events that have been published on a topic. The buffer is
// effectively just the head of an atomically updated single-linked list. Atomic
// accesses are usually to be suspected as premature optimization but this
// specific design has several important features that significantly simplify a
// lot of our PubSub machinery.
//
// The eventBuffer only tracks the most recent set of published events, so
// if there are no consumers, older events are automatically garbage collected.
// Consumers are notified of new events by closing a channel on the previous head
// allowing efficient broadcast to many watchers without having to run multiple
// goroutines or deliver to O(N) separate channels.
//
// Because eventBuffer is a linked list with atomically updated pointers, readers don't
// have to take a lock and can consume at their own pace. We also don't need a
// fixed limit on the number of items, which avoids needing to configure
// buffer length to balance wasting memory, against being able to tolerate
// occasionally slow readers.
//
// The buffer is used to deliver all messages broadcast to a topic for active
// subscribers to consume, but it is also an effective way to both deliver and
// optionally cache snapshots per topic and key. By using an eventBuffer,
// snapshot functions don't have to read the whole snapshot into memory before
// delivery - they can stream from memdb. However simply by storing a pointer to
// the first event in the buffer, we can cache the buffered events for future
// watchers on the same topic. Finally, once we've delivered all the snapshot
// events to the buffer, we can append a next-element which is the first topic
// buffer element with a higher index and so consumers can keep reading the
// same buffer to have subsequent updates streamed after the snapshot is read.
//
// A huge benefit here is that caching snapshots becomes very simple - we don't
// have to do any additional book-keeping to figure out when to truncate the
// topic buffer to make sure the snapshot is still usable or run into issues
// where the cached snapshot is no longer useful since the buffer will keep
// elements around only as long as either the cache or a subscriber need them.
// So we can use whatever simple timeout logic we like to decide how long to
// keep caches (or if we should keep them at all) and the buffers will
// automatically keep the events we need to make that work for exactly the
// optimal amount of time and no longer.
//
// A new buffer is constructed with a sentinel "empty" bufferItem that has a nil
// Events array. This enables subscribers to start watching for the next update
// immediately.
//
// The zero value eventBuffer is _not_ usable, as it has not been
// initialized with an empty bufferItem so can not be used to wait for the first
// published event. Call newEventBuffer to construct a new buffer.
//
// Calls to Append or AppendBuffer that mutate the head must be externally
// synchronized. This allows systems that already serialize writes to append
// without lock overhead (e.g. a snapshot goroutine appending thousands of
// events).
type eventBuffer struct {
	head atomic.Value
}

// newEventBuffer creates an eventBuffer ready for use.
func newEventBuffer() *eventBuffer {
	b := &eventBuffer{}
	b.head.Store(newBufferItem(nil))
	return b
}

// Append a set of events from one raft operation to the buffer and notify
// watchers. Note that events must not have been previously made available to
// any other goroutine since we may mutate them to ensure ACL Rules are
// populated. After calling append, the caller must not make any further
// mutations to the events as they may have been exposed to subscribers in other
// goroutines. Append only supports a single concurrent caller and must be
// externally synchronized with other Append, AppendBuffer or AppendErr calls.
func (b *eventBuffer) Append(events []Event) {
	b.AppendItem(newBufferItem(events))
}

// AppendItem joins another buffer which may be the tail of a separate buffer
// for example a buffer that's had the events from a snapshot appended may
// finally by linked to the topic buffer for the subsequent events so
// subscribers can seamlessly consume the updates. Note that Events in item must
// already be fully populated with ACL rules and must not be mutated further as
// they may have already been published to subscribers.
//
// AppendBuffer only supports a single concurrent caller and must be externally
// synchronized with other Append, AppendBuffer or AppendErr calls.
func (b *eventBuffer) AppendItem(item *bufferItem) {
	// First store it as the next node for the old head this ensures once it's
	// visible to new searchers the linked list is already valid. Not sure it
	// matters but this seems nicer.
	oldHead := b.Head()
	oldHead.link.next.Store(item)
	b.head.Store(item)

	// Now it's added invalidate the oldHead to notify waiters
	close(oldHead.link.ch)
	// don't set chan to nil since that will race with readers accessing it.
}

// Head returns the current head of the buffer. It will always exist but it may
// be a "sentinel" empty item with a nil Events slice to allow consumers to
// watch for the next update. Consumers should always check for empty Events and
// treat them as no-ops. Will panic if eventBuffer was not initialized correctly
// with eventBuffer.
func (b *eventBuffer) Head() *bufferItem {
	return b.head.Load().(*bufferItem)
}

// bufferItem represents a set of events published by a single raft operation.
// The first item returned by a newly constructed buffer will have nil Events.
// It is a sentinel value which is used to wait on the next events via Next.
//
// To iterate to the next event, a Next method may be called which may block if
// there is no next element yet.
//
// Holding a pointer to the item keeps all the events published since in memory
// so it's important that subscribers don't hold pointers to buffer items after
// they have been delivered except where it's intentional to maintain a cache or
// trailing store of events for performance reasons.
//
// Subscribers must not mutate the bufferItem or the Events or Encoded payloads
// inside as these are shared between all readers.
type bufferItem struct {
	// Events is the set of events published at one raft index. This may be nil as
	// a sentinel value to allow watching for the first event in a buffer. Callers
	// should check and skip nil Events at any point in the buffer. It will also
	// be nil if the producer appends an Error event because they can't complete
	// the request to populate the buffer. Err will be non-nil in this case.
	Events []Event

	// Err is non-nil if the producer can't complete their task and terminates the
	// buffer. Subscribers should return the error to clients and cease attempting
	// to read from the buffer.
	Err error

	// link holds the next pointer and channel. This extra bit of indirection
	// allows us to splice buffers together at arbitrary points without including
	// events in one buffer just for the side-effect of watching for the next set.
	// The link may not be mutated once the event is appended to a buffer.
	link *bufferLink
}

type bufferLink struct {
	// next is an atomically updated pointer to the next event in the buffer. It
	// is written exactly once by the single published and will always be set if
	// ch is closed.
	next atomic.Value

	// ch is closed when the next event is published. It should never be mutated
	// (e.g. set to nil) as that is racey, but is closed once when the next event
	// is published. the next pointer will have been set by the time this is
	// closed.
	ch chan struct{}
}

// newBufferItem returns a blank buffer item with a link and chan ready to have
// the fields set and be appended to a buffer.
func newBufferItem(events []Event) *bufferItem {
	return &bufferItem{
		link:   &bufferLink{ch: make(chan struct{})},
		Events: events,
	}
}

// Next return the next buffer item in the buffer. It may block until ctx is
// cancelled or until the next item is published.
func (i *bufferItem) Next(ctx context.Context, closed <-chan struct{}) (*bufferItem, error) {
	// See if there is already a next value, block if so. Note we don't rely on
	// state change (chan nil) as that's not threadsafe but detecting close is.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-closed:
		return nil, fmt.Errorf("subscription closed")
	case <-i.link.ch:
	}

	// If channel closed, there must be a next item to read
	nextRaw := i.link.next.Load()
	if nextRaw == nil {
		// shouldn't be possible
		return nil, errors.New("invalid next item")
	}
	next := nextRaw.(*bufferItem)
	if next.Err != nil {
		return nil, next.Err
	}
	return next, nil
}

// NextNoBlock returns true and the next item in the buffer without blocking.
// If there are no subsequent items it returns false and an empty item.
// The empty item will be ignored by subscribers, but has the same link as this
// bufferItem without the Events.
// When the link.ch of the empty item is closed, subscriptions are notified of
// the next item.
func (i *bufferItem) NextNoBlock() (*bufferItem, bool) {
	nextRaw := i.link.next.Load()
	if nextRaw == nil {
		// Return an empty item that can be followed to the next item published.
		return &bufferItem{link: i.link}, false
	}
	return nextRaw.(*bufferItem), true
}

// HasEventIndex returns true if index matches the Event.Index of this item. Returns
// false if there are no events stored in the item, or the index does not match.
func (i *bufferItem) HasEventIndex(index uint64) bool {
	return len(i.Events) > 0 && i.Events[0].Index == index
}
