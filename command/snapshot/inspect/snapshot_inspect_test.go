package inspect

import (
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

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
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	dir := testutil.TempDir(t, "snapshot")
	defer os.RemoveAll(dir)

	file := path.Join(dir, "backup.tgz")

	// Save a snapshot of the current Consul state
	f, err := os.Create(file)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	snap, _, err := client.Snapshot().Save(nil)
	if err != nil {
		f.Close()
		t.Fatalf("err: %v", err)
	}
	if _, err := io.Copy(f, snap); err != nil {
		f.Close()
		t.Fatalf("err: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Inspect the snapshot
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{file}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for _, key := range []string{
		"ID",
		"Size",
		"Index",
		"Term",
		"Version",
	} {
		if !strings.Contains(output, key) {
			t.Fatalf("bad %#v, missing %q", output, key)
		}
	}
}
