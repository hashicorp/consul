// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !fips

package version

func IsFIPS() bool {
	return false
}

func GetFIPSInfo() string {
	return ""
}
