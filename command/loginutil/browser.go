// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package loginutil

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OpenURL opens the specified URL in the user's default browser. It starts the
// platform opener and returns without waiting for the browser to exit.
// Source: https://github.com/icza/gox/blob/b0478a1e0c7bd014f962a3911e1784b3d569329d/osx/osx.go
func OpenURL(url string) error {
	var cmd string
	var args []string

	switch {
	case runtime.GOOS == "windows" || isWSL():
		cmd = "cmd.exe"
		args = []string{"/c", "start"}
		url = strings.ReplaceAll(url, "&", "^&")
	case runtime.GOOS == "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// isWSL reports whether the binary is running in Windows Subsystem for Linux.
func isWSL() bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return false
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
