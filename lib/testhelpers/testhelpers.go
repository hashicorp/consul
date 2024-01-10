// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package testhelpers

import (
	"os"
	"testing"
)

func SkipFlake(t *testing.T) {
	if os.Getenv("RUN_FLAKEY_TESTS") != "true" {
		t.Skip("Skipped because marked as flake.")
	}
}
