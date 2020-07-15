package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvent_IsEndOfSnapshot(t *testing.T) {
	e := Event{Payload: endOfSnapshot{}}
	require.True(t, e.IsEndOfSnapshot())

	t.Run("not EndOfSnapshot", func(t *testing.T) {
		e := Event{Payload: endOfEmptySnapshot{}}
		require.False(t, e.IsEndOfSnapshot())
	})
}
