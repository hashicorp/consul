//go:build linux || darwin || windows
// +build linux darwin windows

package envoy

import (
	"fmt"
	"os"
	"strings"
)

// testSelfExecOverride is a way for the tests to no fork-bomb themselves by
// self-executing the whole test suite for each case recursively. It's gross but
// the least gross option I could think of.
var testSelfExecOverride string

func isHotRestartOption(s string) bool {
	restartOpts := []string{
		"--restart-epoch",
		"--hot-restart-version",
		"--drain-time-s",
		"--parent-shutdown-time-s",
	}
	for _, opt := range restartOpts {
		if s == opt {
			return true
		}
		if strings.HasPrefix(s, opt+"=") {
			return true
		}
	}
	return false
}

func hasHotRestartOption(argSets ...[]string) bool {
	for _, args := range argSets {
		for _, opt := range args {
			if isHotRestartOption(opt) {
				return true
			}
		}
	}
	return false
}

// execArgs returns the command and args used to execute a binary. By default it
// will return a command of os.Executable with the args unmodified. This is a shim
// for testing, and can be overridden to execute using 'go run' instead.
var execArgs = func(args ...string) (string, []string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", nil, err
	}

	if strings.HasSuffix(execPath, "/envoy.test") {
		return "", nil, fmt.Errorf("set execArgs to use 'go run' instead of doing a self-exec")
	}

	return execPath, args, nil
}
