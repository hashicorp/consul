package structs

import (
	"testing"
)

func TestStructs_ACL_IsSame(t *testing.T) {
	acl := &ACL{
		ID:    "guid",
		Name:  "An ACL for testing",
		Type:  "client",
		Rules: "service \"\" { policy = \"read\" }",
	}
	if !acl.IsSame(acl) {
		t.Fatalf("should be equal to itself")
	}

	other := &ACL{
		ID:    "guid",
		Name:  "An ACL for testing",
		Type:  "client",
		Rules: "service \"\" { policy = \"read\" }",
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}
	if !acl.IsSame(other) || !other.IsSame(acl) {
		t.Fatalf("should not care about Raft fields")
	}

	check := func(twiddle, restore func()) {
		if !acl.IsSame(other) || !other.IsSame(acl) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if acl.IsSame(other) || other.IsSame(acl) {
			t.Fatalf("should not be the same")
		}

		restore()
		if !acl.IsSame(other) || !other.IsSame(acl) {
			t.Fatalf("should be the same")
		}
	}

	check(func() { other.ID = "nope" }, func() { other.ID = "guid" })
	check(func() { other.Name = "nope" }, func() { other.Name = "An ACL for testing" })
	check(func() { other.Type = "management" }, func() { other.Type = "client" })
	check(func() { other.Rules = "" }, func() { other.Rules = "service \"\" { policy = \"read\" }" })
}
