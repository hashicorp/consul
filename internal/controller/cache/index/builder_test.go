// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilderRaw(t *testing.T) {
	var b Builder
	b.Raw([]byte{1, 2, 3})

	require.Equal(t, []byte{1, 2, 3}, b.Bytes())
}

func TestBuilderString(t *testing.T) {
	var b Builder
	b.String("abc")

	// Ensure that the null terminator is tacked on
	require.Equal(t, []byte{'a', 'b', 'c', 0}, b.Bytes())
}

func TestBuilderWrite(t *testing.T) {
	var b Builder
	wrote, err := b.Write([]byte{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, 3, wrote)
	require.Equal(t, []byte{1, 2, 3, 0}, b.Bytes())
}
