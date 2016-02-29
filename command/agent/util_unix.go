// +build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

// ExecScript returns a command to execute a script
func ExecScript(script string, chroot string) (*exec.Cmd, error) {
	shell := "/bin/sh"
	flag := "-c"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)
	if chroot != "" {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Chroot = chroot
		cmd.Dir = "/"
	}
	return cmd, nil
}
