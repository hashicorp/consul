package acl

import (
	"testing"
)

func TestCache_GetPolicy(t *testing.T) {
	c, err := NewCache(1, AllowAll(), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	p, err := c.GetPolicy("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should get the same policy
	p1, err := c.GetPolicy("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p != p1 {
		t.Fatalf("should be cached")
	}

	// Cache a new policy
	_, err = c.GetPolicy(testSimplePolicy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Test invalidation of p
	p3, err := c.GetPolicy("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p == p3 {
		t.Fatalf("should be not cached")
	}
}

func TestCache_GetACL(t *testing.T) {
	policies := map[string]string{
		"foo": testSimplePolicy,
		"bar": testSimplePolicy,
	}
	faultfn := func(id string) (string, error) {
		return policies[id], nil
	}

	c, err := NewCache(1, DenyAll(), faultfn)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := c.GetACL("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if acl.KeyRead("bar/test") {
		t.Fatalf("should deny")
	}
	if !acl.KeyRead("foo/test") {
		t.Fatalf("should allow")
	}

	acl2, err := c.GetACL("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if acl != acl2 {
		t.Fatalf("should be cached")
	}

	// Invalidate cache
	_, err = c.GetACL("bar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl3, err := c.GetACL("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if acl == acl3 {
		t.Fatalf("should not be cached")
	}
}

func TestCache_ClearACL(t *testing.T) {
	policies := map[string]string{
		"foo": testSimplePolicy,
		"bar": testSimplePolicy,
	}
	faultfn := func(id string) (string, error) {
		return policies[id], nil
	}

	c, err := NewCache(1, DenyAll(), faultfn)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := c.GetACL("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke the cache
	c.ClearACL("foo")

	acl2, err := c.GetACL("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if acl == acl2 {
		t.Fatalf("should not be cached")
	}
}

var testSimplePolicy = `
key "foo/" {
	policy = "read"
}
`
