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
	require.Equal(t, "test", got.Payload.(simplePayload).key)

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
	require.Equal(t, ErrSubForceClosed, err)
	require.True(t, elapsed > 200*time.Millisecond,
		"Reload should have happened after blocking 200ms, took %s", elapsed)
	require.True(t, elapsed < 2*time.Second,
		"Reload should have been delivered after short time, took %s", elapsed)
}

func publishTestEvent(index uint64, b *eventBuffer, key string) {
	e := Event{
		Index:   index,
		Topic:   testTopic,
		Payload: simplePayload{key: key},
	}
	b.Append([]Event{e})
}

func newSimpleEvent(key string, index uint64) Event {
	return Event{Index: index, Payload: simplePayload{key: key}}
}

func TestFilterByKey(t *testing.T) {
	type testCase struct {
		name        string
		req         SubscribeRequest
		events      []Event
		expectEvent bool
		expected    Event
		expectedCap int
	}

	fn := func(t *testing.T, tc testCase) {
		events := make(PayloadEvents, 0, 5)
		events = append(events, tc.events...)

		actual, ok := filterByKey(tc.req, events)
		require.Equal(t, tc.expectEvent, ok)
		if !tc.expectEvent {
			return
		}

		require.Equal(t, tc.expected, actual)
		// test if there was a new array allocated or not
		require.Equal(t, tc.expectedCap, cap(actual.Payload.(PayloadEvents)))
	}

	var testCases = []testCase{
		{
			name: "all events match, no key or namespace",
			req:  SubscribeRequest{Topic: testTopic},
			events: []Event{
				newSimpleEvent("One", 102),
				newSimpleEvent("Two", 102)},
			expectEvent: true,
			expected: Event{
				Topic: testTopic,
				Index: 102,
				Payload: PayloadEvents{
					newSimpleEvent("One", 102),
					newSimpleEvent("Two", 102)}},
			expectedCap: 5,
		},
		{
			name: "all events match, no namespace",
			req:  SubscribeRequest{Topic: testTopic, Key: "Same"},
			events: []Event{
				newSimpleEvent("Same", 103),
				newSimpleEvent("Same", 103)},
			expectEvent: true,
			expected: Event{
				Topic: testTopic,
				Index: 103,
				Payload: PayloadEvents{
					newSimpleEvent("Same", 103),
					newSimpleEvent("Same", 103)}},
			expectedCap: 5,
		},
		{
			name: "all events match, no key",
			req:  SubscribeRequest{Topic: testTopic, Namespace: "apps"},
			events: []Event{
				newNSEvent("Something", "apps"),
				newNSEvent("Other", "apps")},
			expectEvent: true,
			expected: Event{
				Topic: testTopic,
				Index: 22,
				Payload: PayloadEvents{
					newNSEvent("Something", "apps"),
					newNSEvent("Other", "apps")}},
			expectedCap: 5,
		},
		{
			name: "some evens match, no namespace",
			req:  SubscribeRequest{Topic: testTopic, Key: "Same"},
			events: []Event{
				newSimpleEvent("Same", 104),
				newSimpleEvent("Other", 104),
				newSimpleEvent("Same", 104)},
			expectEvent: true,
			expected: Event{
				Topic: testTopic,
				Index: 104,
				Payload: PayloadEvents{
					newSimpleEvent("Same", 104),
					newSimpleEvent("Same", 104)}},
			expectedCap: 2,
		},
		{
			name: "some events match, no key",
			req:  SubscribeRequest{Topic: testTopic, Namespace: "apps"},
			events: []Event{
				newNSEvent("app1", "apps"),
				newNSEvent("db1", "dbs"),
				newNSEvent("app2", "apps")},
			expectEvent: true,
			expected: Event{
				Topic: testTopic,
				Index: 22,
				Payload: PayloadEvents{
					newNSEvent("app1", "apps"),
					newNSEvent("app2", "apps")}},
			expectedCap: 2,
		},
		{
			name: "no events match key",
			req:  SubscribeRequest{Topic: testTopic, Key: "Other"},
			events: []Event{
				newSimpleEvent("Same", 0),
				newSimpleEvent("Same", 0)},
		},
		{
			name: "no events match namespace",
			req:  SubscribeRequest{Topic: testTopic, Namespace: "apps"},
			events: []Event{
				newNSEvent("app1", "group1"),
				newNSEvent("app2", "group2")},
			expectEvent: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func newNSEvent(key, namespace string) Event {
	return Event{Index: 22, Payload: nsPayload{key: key, namespace: namespace}}
}

type nsPayload struct {
	key       string
	namespace string
	value     string
}

func (p nsPayload) FilterByKey(key, namespace string) bool {
	return (key == "" || key == p.key) && (namespace == "" || namespace == p.namespace)
}
