// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build integration
// +build integration

package envoy

import (
	"flag"
	"io/ioutil"
	"log"
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

	if *flagWin == true {
		dir := "../../../"
		check_dir_files(dir)
	}

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

func runCmdLinux(t *testing.T, c string, env ...string) {
	t.Helper()

	cmd := exec.Command("./run-tests.sh", c)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

func runCmdWindows(t *testing.T, c string, env ...string) {
	t.Helper()

	param_5 := "false"
	if env != nil {
		param_5 = strings.Join(env, " ")
	}

	cmd := exec.Command("cmd", "/C", "bash run-tests.windows.sh", c, param_5)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

func runCmd(t *testing.T, c string, env ...string) {
	t.Helper()

	if *flagWin == true {
		runCmdWindows(t, c, env...)

	} else {
		runCmdLinux(t, c, env...)
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

// CRLF convert functions
// Recursively iterates through the directory passed by parameter looking for the sh and bash files.
// Upon finding them, it calls crlf_file_check.
func check_dir_files(path string) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, fil := range files {

		v := strings.Split(fil.Name(), ".")
		file_extension := v[len(v)-1]

		file_path := path + "/" + fil.Name()

		if fil.IsDir() == true {
			check_dir_files(file_path)
		}

		if file_extension == "sh" || file_extension == "bash" {
			crlf_file_check(file_path)
		}
	}
}

// Check if a file contains CRLF line endings if so call crlf_normalize
func crlf_file_check(file_name string) {

	file, err := ioutil.ReadFile(file_name)
	text := string(file)

	if edit := crlf_verify(text); edit != -1 {
		crlf_normalize(file_name, text)
	}

	if err != nil {
		log.Fatal(err)
	}
}

// Checks for the existence of CRLF line endings.
func crlf_verify(text string) int {
	position := strings.Index(text, "\r\n")
	return position
}

// Replace CRLF line endings with LF.
func crlf_normalize(filename, text string) {
	text = strings.Replace(text, "\r\n", "\n", -1)
	data := []byte(text)

	ioutil.WriteFile(filename, data, 0644)
}
