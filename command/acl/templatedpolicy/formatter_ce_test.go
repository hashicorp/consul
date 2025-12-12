// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package templatedpolicy

import "testing"

func TestFormatTemplatedPolicy(t *testing.T) {
	testFormatTemplatedPolicy(t, "FormatTemplatedPolicy/ce")
}

func TestFormatTemplatedPolicyList(t *testing.T) {
	testFormatTemplatedPolicyList(t, "FormatTemplatedPolicyList/ce")
}
