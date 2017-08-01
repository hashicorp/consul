// +build windows

package agent

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ExecScript returns a command to execute a script
func ExecScript(script string) (*exec.Cmd, error) {
	shell := "cmd"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	scriptQuoted := "\"" + script + "\"";
	cmd := exec.Command(shell, "/C", scriptQuoted)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: strings.Join(cmd.Args, " "),
	}
	return cmd, nil
}
