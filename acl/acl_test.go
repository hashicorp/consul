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
	if !all.ServiceRead("foobar") {
		t.Fatalf("should allow")
	}
	if !all.ServiceWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !all.EventRead("foobar") {
		t.Fatalf("should allow")
	}
	if !all.EventWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !all.KeyringRead() {
		t.Fatalf("should allow")
	}
	if !all.KeyringWrite() {
		t.Fatalf("should allow")
	}
	if all.ACLList() {
		t.Fatalf("should not allow")
	}
	if all.ACLModify() {
		t.Fatalf("should not allow")
	}
	if all.QueryList() {
		t.Fatalf("should not allow")
	}
	if all.QueryModify() {
		t.Fatalf("should not allow")
	}

	if none.KeyRead("foobar") {
		t.Fatalf("should not allow")
	}
	if none.KeyWrite("foobar") {
		t.Fatalf("should not allow")
	}
	if none.ServiceRead("foobar") {
		t.Fatalf("should not allow")
	}
	if none.ServiceWrite("foobar") {
		t.Fatalf("should not allow")
	}
	if none.EventRead("foobar") {
		t.Fatalf("should not allow")
	}
	if none.EventRead("") {
		t.Fatalf("should not allow")
	}
	if none.EventWrite("foobar") {
		t.Fatalf("should not allow")
	}
	if none.EventWrite("") {
		t.Fatalf("should not allow")
	}
	if none.KeyringRead() {
		t.Fatalf("should now allow")
	}
	if none.KeyringWrite() {
		t.Fatalf("should not allow")
	}
	if none.ACLList() {
		t.Fatalf("should not allow")
	}
	if none.ACLModify() {
		t.Fatalf("should not allow")
	}
	if none.QueryList() {
		t.Fatalf("should not allow")
	}
	if none.QueryModify() {
		t.Fatalf("should not allow")
	}

	if !manage.KeyRead("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.KeyWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.ServiceRead("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.ServiceWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.EventRead("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.EventWrite("foobar") {
		t.Fatalf("should allow")
	}
	if !manage.KeyringRead() {
		t.Fatalf("should allow")
	}
	if !manage.KeyringWrite() {
		t.Fatalf("should allow")
	}
	if !manage.ACLList() {
		t.Fatalf("should allow")
	}
	if !manage.ACLModify() {
		t.Fatalf("should allow")
	}
	if !manage.QueryList() {
		t.Fatalf("should allow")
	}
	if !manage.QueryModify() {
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
		Services: []*ServicePolicy{
			&ServicePolicy{
				Name:   "",
				Policy: ServicePolicyWrite,
			},
			&ServicePolicy{
				Name:   "foo",
				Policy: ServicePolicyRead,
			},
			&ServicePolicy{
				Name:   "bar",
				Policy: ServicePolicyDeny,
			},
			&ServicePolicy{
				Name:   "barfoo",
				Policy: ServicePolicyWrite,
			},
		},
		Events: []*EventPolicy{
			&EventPolicy{
				Event:  "",
				Policy: EventPolicyRead,
			},
			&EventPolicy{
				Event:  "foo",
				Policy: EventPolicyWrite,
			},
			&EventPolicy{
				Event:  "bar",
				Policy: EventPolicyDeny,
			},
		},
	}
	acl, err := New(all, policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	type keycase struct {
		inp         string
		read        bool
		write       bool
		writePrefix bool
	}
	cases := []keycase{
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

	// Test the services
	type servicecase struct {
		inp   string
		read  bool
		write bool
	}
	scases := []servicecase{
		{"other", true, true},
		{"foo", true, false},
		{"bar", false, false},
		{"foobar", true, false},
		{"barfo", false, false},
		{"barfoo", true, true},
		{"barfoo2", true, true},
	}
	for _, c := range scases {
		if c.read != acl.ServiceRead(c.inp) {
			t.Fatalf("Read fail: %#v", c)
		}
		if c.write != acl.ServiceWrite(c.inp) {
			t.Fatalf("Write fail: %#v", c)
		}
	}

	// Test the events
	type eventcase struct {
		inp   string
		read  bool
		write bool
	}
	eventcases := []eventcase{
		{"foo", true, true},
		{"foobar", true, true},
		{"bar", false, false},
		{"barbaz", false, false},
		{"baz", true, false},
	}
	for _, c := range eventcases {
		if c.read != acl.EventRead(c.inp) {
			t.Fatalf("Event fail: %#v", c)
		}
		if c.write != acl.EventWrite(c.inp) {
			t.Fatalf("Event fail: %#v", c)
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
		Services: []*ServicePolicy{
			&ServicePolicy{
				Name:   "other",
				Policy: ServicePolicyWrite,
			},
			&ServicePolicy{
				Name:   "foo",
				Policy: ServicePolicyRead,
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
		Services: []*ServicePolicy{
			&ServicePolicy{
				Name:   "bar",
				Policy: ServicePolicyDeny,
			},
		},
	}
	acl, err := New(root, policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	type keycase struct {
		inp         string
		read        bool
		write       bool
		writePrefix bool
	}
	cases := []keycase{
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

	// Test the services
	type servicecase struct {
		inp   string
		read  bool
		write bool
	}
	scases := []servicecase{
		{"fail", false, false},
		{"other", true, true},
		{"foo", true, false},
		{"bar", false, false},
	}
	for _, c := range scases {
		if c.read != acl.ServiceRead(c.inp) {
			t.Fatalf("Read fail: %#v", c)
		}
		if c.write != acl.ServiceWrite(c.inp) {
			t.Fatalf("Write fail: %#v", c)
		}
	}

	// Check some management functions that chain up
	if acl.ACLList() {
		t.Fatalf("should not allow")
	}
	if acl.ACLModify() {
		t.Fatalf("should not allow")
	}
	if acl.QueryList() {
		t.Fatalf("should not allow")
	}
	if acl.QueryModify() {
		t.Fatalf("should not allow")
	}
}

func TestPolicyACL_Keyring(t *testing.T) {
	// Test keyring ACLs
	type keyringcase struct {
		inp   string
		read  bool
		write bool
	}
	keyringcases := []keyringcase{
		{"", false, false},
		{KeyringPolicyRead, true, false},
		{KeyringPolicyWrite, true, true},
		{KeyringPolicyDeny, false, false},
	}
	for _, c := range keyringcases {
		acl, err := New(DenyAll(), &Policy{Keyring: c.inp})
		if err != nil {
			t.Fatalf("bad: %s", err)
		}
		if acl.KeyringRead() != c.read {
			t.Fatalf("bad: %#v", c)
		}
		if acl.KeyringWrite() != c.write {
			t.Fatalf("bad: %#v", c)
		}
	}
}
