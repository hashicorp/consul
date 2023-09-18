//go:build integration
// +build integration

package envoy

import (
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvoy(t *testing.T) {
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

	cmd := exec.Command("./run-tests.sh", c)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

// Discover the cases so we pick up both CE and ent copies.
func discoverCases() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dirs, err := ioutil.ReadDir(cwd)
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
