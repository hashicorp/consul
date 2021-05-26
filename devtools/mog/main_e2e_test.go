package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

var (
	shouldVet   = flag.Bool("vet-gen", true, "should we vet the generated code")
	shouldPrint = flag.Bool("print-gen", false, "should we print the generated code")
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

	if *shouldVet {
		// go vet the file to check that it is valid Go syntax
		icmd.RunCommand("go", "vet", sourcepkg).Assert(t, icmd.Success)
	}

	actual, err := ioutil.ReadFile(output)
	assert.NilError(t, err)

	if *shouldPrint {
		t.Logf("OUTPUT\n%s\n", PrependLineNumbers(string(actual)))
	}
	golden.Assert(t, string(actual), t.Name()+"-expected-node_gen.go")
}

// PrependLineNumbers prepends line numbers onto the text passed in. In the
// event of some parsing error it just returns the original input, unprefixed.
func PrependLineNumbers(s string) string {
	scan := bufio.NewScanner(strings.NewReader(s))

	var (
		next  = 1
		lines []string
	)
	for scan.Scan() {
		lines = append(lines, fmt.Sprintf("%4d: %s", next, scan.Text()))
		next++
	}
	if scan.Err() != nil {
		return s
	}

	return strings.Join(lines, "\n")
}
