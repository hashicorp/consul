package acl

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse_table(t *testing.T) {
	// Note that the table tests are newer than other tests. Many of the
	// other aspects of policy parsing are tested in older tests below. New
	// parsing tests should be added to this table as its easier to maintain.
	cases := []struct {
		Name     string
		Input    string
		Expected *Policy
		Err      string
	}{
		{
			"service no intentions",
			`
service "foo" {
	policy = "write"
}
			`,
			&Policy{
				Services: []*ServicePolicy{
					{
						Name:   "foo",
						Policy: "write",
					},
				},
			},
			"",
		},

		{
			"service intentions",
			`
service "foo" {
	policy = "write"
	intentions = "read"
}
			`,
			&Policy{
				Services: []*ServicePolicy{
					{
						Name:       "foo",
						Policy:     "write",
						Intentions: "read",
					},
				},
			},
			"",
		},

		{
			"service intention: invalid value",
			`
service "foo" {
	policy = "write"
	intentions = "foo"
}
			`,
			nil,
			"service intentions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			actual, err := Parse(tc.Input, nil)
			assert.Equal(tc.Err != "", err != nil, err)
			if err != nil {
				assert.Contains(err.Error(), tc.Err)
				return
			}
			assert.Equal(tc.Expected, actual)
		})
	}
}

