// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2E(t *testing.T) {
	// Generate new output
	*flagPath = "./e2e/source.pb.go"
	require.NoError(t, run(*flagPath))

	raw, err := os.ReadFile("./e2e/source.rpcglue.pb.go")
	require.NoError(t, err)

	got := string(raw)

	golden(t, got, "./e2e/source.rpcglue.pb.go")
}

// golden reads the expected value from the file at path and returns the
// value.
//
// If the `-update` flag is used with `go test`, the golden file will be
// updated to the value of actual.
func golden(t *testing.T, actual, path string) string {
	t.Helper()

	path += ".golden"
	if golden.ShouldUpdate {
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
