// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// Get reads the expected value from the file at filename and returns the value.
// filename is relative to the ./testdata directory.
//
// If the `-update` flag is used with `go test`, the golden file will be updated
// to the value of actual.
func Get(t *testing.T, actual, filename string) string {
	t.Helper()
	return string(GetBytes(t, []byte(actual), filename))
}

// GetBytes reads the expected value from the file at filename and returns the
// value as a byte array. filename is relative to the ./testdata directory.
//
// If the `-update` flag is used with `go test`, the golden file will be updated
// to the value of actual.
func GetBytes(t *testing.T, actual []byte, filename string) []byte {
	t.Helper()

	path := filepath.Join("testdata", filename)
	if *update {
		if dir := filepath.Dir(path); dir != "." {
			require.NoError(t, os.MkdirAll(dir, 0755))
		}
		err := os.WriteFile(path, actual, 0644)
		require.NoError(t, err)
	}

	return GetBytesAtFilePath(t, path)
}

// GetBytes reads the expected value from the file at filepath and returns the
// value as a byte array. filepath is relative to the ./testdata directory.
func GetBytesAtFilePath(t *testing.T, filepath string) []byte {
	t.Helper()

	expected, err := os.ReadFile(filepath)
	require.NoError(t, err)
	return expected
}
