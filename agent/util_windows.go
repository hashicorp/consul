// +build windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

// makeCmdLine builds a command line out of args by escaping "special"
// characters and joining the arguments with spaces.
func makeCmdLine(args []string) string {
	var s string
	for _, v := range args {
		if s != "" {
			s += " "
		}
		s += v
	}
	return s
}

// ExecScript returns a command to execute a script
func ExecScript(script string) (*exec.Cmd, error) {
	var shell, flag string
	shell = "cmd"
	flag = "/C"

	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)

	var cmdLine string
	cmdLine = makeCmdLine(cmd.Args)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: cmdLine,
	}

	return cmd, nil
}
