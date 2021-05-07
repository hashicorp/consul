package main

import (
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/icmd"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e test too slow for -short")
	}

	sourcepkg := "./internal/e2e/sourcepkg"
	// Cleanup the generated file when the test ends. The source must be a
	// loadable Go package, so it can not be easily generated into a temporary
	// directory.
	output := "./internal/e2e/sourcepkg/node_gen.go"
	t.Cleanup(func() {
		os.Remove(output)
	})

	args := []string{"mog", "-source", sourcepkg}
	err := run(args)
	assert.NilError(t, err)

	// go vet the file to check that it is valid Go syntax
	icmd.RunCommand("go", "vet", sourcepkg).Assert(t, icmd.Success)

	actual, err := ioutil.ReadFile(output)
	assert.NilError(t, err)

	t.Logf("OUTPUT\n%s\n", string(actual))
	golden.Assert(t, string(actual), t.Name()+"-expected-node_gen.go")
}
