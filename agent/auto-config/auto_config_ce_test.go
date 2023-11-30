// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package autoconf

import (
	"testing"
)

func newEnterpriseConfig(t *testing.T) EnterpriseConfig {
	return EnterpriseConfig{}
}
