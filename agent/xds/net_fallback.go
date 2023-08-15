// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package xds

func kernelSupportsIPv6() (bool, error) {
	return true, nil
}
