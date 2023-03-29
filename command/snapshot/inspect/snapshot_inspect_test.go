// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inspect

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, got string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	if *update && got != "" {
		err := os.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := os.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func TestSnapshotInspectCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSnapshotInspectCommand_Validation(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"no file": {
			[]string{},
			"Missing FILE argument",
		},
		"extra args": {
			[]string{"foo", "bar", "baz"},
			"Too many arguments",
		},
	}

	for name, tc := range cases {
		// Ensure our buffer is always clear
		if ui.ErrorWriter != nil {
			ui.ErrorWriter.Reset()
		}
		if ui.OutputWriter != nil {
			ui.OutputWriter.Reset()
		}

		code := c.Run(tc.args)
		if code == 0 {
			t.Errorf("%s: expected non-zero exit", name)
		}

		output := ui.ErrorWriter.String()
		if !strings.Contains(output, tc.output) {
			t.Errorf("%s: expected %q to contain %q", name, output, tc.output)
		}
	}
}

func TestSnapshotInspectCommand(t *testing.T) {

	filepath := "./testdata/backup.snap"

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{filepath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	want := golden(t, t.Name(), ui.OutputWriter.String())
	require.Equal(t, want, ui.OutputWriter.String())
}

func TestSnapshotInspectKVDetailsCommand(t *testing.T) {

	filepath := "./testdata/backupWithKV.snap"

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-kvdetails", filepath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	want := golden(t, t.Name(), ui.OutputWriter.String())
	require.Equal(t, want, ui.OutputWriter.String())
}

func TestSnapshotInspectKVDetailsDepthCommand(t *testing.T) {

	filepath := "./testdata/backupWithKV.snap"

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-kvdetails", "-kvdepth", "3", filepath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	want := golden(t, t.Name(), ui.OutputWriter.String())
	require.Equal(t, want, ui.OutputWriter.String())
}

func TestSnapshotInspectKVDetailsDepthFilterCommand(t *testing.T) {

	filepath := "./testdata/backupWithKV.snap"

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-kvdetails", "-kvdepth", "3", "-kvfilter", "vault/logical", filepath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	want := golden(t, t.Name(), ui.OutputWriter.String())
	require.Equal(t, want, ui.OutputWriter.String())
}

// TestSnapshotInspectCommandRaw test reading a snaphost directly from a raft
// data dir.
func TestSnapshotInspectCommandRaw(t *testing.T) {
	filepath := "./testdata/raw/state.bin"

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{filepath}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	want := golden(t, t.Name(), ui.OutputWriter.String())
	require.Equal(t, want, ui.OutputWriter.String())
}

func TestSnapshotInspectInvalidFile(t *testing.T) {
	// Attempt to open a non-snapshot file.
	filepath := "./testdata/TestSnapshotInspectCommand.golden"

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{filepath}

	code := c.Run(args)
	// Just check it was an error code returned and not a panic - originally this
	// would panic.
	if code == 0 {
		t.Fatalf("should return an error code")
	}
}
