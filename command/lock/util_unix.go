// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows
// +build !windows

package lock

import (
	"syscall"
)

// signalPid sends a sig signal to the process with process id pid.
func signalPid(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}
