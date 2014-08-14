package acl

import (
	"testing"
)

func TestRootACL(t *testing.T) {
	if RootACL("allow") != AllowAll() {
		t.Fatalf("Bad root")
	}
	if RootACL("deny") != DenyAll() {
		t.Fatalf("Bad root")
	}
	if RootACL("manage") != ManageAll() {
		t.Fatalf("Bad root")
	}
	if RootACL("foo") != nil {
		t.Fatalf("bad root")
	}
}

func TestStaticACL(t *testing.T) {
	all := AllowAll()
	if _, ok := all.(*StaticACL); !ok {
		t.Fatalf("expected static")
	}

	none := DenyAll()
	if _, ok := none.(*StaticACL); !ok {
		t.Fatalf("expected static")
	}

	manage := ManageAll()
	if _, ok := none.(*StaticACL); !ok {
		t.Fatalf("expected static")
	}

	if !all.KeyRead("foobar") {
		t.Fatalf("should allow")
	}
	if !all.KeyWrite("foobar") {
		t.Fatalf("should allow")
	}
	if all.ACLList() {
		t.Fatalf("should not allow")
	}
	if all.ACLModify() {
		t.Fatalf("should not allow")
	}

	if none.KeyRead("foobar") {
		t.Fatalf("should not allow")
	}
	if none.KeyWrite("foobar") {
		t.Fatalf("should not allow")
	}
	if none.ACLList() {
		t.Fatalf("should not noneow")
	}
	if none.ACLModify() {
		t.Fatalf("should not noneow")
	}

	if !manage.KeyRead("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.KeyWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.ACLList() {
		t.Fatalf("should allow")
	}
	if !manage.ACLModify() {
		t.Fatalf("should allow")
	}
}

func TestPolicyACL(t *testing.T) {
	all := AllowAll()
	policy := &Policy{
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "foo/",
				Policy: KeyPolicyWrite,
			},
			&KeyPolicy{
				Prefix: "foo/priv/",
				Policy: KeyPolicyDeny,
			},
			&KeyPolicy{
				Prefix: "bar/",
				Policy: KeyPolicyDeny,
			},
			&KeyPolicy{
				Prefix: "zip/",
				Policy: KeyPolicyRead,
			},
		},
	}
	acl, err := New(all, policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	type tcase struct {
		inp         string
		read        bool
		write       bool
		writePrefix bool
	}
	cases := []tcase{
		{"other", true, true, true},
		{"foo/test", true, true, true},
		{"foo/priv/test", false, false, false},
		{"bar/any", false, false, false},
		{"zip/test", true, false, false},
		{"foo/", true, true, false},
		{"", true, true, false},
	}
	for _, c := range cases {
		if c.read != acl.KeyRead(c.inp) {
			t.Fatalf("Read fail: %#v", c)
		}
		if c.write != acl.KeyWrite(c.inp) {
			t.Fatalf("Write fail: %#v", c)
		}
		if c.writePrefix != acl.KeyWritePrefix(c.inp) {
			t.Fatalf("Write prefix fail: %#v", c)
		}
	}
}

func TestPolicyACL_Parent(t *testing.T) {
	deny := DenyAll()
	policyRoot := &Policy{
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "foo/",
				Policy: KeyPolicyWrite,
			},
			&KeyPolicy{
				Prefix: "bar/",
				Policy: KeyPolicyRead,
			},
		},
	}
	root, err := New(deny, policyRoot)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	policy := &Policy{
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "foo/priv/",
				Policy: KeyPolicyRead,
			},
			&KeyPolicy{
				Prefix: "bar/",
				Policy: KeyPolicyDeny,
			},
			&KeyPolicy{
				Prefix: "zip/",
				Policy: KeyPolicyRead,
			},
		},
	}
	acl, err := New(root, policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	type tcase struct {
		inp         string
		read        bool
		write       bool
		writePrefix bool
	}
	cases := []tcase{
		{"other", false, false, false},
		{"foo/test", true, true, true},
		{"foo/priv/test", true, false, false},
		{"bar/any", false, false, false},
		{"zip/test", true, false, false},
	}
	for _, c := range cases {
		if c.read != acl.KeyRead(c.inp) {
			t.Fatalf("Read fail: %#v", c)
		}
		if c.write != acl.KeyWrite(c.inp) {
			t.Fatalf("Write fail: %#v", c)
		}
		if c.writePrefix != acl.KeyWritePrefix(c.inp) {
			t.Fatalf("Write prefix fail: %#v", c)
		}
	}
}
