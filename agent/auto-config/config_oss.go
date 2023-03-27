// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package autoconf

// EnterpriseConfig stub - only populated in Consul Enterprise
type EnterpriseConfig struct{}

// finalize is a noop for OSS
func (_ *EnterpriseConfig) validateAndFinalize() error {
	return nil
}
