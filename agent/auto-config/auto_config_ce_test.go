// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package autoconf

import (
	"testing"
)

func newEnterpriseConfig(t *testing.T) EnterpriseConfig {
	return EnterpriseConfig{}
}
