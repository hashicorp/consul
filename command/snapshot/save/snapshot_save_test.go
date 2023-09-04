package save

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestSnapshotSaveCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSnapshotSaveCommand_Validation(t *testing.T) {
	t.Parallel()

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
		ui := cli.NewMockUi()
		c := New(ui)

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

func TestSnapshotSaveCommandWithAppendFileNameFlag(t *testing.T) {
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
		"-append-filename=version,dc,node,status",
		"-http-addr=" + a.HTTPAddr(),
		file,
	}

	stats := a.Stats()

	status := "follower"

	if stats["consul"]["leader"] == "true" {
		status = "leader"
	}

	newFilePath := filepath.Join(dir, "backup"+"-"+a.Config.Version+"-"+a.Config.Datacenter+
		"-"+a.Config.NodeName+"-"+status+".tgz")

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	fi, err := os.Stat(newFilePath)
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), os.FileMode(0600))

	f, err := os.Open(newFilePath)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer f.Close()

	if err := client.Snapshot().Restore(nil, f); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSnapshotSaveCommand(t *testing.T) {
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

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	fi, err := os.Stat(file)
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), os.FileMode(0600))

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer f.Close()

	if err := client.Snapshot().Restore(nil, f); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSnapshotSaveCommand_TruncatedStream(t *testing.T) {
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

	var fakeResult atomic.Value

	// Run a fake webserver to pretend to be the snapshot API.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/snapshot" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		raw := fakeResult.Load()
		if raw == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		data := raw.([]byte)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	// Wait until the server is actually listening.
	retry.Run(t, func(r *retry.R) {
		resp, err := srv.Client().Get(srv.URL + "/not-real")
		require.NoError(r, err)
		require.Equal(r, http.StatusNotFound, resp.StatusCode)
	})

	dir := testutil.TempDir(t, "snapshot")

	for _, removeBytes := range []int{200, 16, 8, 4, 2, 1} {
		t.Run(fmt.Sprintf("truncate %d bytes from end", removeBytes), func(t *testing.T) {
			// Lop off part of the end.
			data := inputData[0 : len(inputData)-removeBytes]

			fakeResult.Store(data)

			ui := cli.NewMockUi()
			c := New(ui)

			file := filepath.Join(dir, "backup.tgz")
			args := []string{
				"-http-addr=" + srv.Listener.Addr().String(), // point to the fake
				file,
			}

			code := c.Run(args)
			require.Equal(t, 1, code, "expected non-zero exit")

			output := ui.ErrorWriter.String()
			require.Contains(t, output, "Error verifying snapshot file")
			require.Contains(t, output, "EOF")

			// file should not have been created

			_, err := os.Stat(file)
			require.Error(t, err, "file is not supposed to exist")
			require.True(t, os.IsNotExist(err), "file is not supposed to exist")

			// also check that the unverified inputs are gone as well
			_, err = os.Stat(file + ".unverified")
			require.Error(t, err, "unverified file is not supposed to exist")
			require.True(t, os.IsNotExist(err), "unverified file is not supposed to exist")
		})
	}
}
