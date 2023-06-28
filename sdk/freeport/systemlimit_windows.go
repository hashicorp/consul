// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows
// +build windows

package freeport

func systemLimit() (int, error) {
	return 0, nil
}
