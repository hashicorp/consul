package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvent_IsEndOfSnapshot(t *testing.T) {
	e := Event{Payload: endOfSnapshot{}}
	require.True(t, e.IsEndOfSnapshot())

	t.Run("not EndOfSnapshot", func(t *testing.T) {
		e := Event{Payload: newSnapshotToFollow{}}
		require.False(t, e.IsEndOfSnapshot())
	})
}

func newSimpleEvent(key string, index uint64) Event {
	return Event{Index: index, Payload: simplePayload{key: key}}
}

func TestPayloadEvents_FilterByKey(t *testing.T) {
	type testCase struct {
		name        string
		req         SubscribeRequest
		events      []Event
		expectEvent bool
		expected    *PayloadEvents
		expectedCap int
	}

	fn := func(t *testing.T, tc testCase) {
		events := make([]Event, 0, 5)
		events = append(events, tc.events...)

		pe := &PayloadEvents{Items: events}
		ok := pe.MatchesKey(tc.req.Key, tc.req.Namespace, tc.req.Partition)
		require.Equal(t, tc.expectEvent, ok)
		if !tc.expectEvent {
			return
		}

		require.Equal(t, tc.expected, pe)
		// test if there was a new array allocated or not
		require.Equal(t, tc.expectedCap, cap(pe.Items))
	}

	var testCases = []testCase{
		{
			name: "all events match, no key or namespace",
			req:  SubscribeRequest{Topic: testTopic},
			events: []Event{
				newSimpleEvent("One", 102),
				newSimpleEvent("Two", 102)},
			expectEvent: true,
			expected: newPayloadEvents(
				newSimpleEvent("One", 102),
				newSimpleEvent("Two", 102)),
			expectedCap: 5,
		},
		{
			name: "all events match, no namespace",
			req:  SubscribeRequest{Topic: testTopic, Key: "Same"},
			events: []Event{
				newSimpleEvent("Same", 103),
				newSimpleEvent("Same", 103)},
			expectEvent: true,
			expected: newPayloadEvents(
				newSimpleEvent("Same", 103),
				newSimpleEvent("Same", 103)),
			expectedCap: 5,
		},
		{
			name: "all events match, no key",
			req:  SubscribeRequest{Topic: testTopic, Namespace: "apps"},
			events: []Event{
				newNSEvent("Something", "apps"),
				newNSEvent("Other", "apps")},
			expectEvent: true,
			expected: newPayloadEvents(
				newNSEvent("Something", "apps"),
				newNSEvent("Other", "apps")),
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
			expected: newPayloadEvents(
				newSimpleEvent("Same", 104),
				newSimpleEvent("Same", 104)),
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
			expected: newPayloadEvents(
				newNSEvent("app1", "apps"),
				newNSEvent("app2", "apps")),
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

// TODO(partitions)
func newNSEvent(key, namespace string) Event {
	return Event{Index: 22, Payload: nsPayload{key: key, namespace: namespace}}
}

type nsPayload struct {
	framingEvent
	key       string
	namespace string
	partition string
	value     string
}

func (p nsPayload) MatchesKey(key, namespace, partition string) bool {
	return (key == "" || key == p.key) &&
		(namespace == "" || namespace == p.namespace) &&
		(partition == "" || partition == p.partition)
}

func TestPayloadEvents_HasReadPermission(t *testing.T) {
	t.Run("some events filtered", func(t *testing.T) {
		ep := newPayloadEvents(
			Event{Payload: simplePayload{key: "one", noReadPerm: true}},
			Event{Payload: simplePayload{key: "two", noReadPerm: false}},
			Event{Payload: simplePayload{key: "three", noReadPerm: true}},
			Event{Payload: simplePayload{key: "four", noReadPerm: false}})

		require.True(t, ep.HasReadPermission(nil))
		expected := []Event{
			{Payload: simplePayload{key: "two"}},
			{Payload: simplePayload{key: "four"}},
		}
		require.Equal(t, expected, ep.Items)
	})

	t.Run("all events filtered", func(t *testing.T) {
		ep := newPayloadEvents(
			Event{Payload: simplePayload{key: "one", noReadPerm: true}},
			Event{Payload: simplePayload{key: "two", noReadPerm: true}},
			Event{Payload: simplePayload{key: "three", noReadPerm: true}},
			Event{Payload: simplePayload{key: "four", noReadPerm: true}})

		require.False(t, ep.HasReadPermission(nil))
	})

}
