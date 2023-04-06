// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// This doesn't really test the "atomic" part of this function. It really
// tests that it just writes the file properly. I would love to test this
// better but I'm not sure how. -mitchellh
func TestWriteAtomic(t *testing.T) {
	td, err := os.MkdirTemp("", "lib-file")
	require.NoError(t, err)
	defer os.RemoveAll(td)

	// Create a subdir that doesn't exist to test that it is created
	path := filepath.Join(td, "subdir", "file")

	// Write
	expected := []byte("hello")
	require.NoError(t, WriteAtomic(path, expected))

	// Read and verify
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
