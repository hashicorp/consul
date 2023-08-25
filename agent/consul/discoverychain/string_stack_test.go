// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package discoverychain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringStack(t *testing.T) {
	var (
		v  string
		ok bool
	)

	var ss stringStack
	require.Equal(t, 0, ss.Len())
	v, ok = ss.Peek()
	require.Empty(t, v)
	require.False(t, ok)

	ss.Push("foo")
	require.Equal(t, 1, ss.Len())

	v, ok = ss.Peek()
	require.Equal(t, "foo", v)
	require.True(t, ok)
	require.Equal(t, 1, ss.Len())

	v, ok = ss.Pop()
	require.Equal(t, "foo", v)
	require.True(t, ok)
	require.Equal(t, 0, ss.Len())

	ss.Push("foo")
	ss.Push("bar")
	ss.Push("baz")
	require.Equal(t, 3, ss.Len())

	v, ok = ss.Peek()
	require.Equal(t, "baz", v)
	require.True(t, ok)
	require.Equal(t, 3, ss.Len())

	v, ok = ss.Pop()
	require.Equal(t, "baz", v)
	require.True(t, ok)
	require.Equal(t, 2, ss.Len())

	v, ok = ss.Pop()
	require.Equal(t, "bar", v)
	require.True(t, ok)
	require.Equal(t, 1, ss.Len())

	v, ok = ss.Pop()
	require.Equal(t, "foo", v)
	require.True(t, ok)
	require.Equal(t, 0, ss.Len())
}
