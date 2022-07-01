//go:build integration
// +build integration

package envoy

import (
	"flag"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	flagWin = flag.Bool("win", false, "Execute tests on windows")
)

func TestEnvoy(t *testing.T) {
	flag.Parse()

	testcases, err := discoverCases()
	require.NoError(t, err)

	runCmd(t, "suite_setup")

	defer runCmd(t, "suite_teardown")

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			caseDir := "CASE_DIR=" + tc

			t.Cleanup(func() {
				if t.Failed() {
					runCmd(t, "capture_logs", caseDir)
				}

				runCmd(t, "test_teardown", caseDir)
			})

			runCmd(t, "run_tests", caseDir)
		})
	}
}

func runCmd(t *testing.T, c string, env ...string) {
	t.Helper()

	param_1 := " "
	param_2 := " "
	param_3 := " "
	param_4 := " "
	param_5 := "false"

	if *flagWin == true {
		param_1 = "cmd"
		param_2 = "/C"
		param_3 = "bash run-tests.windows.sh"
		param_4 = c
		if env != nil {
			param_5 = strings.Join(env, " ")
		}

	} else {
		param_1 = "./run-tests.sh"
		param_2 = c
	}

	cmd := exec.Command(param_1, param_2, param_3, param_4, param_5)

	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

// Discover the cases so we pick up both oss and ent copies.
func discoverCases() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dirs, err := os.ReadDir(cwd)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, fi := range dirs {
		if fi.IsDir() && strings.HasPrefix(fi.Name(), "case-") {
			out = append(out, fi.Name())
		}
	}

	sort.Strings(out)
	return out, nil
}
