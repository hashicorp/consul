package watch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMap(t *testing.T) {
	m := NewMap[string, string]()

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
		require.Equal(t, "world", got)
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

func TestMap_ForEach(t *testing.T) {
	type testType struct {
		s string
	}

	m := NewMap[string, any]()
	inputs := map[string]any{
		"hello": 13,
		"foo":   struct{}{},
		"bar":   &testType{s: "wow"},
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
