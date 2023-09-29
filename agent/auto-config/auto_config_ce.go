// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package autoconf

// AutoConfigEnterprise has no fields in CE
type AutoConfigEnterprise struct{}

// newAutoConfigEnterprise initializes the enterprise AutoConfig struct
func newAutoConfigEnterprise(config Config) AutoConfigEnterprise {
	return AutoConfigEnterprise{}
}
