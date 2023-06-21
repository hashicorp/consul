// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux
// +build !linux

package xds

func kernelSupportsIPv6() (bool, error) {
	return true, nil
}
