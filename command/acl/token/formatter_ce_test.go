// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package token

import (
	"testing"
)

func TestFormatTokenExpanded(t *testing.T) {
	testFormatTokenExpanded(t, "FormatTokenExpanded/oss")
}
