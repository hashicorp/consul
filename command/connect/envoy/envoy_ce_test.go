// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package envoy

// enterpriseGenerateConfigTestCases returns enterprise-only configurations to
// test in TestGenerateConfig.
func enterpriseGenerateConfigTestCases() []generateConfigTestCase {
	return nil
}
