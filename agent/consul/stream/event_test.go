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
