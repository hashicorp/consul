// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

type EnterpriseConfig struct{}

func DefaultEnterpriseConfig() *EnterpriseConfig {
	return nil
}
