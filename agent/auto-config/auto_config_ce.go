// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package autoconf

// AutoConfigEnterprise has no fields in CE
type AutoConfigEnterprise struct{}

// newAutoConfigEnterprise initializes the enterprise AutoConfig struct
func newAutoConfigEnterprise(config Config) AutoConfigEnterprise {
	return AutoConfigEnterprise{}
}
