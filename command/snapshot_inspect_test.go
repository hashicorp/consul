package command

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestSnapshotInspectCommand_implements(t *testing.T) {
	var _ cli.Command = &SnapshotInspectCommand{}
}

func TestSnapshotInspectCommand_noTabs(t *testing.T) {
	assertNoTabs(t, new(SnapshotInspectCommand))
}

func TestSnapshotInspectCommand_Validation(t *testing.T) {
	ui := new(cli.MockUi)
	c := &SnapshotInspectCommand{Ui: ui}

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

func TestSnapshotInspectCommand_Run(t *testing.T) {
	srv, client := testAgentWithAPIClient(t)
	defer srv.Shutdown()
	waitForLeader(t, srv.httpAddr)

	ui := new(cli.MockUi)

	dir, err := ioutil.TempDir("", "snapshot")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
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
	inspect := &SnapshotInspectCommand{Ui: ui}
	args := []string{file}

	code := inspect.Run(args)
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
