// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/hashicorp/consul/version/versiontest"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSnapshotDecodeCommand_noTabs(t *testing.T) {
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSnapshotDecodeCommand_Validation(t *testing.T) {
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

func TestSnapshotDecodeCommand(t *testing.T) {
	cases := map[string]string{
		"no-kv":   "./testdata/backup.snap",
		"with-kv": "./testdata/backupWithKV.snap",
		"all":     "./testdata/all.snap",
	}

	for name, fpath := range cases {
		fpath := fpath
		t.Run(name, func(t *testing.T) {
			// Inspect the snapshot
			ui := cli.NewMockUi()
			c := New(ui)
			args := []string{fpath}

			code := c.Run(args)
			if code != 0 {
				t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
			}

			actual := ui.OutputWriter.String()
			fname := t.Name() + ".ce.golden"
			if versiontest.IsEnterprise() {
				fname = t.Name() + ".ent.golden"
			}
			want := golden.Get(t, actual, fname)
			require.Equal(t, want, actual)
		})
	}
}

func TestSnapshotDecodeInvalidFile(t *testing.T) {
	// Attempt to open a non-snapshot file.
	filepath := "./testdata/TestSnapshotDecodeCommand/no-kv.golden"

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
