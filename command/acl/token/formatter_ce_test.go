// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package token

import (
	"testing"
)

func TestFormatTokenExpanded(t *testing.T) {
	testFormatTokenExpanded(t, "FormatTokenExpanded/oss")
}
