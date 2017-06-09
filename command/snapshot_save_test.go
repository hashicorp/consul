package command

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

func testSnapshotSaveCommand(t *testing.T) (*cli.MockUi, *SnapshotSaveCommand) {
	ui := cli.NewMockUi()
	return ui, &SnapshotSaveCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetHTTP,
		},
	}
}

func TestSnapshotSaveCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SnapshotSaveCommand{}
}

func TestSnapshotSaveCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(SnapshotSaveCommand))
}

func TestSnapshotSaveCommand_Validation(t *testing.T) {
	t.Parallel()
	ui, c := testSnapshotSaveCommand(t)

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

func TestSnapshotSaveCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testSnapshotSaveCommand(t)

	dir := testutil.TempDir(t, "snapshot")
	defer os.RemoveAll(dir)

	file := path.Join(dir, "backup.tgz")
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		file,
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer f.Close()

	if err := client.Snapshot().Restore(nil, f); err != nil {
		t.Fatalf("err: %v", err)
	}
}
