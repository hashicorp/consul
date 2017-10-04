// +build !windows

package agent

import (
	"os"
	"os/exec"
)

// ExecScript returns a command to execute a script through a shell.
func ExecScript(script string) (*exec.Cmd, error) {
	shell := "/bin/sh"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	return exec.Command(shell, "-c", script), nil
}
