// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows
// +build windows

package exec

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Script returns a command to execute a script through a shell.
func Script(script string) (*exec.Cmd, error) {
	shell := "cmd"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	script = "\"" + script + "\""
	cmd := exec.Command(shell, "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: strings.Join(cmd.Args, " "),
	}
	return cmd, nil
}

func SetSysProcAttr(cmd *exec.Cmd) {}

func KillCommandSubtree(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
