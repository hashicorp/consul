// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package autoconf

import (
	"testing"
)

func newEnterpriseConfig(t *testing.T) EnterpriseConfig {
	return EnterpriseConfig{}
}
