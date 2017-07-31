// +build !windows

package agent

import (
	"os"
	"os/exec"
)


// ExecScript returns a command to execute a script
func ExecScript(script string) (*exec.Cmd, error) {
	var shell, flag string
	shell = "/bin/sh"
	flag = "-c"

	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)

	return cmd, nil
}
