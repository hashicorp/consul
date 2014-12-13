package acl

import (
	"reflect"
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
	}
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
	}

	out, err := Parse(inp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}
