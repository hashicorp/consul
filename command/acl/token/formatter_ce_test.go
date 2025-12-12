// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package token

import (
	"testing"
)

func TestFormatTokenExpanded(t *testing.T) {
	testFormatTokenExpanded(t, "FormatTokenExpanded/ce")
}
