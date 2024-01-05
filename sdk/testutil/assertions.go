// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"strings"
	"testing"
)

// RequireErrorContains is a test helper for asserting that an error occurred
// and the error message returned contains the expected error message as a
// substring.
func RequireErrorContains(t testing.TB, err error, expectedErrorMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("An error is expected but got nil.")
	}
	if !strings.Contains(err.Error(), expectedErrorMessage) {
		t.Fatalf("expected err %v to contain %q", err, expectedErrorMessage)
	}
}

// RunStep is a test helper to help you stop a series of subtests from
// executing after the first one that fails.
func RunStep(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}
