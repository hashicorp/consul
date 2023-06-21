// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package debug

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestDebugCommand_Help_TextContainsNoTabs(t *testing.T) {
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestDebugCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testDir := testutil.TempDir(t, "debug")

	a := agent.NewTestAgent(t, `
	enable_debug = true
	`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)
	cmd.validateTiming = false

	it := &incrementalTime{
		base: time.Date(2021, 7, 8, 9, 10, 11, 0, time.UTC),
	}
	cmd.timeNow = it.Now

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-duration=2s",
		"-interval=1s",
		"-archive=false",
	}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
	require.Equal(t, "", ui.ErrorWriter.String())

	expected := fs.Expected(t,
		fs.WithDir("debug",
			fs.WithFile("index.json", "", fs.MatchFileContent(validIndexJSON)),
			fs.WithFile("agent.json", "", fs.MatchFileContent(validJSON)),
			fs.WithFile("host.json", "", fs.MatchFileContent(validJSON)),
			fs.WithFile("members.json", "", fs.MatchFileContent(validJSON)),
			fs.WithFile("metrics.json", "", fs.MatchAnyFileContent),
			fs.WithFile("consul.log", "", fs.MatchFileContent(validLogFile)),
			fs.WithFile("profile.prof", "", fs.MatchFileContent(validProfileData)),
			fs.WithFile("trace.out", "", fs.MatchAnyFileContent),
			fs.WithDir("2021-07-08T09-10-12Z",
				fs.WithFile("goroutine.prof", "", fs.MatchFileContent(validProfileData)),
				fs.WithFile("heap.prof", "", fs.MatchFileContent(validProfileData))),
			fs.WithDir("2021-07-08T09-10-13Z",
				fs.WithFile("goroutine.prof", "", fs.MatchFileContent(validProfileData)),
				fs.WithFile("heap.prof", "", fs.MatchFileContent(validProfileData))),
			// Ignore the extra directories, they should be the same as the first two
			fs.MatchExtraFiles))
	assert.Assert(t, fs.Equal(testDir, expected))

	require.Equal(t, "", ui.ErrorWriter.String(), "expected no error output")
}

func validLogFile(raw []byte) fs.CompareResult {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		logLine := scanner.Text()
		if !validateLogLine([]byte(logLine)) {
			return cmp.ResultFailure(fmt.Sprintf("log line is not valid %s", logLine))
		}
	}
	if scanner.Err() != nil {
		return cmp.ResultFailure(scanner.Err().Error())
	}
	return cmp.ResultSuccess
}

func validIndexJSON(raw []byte) fs.CompareResult {
	var target debugIndex
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil {
		return cmp.ResultFailure(err.Error())
	}
	return cmp.ResultSuccess
}

func validJSON(raw []byte) fs.CompareResult {
	var target interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&target); err != nil {
		return cmp.ResultFailure(err.Error())
	}
	return cmp.ResultSuccess
}

func validProfileData(raw []byte) fs.CompareResult {
	if _, err := profile.ParseData(raw); err != nil {
		return cmp.ResultFailure(err.Error())
	}
	return cmp.ResultSuccess
}

type incrementalTime struct {
	base time.Time
	next uint64
}

func (t *incrementalTime) Now() time.Time {
	t.next++
	return t.base.Add(time.Duration(t.next) * time.Second)
}

func TestDebugCommand_Archive(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testDir := testutil.TempDir(t, "debug")

	a := agent.NewTestAgent(t, `
	enable_debug = true
	`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)
	cmd.validateTiming = false

	outputPath := fmt.Sprintf("%s/debug", testDir)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-output=" + outputPath,
		"-capture=agent",
	}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
	require.Equal(t, "", ui.ErrorWriter.String())

	archivePath := outputPath + debugArchiveExtension
	file, err := os.Open(archivePath)
	require.NoError(t, err)

	gz, err := gzip.NewReader(file)
	require.NoError(t, err)
	tr := tar.NewReader(gz)

	for {
		h, err := tr.Next()
		switch {
		case err == io.EOF:
			return
		case err != nil:
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
	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{"foo", "bad"}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should exit non-zero, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "Too many arguments") {
		t.Errorf("expected error output, got %q", errOutput)
	}
}

func TestDebugCommand_InvalidFlags(t *testing.T) {
	ui := cli.NewMockUi()
	cmd := New(ui)
	cmd.validateTiming = false

	outputPath := ""
	args := []string{
		"-invalid=value",
		"-output=" + outputPath,
		"-duration=100ms",
		"-interval=50ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should exit non-zero, got code: %d", code)
	}

	errOutput := ui.ErrorWriter.String()
	if !strings.Contains(errOutput, "==> Error parsing flags: flag provided but not defined:") {
		t.Errorf("expected error output, got %q", errOutput)
	}
}

func TestDebugCommand_OutputPathBad(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	testDir := testutil.TempDir(t, "debug")

	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			[]string{"metrics.json"},
		},
		"metrics-only": {
			[]string{"metrics"},
			[]string{"metrics.json"},
			[]string{"agent.json", "host.json", "members.json"},
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
				"members.json",
				"metrics.json",
				"consul.log",
			},
			[]string{},
		},
	}

	for name, tc := range cases {
		testDir := testutil.TempDir(t, "debug")

		a := agent.NewTestAgent(t, `
		enable_debug = true
		`)

		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		ui := cli.NewMockUi()
		cmd := New(ui)
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

func validateLogLine(content []byte) bool {
	fields := strings.SplitN(string(content), " ", 2)
	if len(fields) != 2 {
		return false
	}
	const logTimeFormat = "2006-01-02T15:04:05.000"
	t := content[:len(logTimeFormat)]
	_, err := time.Parse(logTimeFormat, string(t))
	if err != nil {
		return false
	}
	re := regexp.MustCompile(`(\[(ERROR|WARN|INFO|DEBUG|TRACE)]) (.*?): (.*)`)
	valid := re.Match([]byte(fields[1]))
	return valid
}

func TestDebugCommand_Prepare_ValidateTiming(t *testing.T) {
	cases := map[string]struct {
		duration string
		interval string
		expected string
	}{
		"both": {
			duration: "20ms",
			interval: "10ms",
			expected: "duration must be longer",
		},
		"short interval": {
			duration: "10s",
			interval: "10ms",
			expected: "interval must be longer",
		},
		"lower duration": {
			duration: "20s",
			interval: "30s",
			expected: "must be longer than interval",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)

			args := []string{
				"-duration=" + tc.duration,
				"-interval=" + tc.interval,
			}
			err := cmd.flags.Parse(args)
			require.NoError(t, err)

			_, err = cmd.prepare()
			testutil.RequireErrorContains(t, err, tc.expected)
		})
	}
}

func TestDebugCommand_DebugDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testDir := testutil.TempDir(t, "debug")

	a := agent.NewTestAgent(t, `
	enable_debug = false
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)
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
		fs, _ := filepath.Glob(fmt.Sprintf("%s/*r/%s", outputPath, v))
		require.True(t, len(fs) == 0)
	}

	errOutput := ui.ErrorWriter.String()
	require.Contains(t, errOutput, "Unable to capture pprof")
}
