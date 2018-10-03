package debug

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
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
	args := []string{"-http-addr=" + a.HTTPAddr(), "-output=" + outputPath}

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
	args := []string{"-output=" + outputPath}

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
	args := []string{"-output=" + outputPath}

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
			[]string{},
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
				// "metrics/something.json",
				// "pprof/something.pprof",
				// "logs/something.log",
				"host.json",
				"agent.json",
				"members.json",
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
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		ui := cli.NewMockUi()
		cmd := New(ui, nil)

		outputPath := fmt.Sprintf("%s/debug-%s", testDir, name)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-output=" + outputPath,
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

		// Ensure the captured files exist
		for _, f := range tc.files {
			_, err := os.Stat(outputPath + "/" + f)
			if err != nil {
				t.Fatalf("%s: output data should exist for %s: %s", name, f, err)
			}
		}

		// Ensure any excluded files do not exist
		for _, f := range tc.excludedFiles {
			_, err := os.Stat(outputPath + "/" + f)
			if err == nil {
				t.Fatalf("%s: output data should not exist for %s: %s", name, f, err)
			}
		}
	}
}
