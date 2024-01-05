// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows
// +build !windows

package freeport

import "golang.org/x/sys/unix"

func systemLimit() (int, error) {
	var limit unix.Rlimit
	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit)
	return int(limit.Cur), err
}
