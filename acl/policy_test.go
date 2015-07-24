package acl

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	inp := `
key "" {
	policy = "read"
}
key "foo/" {
	policy = "write"
}
key "foo/bar/" {
	policy = "read"
}
key "foo/bar/baz" {
	policy = "deny"
}
service "" {
	policy = "write"
}
service "foo" {
	policy = "read"
}
event "" {
	policy = "read"
}
event "foo" {
	policy = "write"
}
event "bar" {
	policy = "deny"
}
keyring = "deny"
	`
	exp := &Policy{
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "",
				Policy: KeyPolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/",
				Policy: KeyPolicyWrite,
			},
			&KeyPolicy{
				Prefix: "foo/bar/",
				Policy: KeyPolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/bar/baz",
				Policy: KeyPolicyDeny,
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
		Keyring: KeyringPolicyDeny,
	}

	out, err := Parse(inp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestParse_JSON(t *testing.T) {
	inp := `{
	"key": {
		"": {
			"policy": "read"
		},
		"foo/": {
			"policy": "write"
		},
		"foo/bar/": {
			"policy": "read"
		},
		"foo/bar/baz": {
			"policy": "deny"
		}
	},
	"service": {
		"": {
			"policy": "write"
		},
		"foo": {
			"policy": "read"
		}
	},
	"event": {
		"": {
			"policy": "read"
		},
		"foo": {
			"policy": "write"
		},
		"bar": {
			"policy": "deny"
		}
	},
	"keyring": "deny"
}`
	exp := &Policy{
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "",
				Policy: KeyPolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/",
				Policy: KeyPolicyWrite,
			},
			&KeyPolicy{
				Prefix: "foo/bar/",
				Policy: KeyPolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/bar/baz",
				Policy: KeyPolicyDeny,
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
		Keyring: KeyringPolicyDeny,
	}

	out, err := Parse(inp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestACLPolicy_badPolicy(t *testing.T) {
	cases := []string{
		`key "" { policy = "nope" }`,
		`service "" { policy = "nope" }`,
		`event "" { policy = "nope" }`,
		`keyring = "nope"`,
	}
	for _, c := range cases {
		_, err := Parse(c)
		if err == nil || !strings.Contains(err.Error(), "Invalid") {
			t.Fatalf("expected policy error, got: %#v", err)
		}
	}
}
