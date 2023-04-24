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
