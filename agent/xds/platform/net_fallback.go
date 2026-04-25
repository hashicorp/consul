// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package platform

func SupportsIPv6() (bool, error) {
	return true, nil
}
