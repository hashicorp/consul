// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package watch

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	m := NewMap[string, testVal]()

	// Set without init is a no-op
	{
		m.Set("hello", "world")
		require.Equal(t, 0, m.Len())
	}

	// Getting from empty map
	{
		got, ok := m.Get("hello")
		require.False(t, ok)
		require.Empty(t, got)
	}

	var called bool
	cancelMock := func() {
		called = true
	}

	// InitWatch successful
	{
		m.InitWatch("hello", cancelMock)
		require.Equal(t, 1, m.Len())
	}

	// Get still returns false
	{
		got, ok := m.Get("hello")
		require.False(t, ok)
		require.Empty(t, got)
	}

	// Set successful
	{
		require.True(t, m.Set("hello", "world"))
		require.Equal(t, 1, m.Len())
	}

	// Get successful
	{
		got, ok := m.Get("hello")
		require.True(t, ok)
		require.Equal(t, "world", string(got))
	}

	// CancelWatch successful
	{
		m.CancelWatch("hello")
		require.Equal(t, 0, m.Len())
		require.True(t, called)
	}

	// Get no-op
	{
		got, ok := m.Get("hello")
		require.False(t, ok)
		require.Empty(t, got)
	}

	// Set no-op
	{
		require.False(t, m.Set("hello", "world"))
		require.Equal(t, 0, m.Len())
	}
}

func TestMap_UpdateWatch(t *testing.T) {
	// UpdateWatch on an empty map should behave like InitWatch:
	// the entry is created with Val == nil and Get returns (zero, false).
	t.Run("empty map falls through to InitWatch behavior", func(t *testing.T) {
		m := NewMap[string, testVal]()

		var firstCancelCalled bool
		m.UpdateWatch("hello", func() { firstCancelCalled = true })

		require.Equal(t, 1, m.Len())
		require.True(t, m.IsWatched("hello"))
		got, ok := m.Get("hello")
		require.False(t, ok, "Val must be nil until Set is called")
		require.Empty(t, got)
		require.False(t, firstCancelCalled, "new cancel must not be invoked")
	})

	// UpdateWatch on a key that already has a stored value must preserve
	// the value and swap in the new cancel function. The old cancel must
	// be invoked exactly once.
	t.Run("existing key preserves stored value and swaps cancel", func(t *testing.T) {
		m := NewMap[string, testVal]()

		var firstCancelCalls int
		m.InitWatch("hello", func() { firstCancelCalls++ })
		require.True(t, m.Set("hello", "world"))

		var secondCancelCalls int
		m.UpdateWatch("hello", func() { secondCancelCalls++ })

		require.Equal(t, 1, firstCancelCalls, "old cancel must be called exactly once")
		require.Equal(t, 0, secondCancelCalls, "new cancel must not be called yet")

		got, ok := m.Get("hello")
		require.True(t, ok, "stored value must survive UpdateWatch")
		require.Equal(t, testVal("world"), got)

		// CancelWatch must invoke the most recent cancel and clear the entry.
		m.CancelWatch("hello")
		require.Equal(t, 1, firstCancelCalls, "old cancel must not be re-invoked")
		require.Equal(t, 1, secondCancelCalls, "current cancel must be invoked on CancelWatch")
		require.Equal(t, 0, m.Len())
	})

	// Repeated UpdateWatch calls without an intervening Set must still
	// invoke every superseded cancel exactly once.
	t.Run("repeated UpdateWatch cancels each superseded cancel exactly once", func(t *testing.T) {
		m := NewMap[string, testVal]()

		var cancelCounts [3]int
		m.UpdateWatch("k", func() { cancelCounts[0]++ })
		m.UpdateWatch("k", func() { cancelCounts[1]++ })
		m.UpdateWatch("k", func() { cancelCounts[2]++ })

		require.Equal(t, 1, cancelCounts[0])
		require.Equal(t, 1, cancelCounts[1])
		require.Equal(t, 0, cancelCounts[2], "most recent cancel must still be live")

		m.CancelWatch("k")
		require.Equal(t, 1, cancelCounts[2])
	})

	// UpdateWatch must tolerate a nil cancel function, mirroring
	// InitWatch's contract.
	t.Run("nil cancel is allowed", func(t *testing.T) {
		m := NewMap[string, testVal]()
		require.NotPanics(t, func() { m.UpdateWatch("k", nil) })
		require.True(t, m.IsWatched("k"))
		require.NotPanics(t, func() { m.UpdateWatch("k", nil) })
		require.NotPanics(t, func() { m.CancelWatch("k") })
	})

	// Set followed by UpdateWatch followed by Set must update the stored
	// value to the latest value (sanity-check that the entry remains
	// writable after UpdateWatch).
	t.Run("Set still works after UpdateWatch", func(t *testing.T) {
		m := NewMap[string, testVal]()
		m.InitWatch("k", nil)
		require.True(t, m.Set("k", "v1"))

		m.UpdateWatch("k", nil)
		got, ok := m.Get("k")
		require.True(t, ok)
		require.Equal(t, testVal("v1"), got)

		require.True(t, m.Set("k", "v2"))
		got, ok = m.Get("k")
		require.True(t, ok)
		require.Equal(t, testVal("v2"), got)
	})
}

func TestMap_ForEach(t *testing.T) {
	m := NewMap[string, testVal]()
	inputs := map[string]testVal{
		"hello": "world",
		"foo":   "bar",
		"baz":   "bat",
	}
	for k, v := range inputs {
		m.InitWatch(k, nil)
		m.Set(k, v)
	}
	require.Equal(t, 3, m.Len())

	// returning true continues iteration
	{
		var count int
		m.ForEachKey(func(k string) bool {
			count++
			return true
		})
		require.Equal(t, 3, count)
	}

	// returning false exits loop
	{
		var count int
		m.ForEachKey(func(k string) bool {
			count++
			return false
		})
		require.Equal(t, 1, count)
	}
}

func TestMap_ForEachE(t *testing.T) {
	m := NewMap[string, testVal]()
	inputs := map[string]testVal{
		"hello": "world",
		"foo":   "bar",
		"baz":   "bat",
	}
	for k, v := range inputs {
		m.InitWatch(k, nil)
		m.Set(k, v)
	}
	require.Equal(t, 3, m.Len())

	// returning nil error continues iteration
	{
		var count int
		err := m.ForEachKeyE(func(k string) error {
			count++
			return nil
		})
		require.Equal(t, 3, count)
		require.Nil(t, err)
	}

	// returning an error should exit immediately
	{
		var count int
		err := m.ForEachKeyE(func(k string) error {
			count++
			return errors.New("boooo")
		})
		require.Equal(t, 1, count)
		require.Errorf(t, err, "boo")
	}
}

func TestMap_DeepCopy(t *testing.T) {
	orig := NewMap[string, testVal]()
	inputs := map[string]testVal{
		"hello": "world",
		"foo":   "bar",
		"baz":   "bat",
	}
	for k, v := range inputs {
		orig.InitWatch(k, nil)
		orig.Set(k, v)
	}
	require.Equal(t, 3, orig.Len())

	clone := orig.DeepCopy()
	require.Equal(t, 3, clone.Len())

	orig.CancelWatch("hello")
	require.NotEqual(t, orig.Len(), clone.Len())
}

type testVal string

func (tv testVal) DeepCopy() testVal { return tv }
