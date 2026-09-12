// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package pipebootstrap

import (
	"os"
	"testing"

	"github.com/mitchellh/cli"
)

// countOpenFDs returns the number of file descriptors currently open by this
// process, read from /proc/self/fd. Linux only, hence this file's _linux
// suffix.
func countOpenFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("reading /proc/self/fd: %v", err)
	}
	return len(entries)
}

// TestRun_WriteErrorDoesNotLeakDescriptor verifies that Run closes the named
// pipe descriptor when writing the bootstrap payload fails. /dev/full can be
// opened for writing but fails every write with ENOSPC, which exercises the
// buf.WriteTo error path. Before the fix for issue #23256, the descriptor
// opened by os.OpenFile was leaked on that path.
func TestRun_WriteErrorDoesNotLeakDescriptor(t *testing.T) {
	// Feed non-empty stdin so buf.WriteTo actually attempts a write.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinW.WriteString("bootstrap-payload"); err != nil {
		t.Fatal(err)
	}
	_ = stdinW.Close()

	origStdin := os.Stdin
	os.Stdin = stdinR
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = stdinR.Close()
	})

	// Baseline is taken after stdinR replaces os.Stdin, so its descriptor is
	// already accounted for in the count.
	before := countOpenFDs(t)

	code := New(cli.NewMockUi()).Run([]string{"/dev/full"})
	if code != 1 {
		t.Fatalf("Run: expected exit code 1 on write failure, got %d", code)
	}

	if after := countOpenFDs(t); after > before {
		t.Fatalf("file descriptor leaked on write-error path: %d open before Run, %d after", before, after)
	}
}
