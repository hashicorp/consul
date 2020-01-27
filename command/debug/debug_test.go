package debug

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
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

	a := agent.NewTestAgent(t, t.Name(), `
	enable_debug = true
	`)
	a.Agent.LogWriter = logger.NewLogWriter(512)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-duration=100ms",
		"-interval=50ms",
	}

	code := cmd.Run(args)

	if code != 0 {
		t.Errorf("should exit 0, got code: %d", code)
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

	a := agent.NewTestAgent(t, t.Name(), `
	enable_debug = true
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-capture=agent",
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("should exit 0, got code: %d", code)
	}

	archivePath := fmt.Sprintf("%s%s", outputPath, debugArchiveExtension)
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %s", err)
	}
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("failed to read gzip archive: %s", err)
	}
	tr := tar.NewReader(gz)

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
		if h.Name != "debug/agent.json" && h.Name != "debug/index.json" {
			t.Fatalf("archive contents do not match: %s", h.Name)
		}
	}

}

func TestDebugCommand_ArgsBad(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	ui := cli.NewMockUi()
	cmd := New(ui, nil)

	args := []string{
		"foo",
		"bad",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should exit non-zero, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "Too many arguments") {
		t.Errorf("expected error output, got %q", errOutput)
	}
}

func TestDebugCommand_OutputPathBad(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

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

	a := agent.NewTestAgent(t, t.Name(), "")
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

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
			[]string{"host.json", "cluster.json"},
		},
		"static": {
			[]string{"agent", "host", "cluster"},
			[]string{"agent.json", "host.json", "cluster.json"},
			[]string{"*/metrics.json"},
		},
		"metrics-only": {
			[]string{"metrics"},
			[]string{"*/metrics.json"},
			[]string{"agent.json", "host.json", "cluster.json"},
		},
		"all-but-pprof": {
			[]string{
				"metrics",
				"logs",
				"host",
				"agent",
				"cluster",
			},
			[]string{
				"host.json",
				"agent.json",
				"cluster.json",
				"*/metrics.json",
				"*/consul.log",
			},
			[]string{},
		},
	}

	for name, tc := range cases {
		testDir := testutil.TempDir(t, "debug")
		defer os.RemoveAll(testDir)

		a := agent.NewTestAgent(t, t.Name(), `
		enable_debug = true
		`)
		a.Agent.LogWriter = logger.NewLogWriter(512)

		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		ui := cli.NewMockUi()
		cmd := New(ui, nil)
		cmd.validateTiming = false

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
				t.Fatalf("%s: output data should exist for %s", name, f)
			}
		}

		// Ensure any excluded files do not exist
		for _, f := range tc.excludedFiles {
			path := fmt.Sprintf("%s/%s", outputPath, f)
			// Glob ignores file system errors
			fs, _ := filepath.Glob(path)
			if len(fs) > 0 {
				t.Fatalf("%s: output data should not exist for %s", name, f)
			}
		}
	}
}

func TestDebugCommand_ProfilesExist(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	enable_debug = true
	`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

	outputPath := fmt.Sprintf("%s/debug", testDir)
	println(outputPath)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		// CPU profile has a minimum of 1s
		"-archive=false",
		"-duration=1s",
		"-interval=1s",
		"-capture=pprof",
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("should exit 0, got code: %d", code)
	}

	profiles := []string{"heap.prof", "profile.prof", "goroutine.prof", "trace.out"}
	// Glob ignores file system errors
	for _, v := range profiles {
		fs, _ := filepath.Glob(fmt.Sprintf("%s/*/%s", outputPath, v))
		if len(fs) == 0 {
			t.Errorf("output data should exist for %s", v)
		}
	}

	errOutput := ui.ErrorWriter.String()
	if errOutput != "" {
		t.Errorf("expected no error output, got %s", errOutput)
	}
}

func TestDebugCommand_ValidateTiming(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		duration string
		interval string
		output   string
		code     int
	}{
		"both": {
			"20ms",
			"10ms",
			"duration must be longer",
			1,
		},
		"short interval": {
			"10s",
			"10ms",
			"interval must be longer",
			1,
		},
		"lower duration": {
			"20s",
			"30s",
			"must be longer than interval",
			1,
		},
	}

	for name, tc := range cases {
		// Because we're only testng validation, we want to shut down
		// the valid duration test to avoid hanging
		shutdownCh := make(chan struct{})

		testDir := testutil.TempDir(t, "debug")
		defer os.RemoveAll(testDir)

		a := agent.NewTestAgent(t, t.Name(), "")
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		ui := cli.NewMockUi()
		cmd := New(ui, shutdownCh)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-duration=" + tc.duration,
			"-interval=" + tc.interval,
			"-capture=agent",
		}
		code := cmd.Run(args)

		if code != tc.code {
			t.Errorf("%s: should exit %d, got code: %d", name, tc.code, code)
		}

		errOutput := ui.ErrorWriter.String()
		if !strings.Contains(errOutput, tc.output) {
			t.Errorf("%s: expected error output '%s', got '%q'", name, tc.output, errOutput)
		}
	}
}

func TestDebugCommand_DebugDisabled(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "debug")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	enable_debug = false
	`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui, nil)
	cmd.validateTiming = false

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-archive=false",
		// CPU profile has a minimum of 1s
		"-duration=1s",
		"-interval=1s",
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("should exit 0, got code: %d", code)
	}

	profiles := []string{"heap.prof", "profile.prof", "goroutine.prof", "trace.out"}
	// Glob ignores file system errors
	for _, v := range profiles {
		fs, _ := filepath.Glob(fmt.Sprintf("%s/*/%s", outputPath, v))
		if len(fs) > 0 {
			t.Errorf("output data should not exist for %s", v)
		}
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "Unable to capture pprof") {
		t.Errorf("expected warn output, got %s", errOutput)
	}
}
