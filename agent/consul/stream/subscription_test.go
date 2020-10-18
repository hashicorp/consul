package stream

import (
	"context"
	"testing"
	time "time"

	"github.com/stretchr/testify/require"
)

func noopUnSub() {}

func TestSubscription(t *testing.T) {
	eb := newEventBuffer()

	index := uint64(100)

	startHead := eb.Head()

	// Start with an event in the buffer
	publishTestEvent(index, eb, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := SubscribeRequest{
		Topic: testTopic,
		Key:   "test",
	}
	sub := newSubscription(req, startHead, noopUnSub)

	// First call to sub.Next should return our published event immediately
	start := time.Now()
	got, err := sub.Next(ctx)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)

	// Schedule an event publish in a while
	index++
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		publishTestEvent(index, eb, "test")
	})

	// Next call should block until event is delivered
	got, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Event should have been delivered after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Event should have been delivered after short time, took %s", elapsed)
	require.Equal(t, index, got.Index)

	// Event with wrong key should not be delivered. Deliver a good message right
	// so we don't have to block test thread forever or cancel func yet.
	index++
	publishTestEvent(index, eb, "nope")
	index++
	publishTestEvent(index, eb, "test")

	start = time.Now()
	got, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)
	require.Equal(t, "test", got.Key)

	// Cancelling the subscription context should unblock Next
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		cancel()
	})

	_, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.Error(t, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Event should have been delivered after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Event should have been delivered after short time, took %s", elapsed)
}

func TestSubscription_Close(t *testing.T) {
	eb := newEventBuffer()
	index := uint64(100)
	startHead := eb.Head()

	// Start with an event in the buffer
	publishTestEvent(index, eb, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := SubscribeRequest{
		Topic: testTopic,
		Key:   "test",
	}
	sub := newSubscription(req, startHead, noopUnSub)

	// First call to sub.Next should return our published event immediately
	start := time.Now()
	got, err := sub.Next(ctx)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.True(t, elapsed < 200*time.Millisecond,
		"Event should have been delivered immediately, took %s", elapsed)
	require.Equal(t, index, got.Index)

	// Schedule a Close simulating the server deciding this subscroption
	// needs to reset (e.g. on ACL perm change).
	start = time.Now()
	time.AfterFunc(200*time.Millisecond, func() {
		sub.forceClose()
	})

	_, err = sub.Next(ctx)
	elapsed = time.Since(start)
	require.Error(t, err)
	require.Equal(t, ErrSubscriptionClosed, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Reload should have happened after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Reload should have been delivered after short time, took %s", elapsed)
}

func publishTestEvent(index uint64, b *eventBuffer, key string) {
	// Don't care about the event payload for now just the semantics of publishing
	// something. This is not a valid stream in the end-to-end streaming protocol
	// but enough to test subscription mechanics.
	e := Event{
		Index: index,
		Topic: testTopic,
		Key:   key,
	}
	b.Append([]Event{e})
}

func TestFilter_NoKey(t *testing.T) {
	events := make([]Event, 0, 5)
	events = append(events, Event{Key: "One", Index: 102}, Event{Key: "Two"})

	req := SubscribeRequest{Topic: testTopic}
	actual, ok := filterByKey(req, events)
	require.True(t, ok)
	require.Equal(t, Event{Topic: testTopic, Index: 102, Payload: events}, actual)

	// test that a new array was not allocated
	require.Equal(t, cap(actual.Payload.([]Event)), 5)
}

func TestFilter_WithKey_AllEventsMatch(t *testing.T) {
	events := make([]Event, 0, 5)
	events = append(events, Event{Key: "Same", Index: 103}, Event{Key: "Same"})

	req := SubscribeRequest{Topic: testTopic, Key: "Same"}
	actual, ok := filterByKey(req, events)
	require.True(t, ok)
	expected := Event{Topic: testTopic, Index: 103, Key: "Same", Payload: events}
	require.Equal(t, expected, actual)

	// test that a new array was not allocated
	require.Equal(t, 5, cap(actual.Payload.([]Event)))
}

func TestFilter_WithKey_SomeEventsMatch(t *testing.T) {
	events := make([]Event, 0, 5)
	events = append(events, Event{Key: "Same", Index: 104}, Event{Key: "Other"}, Event{Key: "Same"})

	req := SubscribeRequest{Topic: testTopic, Key: "Same"}
	actual, ok := filterByKey(req, events)
	require.True(t, ok)
	expected := Event{
		Topic:   testTopic,
		Index:   104,
		Key:     "Same",
		Payload: []Event{{Key: "Same", Index: 104}, {Key: "Same"}},
	}
	require.Equal(t, expected, actual)

	// test that a new array was allocated with the correct size
	require.Equal(t, cap(actual.Payload.([]Event)), 2)
}

func TestFilter_WithKey_NoEventsMatch(t *testing.T) {
	events := make([]Event, 0, 5)
	events = append(events, Event{Key: "Same"}, Event{Key: "Same"})

	req := SubscribeRequest{Topic: testTopic, Key: "Other"}
	_, ok := filterByKey(req, events)
	require.False(t, ok)
}
