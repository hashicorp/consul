package debug

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

func TestDebugCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi(), nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestDebugCommand(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t.Name(), `
	enable_debug = true
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-duration=100ms",
		"-interval=50ms",
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("should exit 0, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if errOutput != "" {
		t.Errorf("expected no error output, got %q", errOutput)
	}
}

func TestDebugCommand_Archive(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t.Name(), `
	enable_debug = true
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-capture=agent",
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("should exit 0, got code: %d", code)
	}

	archivePath := fmt.Sprintf("%s.tar.gz", outputPath)
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %s", err)
	}
	tr := tar.NewReader(file)

	for {
		h, err := tr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read file in archive: %s", err)
		}

		// ignore the outer directory
		if h.Name == "debug" {
			continue
		}

		// should only contain this one capture target
		if h.Name != "debug/agent.json" {
			t.Fatalf("archive contents do not match: %s", h.Name)
		}
	}

}

func TestDebugCommand_OutputPathBad(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)

	outputPath := ""
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-duration=100ms",
		"-interval=50ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should exit non-zero, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "no such file or directory") {
		t.Errorf("expected error output, got %q", errOutput)
	}
}

func TestDebugCommand_OutputPathExists(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-duration=100ms",
		"-interval=50ms",
	}

	// Make a directory that conflicts with the output path
	err := os.Mkdir(outputPath, 0755)
	if err != nil {
		t.Fatalf("duplicate test directory creation failed: %s", err)
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should exit non-zero, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "directory already exists") {
		t.Errorf("expected error output, got %q", errOutput)
	}
}

func TestDebugCommand_CaptureTargets(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		// used in -target param
		targets []string
		// existence verified after execution
		files []string
		// non-existence verified after execution
		excludedFiles []string
	}{
		"single": {
			[]string{"agent"},
			[]string{"agent.json"},
			[]string{"host.json", "members.json"},
		},
		"static": {
			[]string{"agent", "host", "cluster"},
			[]string{"agent.json", "host.json", "members.json"},
			[]string{"*/metrics.json"},
		},
		"metrics-only": {
			[]string{"metrics"},
			[]string{"*/metrics.json"},
			[]string{"agent.json", "host.json", "members.json"},
		},
		"all": {
			[]string{
				"metrics",
				"pprof",
				"logs",
				"host",
				"agent",
				"cluster",
			},
			[]string{
				"host.json",
				"agent.json",
				"members.json",
				"*/metrics.json",
				"*/consul.log",
			},
			[]string{},
		},
	}

	for name, tc := range cases {
		testDir := testutil.TempDir(t, "debug")
		defer os.RemoveAll(testDir)

		a := agent.NewTestAgent(t.Name(), `
		enable_debug = true
		`)
		a.Agent.LogWriter = logger.NewLogWriter(512)

		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		ui := cli.NewMockUi()
		cmd := New(ui, nil)

		outputPath := fmt.Sprintf("%s/debug-%s", testDir, name)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-output=" + outputPath,
			"-archive=false",
			"-duration=100ms",
			"-interval=50ms",
		}
		for _, t := range tc.targets {
			args = append(args, "-capture="+t)
		}

		if code := cmd.Run(args); code != 0 {
			t.Fatalf("should exit 0, got code: %d", code)
		}

		errOutput := ui.ErrorWriter.String()
		if errOutput != "" {
			t.Errorf("expected no error output, got %q", errOutput)
		}

		// Ensure the debug data was written
		_, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("output path should exist: %s", err)
		}

		// Ensure the captured static files exist
		for _, f := range tc.files {
			path := fmt.Sprintf("%s/%s", outputPath, f)
			// Glob ignores file system errors
			fs, _ := filepath.Glob(path)
			if len(fs) <= 0 {
				t.Fatalf("%s: output data should exist for %s: %v", name, f, fs)
			}
		}

		// Ensure any excluded files do not exist
		for _, f := range tc.excludedFiles {
			path := fmt.Sprintf("%s/%s", outputPath, f)
			// Glob ignores file system errors
			fs, _ := filepath.Glob(path)
			if len(fs) > 0 {
				t.Fatalf("%s: output data should not exist for %s: %v", name, f, fs)
			}
		}
	}
}
