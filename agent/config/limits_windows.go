// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows
// +build windows

package config

// getrlimit is no-op on Windows, as max fd/process is 2^24 on Wow64
// Return (16 777 216, nil)
func getrlimit() (uint64, error) {
	return 16_777_216, nil
}
