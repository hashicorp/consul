package agent

import (
	"os"
	"os/exec"
)

// ExecScript returns a command to execute a script
func ExecScript(script string, chroot string) (*exec.Cmd, error) {
	shell := "cmd"
	flag := "/C"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)
	return cmd, nil
}
