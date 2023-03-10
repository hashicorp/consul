package restore

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSnapshotRestoreCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSnapshotRestoreCommand_Validation(t *testing.T) {
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

func TestSnapshotRestoreCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	dir := testutil.TempDir(t, "snapshot")
	file := filepath.Join(dir, "backup.tgz")
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		file,
	}

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

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestSnapshotRestoreCommand_TruncatedSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Seed it with 64K of random data just so we have something to work with.
	{
		blob := make([]byte, 64*1024)
		_, err := rand.Read(blob)
		require.NoError(t, err)

		_, err = client.KV().Put(&api.KVPair{Key: "blob", Value: blob}, nil)
		require.NoError(t, err)
	}

	// Do a manual snapshot so we can send back roughly reasonable data.
	var inputData []byte
	{
		rc, _, err := client.Snapshot().Save(nil)
		require.NoError(t, err)
		defer rc.Close()

		inputData, err = ioutil.ReadAll(rc)
		require.NoError(t, err)
	}

	dir := testutil.TempDir(t, "snapshot")

	for _, removeBytes := range []int{200, 16, 8, 4, 2, 1} {
		t.Run(fmt.Sprintf("truncate %d bytes from end", removeBytes), func(t *testing.T) {
			// Lop off part of the end.
			data := inputData[0 : len(inputData)-removeBytes]

			ui := cli.NewMockUi()
			c := New(ui)

			file := filepath.Join(dir, "backup.tgz")
			require.NoError(t, ioutil.WriteFile(file, data, 0644))
			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				file,
			}

			code := c.Run(args)
			require.Equal(t, 1, code, "expected non-zero exit")

			output := ui.ErrorWriter.String()
			require.Contains(t, output, "Error restoring snapshot")
			require.Contains(t, output, "EOF")
		})
	}
}
