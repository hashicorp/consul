package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

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
	if *update {
		if dir := filepath.Dir(path); dir != "." {
			require.NoError(t, os.MkdirAll(dir, 0755))
		}
		err := ioutil.WriteFile(path, []byte(actual), 0644)
		require.NoError(t, err)
	}

	expected, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	return string(expected)
}
