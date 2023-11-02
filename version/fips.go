// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !fips

package version

func IsFIPS() bool {
	return false
}

func GetFIPSInfo() string {
	return ""
}
