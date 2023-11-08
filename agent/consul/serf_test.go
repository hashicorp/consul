// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"
)

func TestUserEventNames(t *testing.T) {
	t.Parallel()
	out := userEventName("foo")
	if out != "consul:event:foo" {
		t.Fatalf("bad: %v", out)
	}
	if !isUserEvent(out) {
		t.Fatalf("bad")
	}
	if isUserEvent("foo") {
		t.Fatalf("bad")
	}
	if raw := rawUserEventName(out); raw != "foo" {
		t.Fatalf("bad: %v", raw)
	}
}
