// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux && !darwin
// +build !linux,!darwin

package freeport

func getEphemeralPortRange() (int, int, error) {
	return 0, 0, nil
}
