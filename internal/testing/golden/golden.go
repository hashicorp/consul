// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

	path := filepath.Join("testdata", filename)
	if *update {
		if dir := filepath.Dir(path); dir != "." {
			require.NoError(t, os.MkdirAll(dir, 0755))
		}
		err := os.WriteFile(path, []byte(actual), 0644)
		require.NoError(t, err)
	}

	expected, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(expected)
}