func TestACLPolicy_Parse_HCL(t *testing.T) {
	inp := `
agent "foo" {
	policy = "read"
}
agent "bar" {
	policy = "write"
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
keyring = "deny"
node "" {
	policy = "read"
}
node "foo" {
	policy = "write"
}
node "bar" {
	policy = "deny"
}
operator = "deny"
service "" {
	policy = "write"
}
service "foo" {
	policy = "read"
}
session "foo" {
	policy = "write"
}
session "bar" {
	policy = "deny"
}
query "" {
	policy = "read"
}
query "foo" {
	policy = "write"
}
query "bar" {
	policy = "deny"
}
	`
	exp := &Policy{
		Agents: []*AgentPolicy{
			&AgentPolicy{
				Node:   "foo",
				Policy: PolicyRead,
			},
			&AgentPolicy{
				Node:   "bar",
				Policy: PolicyWrite,
			},
		},
		Events: []*EventPolicy{
			&EventPolicy{
				Event:  "",
				Policy: PolicyRead,
			},
			&EventPolicy{
				Event:  "foo",
				Policy: PolicyWrite,
			},
			&EventPolicy{
				Event:  "bar",
				Policy: PolicyDeny,
			},
		},
		Keyring: PolicyDeny,
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "",
				Policy: PolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/",
				Policy: PolicyWrite,
			},
			&KeyPolicy{
				Prefix: "foo/bar/",
				Policy: PolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/bar/baz",
				Policy: PolicyDeny,
			},
		},
		Nodes: []*NodePolicy{
			&NodePolicy{
				Name:   "",
				Policy: PolicyRead,
			},
			&NodePolicy{
				Name:   "foo",
				Policy: PolicyWrite,
			},
			&NodePolicy{
				Name:   "bar",
				Policy: PolicyDeny,
			},
		},
		Operator: PolicyDeny,
		PreparedQueries: []*PreparedQueryPolicy{
			&PreparedQueryPolicy{
				Prefix: "",
				Policy: PolicyRead,
			},
			&PreparedQueryPolicy{
				Prefix: "foo",
				Policy: PolicyWrite,
			},
			&PreparedQueryPolicy{
				Prefix: "bar",
				Policy: PolicyDeny,
			},
		},
		Services: []*ServicePolicy{
			&ServicePolicy{
				Name:   "",
				Policy: PolicyWrite,
			},
			&ServicePolicy{
				Name:   "foo",
				Policy: PolicyRead,
			},
		},
		Sessions: []*SessionPolicy{
			&SessionPolicy{
				Node:   "foo",
				Policy: PolicyWrite,
			},
			&SessionPolicy{
				Node:   "bar",
				Policy: PolicyDeny,
			},
		},
	}

	out, err := Parse(inp, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestACLPolicy_Parse_JSON(t *testing.T) {
	inp := `{
	"agent": {
		"foo": {
			"policy": "write"
		},
		"bar": {
			"policy": "deny"
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
	"keyring": "deny",
	"node": {
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
	"operator": "deny",
	"query": {
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
	"service": {
		"": {
			"policy": "write"
		},
		"foo": {
			"policy": "read"
		}
	},
	"session": {
		"foo": {
			"policy": "write"
		},
		"bar": {
			"policy": "deny"
		}
	}
}`
	exp := &Policy{
		Agents: []*AgentPolicy{
			&AgentPolicy{
				Node:   "foo",
				Policy: PolicyWrite,
			},
			&AgentPolicy{
				Node:   "bar",
				Policy: PolicyDeny,
			},
		},
		Events: []*EventPolicy{
			&EventPolicy{
				Event:  "",
				Policy: PolicyRead,
			},
			&EventPolicy{
				Event:  "foo",
				Policy: PolicyWrite,
			},
			&EventPolicy{
				Event:  "bar",
				Policy: PolicyDeny,
			},
		},
		Keyring: PolicyDeny,
		Keys: []*KeyPolicy{
			&KeyPolicy{
				Prefix: "",
				Policy: PolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/",
				Policy: PolicyWrite,
			},
			&KeyPolicy{
				Prefix: "foo/bar/",
				Policy: PolicyRead,
			},
			&KeyPolicy{
				Prefix: "foo/bar/baz",
				Policy: PolicyDeny,
			},
		},
		Nodes: []*NodePolicy{
			&NodePolicy{
				Name:   "",
				Policy: PolicyRead,
			},
			&NodePolicy{
				Name:   "foo",
				Policy: PolicyWrite,
			},
			&NodePolicy{
				Name:   "bar",
				Policy: PolicyDeny,
			},
		},
		Operator: PolicyDeny,
		PreparedQueries: []*PreparedQueryPolicy{
			&PreparedQueryPolicy{
				Prefix: "",
				Policy: PolicyRead,
			},
			&PreparedQueryPolicy{
				Prefix: "foo",
				Policy: PolicyWrite,
			},
			&PreparedQueryPolicy{
				Prefix: "bar",
				Policy: PolicyDeny,
			},
		},
		Services: []*ServicePolicy{
			&ServicePolicy{
				Name:   "",
				Policy: PolicyWrite,
			},
			&ServicePolicy{
				Name:   "foo",
				Policy: PolicyRead,
			},
		},
		Sessions: []*SessionPolicy{
			&SessionPolicy{
				Node:   "foo",
				Policy: PolicyWrite,
			},
			&SessionPolicy{
				Node:   "bar",
				Policy: PolicyDeny,
			},
		},
	}

	out, err := Parse(inp, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestACLPolicy_Keyring_Empty(t *testing.T) {
	inp := `
keyring = ""
	`
	exp := &Policy{
		Keyring: "",
	}

	out, err := Parse(inp, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestACLPolicy_Operator_Empty(t *testing.T) {
	inp := `
operator = ""
	`
	exp := &Policy{
		Operator: "",
	}

	out, err := Parse(inp, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, exp) {
		t.Fatalf("bad: %#v %#v", out, exp)
	}
}

func TestACLPolicy_Bad_Policy(t *testing.T) {
	cases := []string{
		`agent "" { policy = "nope" }`,
		`event "" { policy = "nope" }`,
		`key "" { policy = "nope" }`,
		`keyring = "nope"`,
		`node "" { policy = "nope" }`,
		`operator = "nope"`,
		`query "" { policy = "nope" }`,
		`service "" { policy = "nope" }`,
		`session "" { policy = "nope" }`,
	}
	for _, c := range cases {
		_, err := Parse(c, nil)
		if err == nil || !strings.Contains(err.Error(), "Invalid") {
			t.Fatalf("expected policy error, got: %#v", err)
		}
	}
}
