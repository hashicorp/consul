package acl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func errStartsWith(t *testing.T, actual error, expected string) {
	t.Helper()
	require.Error(t, actual)
	require.Truef(t, strings.HasPrefix(actual.Error(), expected), "Received unexpected error: %#v\nExpecting an error with the prefix: %q", actual, expected)
}

func TestPolicySourceParse(t *testing.T) {
	ljoin := func(lines ...string) string {
		return strings.Join(lines, "\n")
	}
	cases := []struct {
		Name     string
		Syntax   SyntaxVersion
		Rules    string
		Expected *Policy
		Err      string
	}{
		{
			"Legacy Basic",
			SyntaxLegacy,
			ljoin(
				`agent "foo" {      `,
				`	policy = "read"  `,
				`}                  `,
				`agent "bar" {      `,
				`	policy = "write" `,
				`}                  `,
				`event "" {         `,
				`	policy = "read"  `,
				`}                  `,
				`event "foo" {      `,
				`	policy = "write" `,
				`}                  `,
				`event "bar" {      `,
				`	policy = "deny"  `,
				`}                  `,
				`key "" {           `,
				`	policy = "read"  `,
				`}                  `,
				`key "foo/" {       `,
				`	policy = "write" `,
				`}                  `,
				`key "foo/bar/" {   `,
				`	policy = "read"  `,
				`}                  `,
				`key "foo/bar/baz" {`,
				`	policy = "deny"  `,
				`}                  `,
				`keyring = "deny"   `,
				`node "" {          `,
				`	policy = "read"  `,
				`}                  `,
				`node "foo" {       `,
				`	policy = "write" `,
				`}                  `,
				`node "bar" {       `,
				`	policy = "deny"  `,
				`}                  `,
				`operator = "deny"  `,
				`service "" {       `,
				`	policy = "write" `,
				`}                  `,
				`service "foo" {    `,
				`	policy = "read"  `,
				`}                  `,
				`session "foo" {    `,
				`	policy = "write" `,
				`}                  `,
				`session "bar" {    `,
				`	policy = "deny"  `,
				`}                  `,
				`query "" {         `,
				`	policy = "read"  `,
				`}                  `,
				`query "foo" {      `,
				`	policy = "write" `,
				`}                  `,
				`query "bar" {      `,
				`	policy = "deny"  `,
				`}                  `),
			&Policy{
				AgentPrefixes: []*AgentPolicy{
					&AgentPolicy{
						Node:   "foo",
						Policy: PolicyRead,
					},
					&AgentPolicy{
						Node:   "bar",
						Policy: PolicyWrite,
					},
				},
				EventPrefixes: []*EventPolicy{
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
				KeyPrefixes: []*KeyPolicy{
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
				NodePrefixes: []*NodePolicy{
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
				PreparedQueryPrefixes: []*PreparedQueryPolicy{
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
				ServicePrefixes: []*ServicePolicy{
					&ServicePolicy{
						Name:   "",
						Policy: PolicyWrite,
					},
					&ServicePolicy{
						Name:   "foo",
						Policy: PolicyRead,
					},
				},
				SessionPrefixes: []*SessionPolicy{
					&SessionPolicy{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					&SessionPolicy{
						Node:   "bar",
						Policy: PolicyDeny,
					},
				},
			},
			"",
		},
		{
			"Legacy (JSON)",
			SyntaxLegacy,
			ljoin(
				`{                         `,
				`	"agent": {              `,
				`		"foo": {             `,
				`			"policy": "write" `,
				`		},                   `,
				`		"bar": {             `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	},                      `,
				`	"event": {              `,
				`		"": {                `,
				`			"policy": "read"  `,
				`		},                   `,
				`		"foo": {             `,
				`			"policy": "write" `,
				`		},                   `,
				`		"bar": {             `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	},                      `,
				`	"key": {                `,
				`		"": {                `,
				`			"policy": "read"  `,
				`		},                   `,
				`		"foo/": {            `,
				`			"policy": "write" `,
				`		},                   `,
				`		"foo/bar/": {        `,
				`			"policy": "read"  `,
				`		},                   `,
				`		"foo/bar/baz": {     `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	},                      `,
				`	"keyring": "deny",      `,
				`	"node": {               `,
				`		"": {                `,
				`			"policy": "read"  `,
				`		},                   `,
				`		"foo": {             `,
				`			"policy": "write" `,
				`		},                   `,
				`		"bar": {             `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	},                      `,
				`	"operator": "deny",     `,
				`	"query": {              `,
				`		"": {                `,
				`			"policy": "read"  `,
				`		},                   `,
				`		"foo": {             `,
				`			"policy": "write" `,
				`		},                   `,
				`		"bar": {             `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	},                      `,
				`	"service": {            `,
				`		"": {                `,
				`			"policy": "write" `,
				`		},                   `,
				`		"foo": {             `,
				`			"policy": "read"  `,
				`		}                    `,
				`	},                      `,
				`	"session": {            `,
				`		"foo": {             `,
				`			"policy": "write" `,
				`		},                   `,
				`		"bar": {             `,
				`			"policy": "deny"  `,
				`		}                    `,
				`	}                       `,
				`}                         `),
			&Policy{
				AgentPrefixes: []*AgentPolicy{
					&AgentPolicy{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					&AgentPolicy{
						Node:   "bar",
						Policy: PolicyDeny,
					},
				},
				EventPrefixes: []*EventPolicy{
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
				KeyPrefixes: []*KeyPolicy{
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
				NodePrefixes: []*NodePolicy{
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
				PreparedQueryPrefixes: []*PreparedQueryPolicy{
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
				ServicePrefixes: []*ServicePolicy{
					&ServicePolicy{
						Name:   "",
						Policy: PolicyWrite,
					},
					&ServicePolicy{
						Name:   "foo",
						Policy: PolicyRead,
					},
				},
				SessionPrefixes: []*SessionPolicy{
					&SessionPolicy{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					&SessionPolicy{
						Node:   "bar",
						Policy: PolicyDeny,
					},
				},
			},
			"",
		},
		{
			"Service No Intentions (Legacy)",
			SyntaxLegacy,
			ljoin(
				`service "foo" {    `,
				`   policy = "write"`,
				`}                  `),
			&Policy{
				ServicePrefixes: []*ServicePolicy{
					{
						Name:   "foo",
						Policy: "write",
					},
				},
			},
			"",
		},
		{
			"Service Intentions (Legacy)",
			SyntaxLegacy,
			ljoin(
				`service "foo" {       `,
				`   policy = "write"   `,
				`   intentions = "read"`,
				`}                     `),
			&Policy{
				ServicePrefixes: []*ServicePolicy{
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
			"Service Intention: invalid value (Legacy)",
			SyntaxLegacy,
			ljoin(
				`service "foo" {      `,
				`   policy = "write"  `,
				`   intentions = "foo"`,
				`}                    `),
			nil,
			"Invalid service intentions policy",
		},
		{
			"Bad Policy - ACL",

			SyntaxCurrent,
			`acl = "nope"`,
			nil,
			"Invalid acl policy",
		},
		{
			"Bad Policy - Agent",
			SyntaxCurrent,
			`agent "foo" { policy = "nope" }`,
			nil,
			"Invalid agent policy",
		},
		{
			"Bad Policy - Agent Prefix",
			SyntaxCurrent,
			`agent_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid agent_prefix policy",
		},
		{
			"Bad Policy - Key",
			SyntaxCurrent,
			`key "foo" { policy = "nope" }`,
			nil,
			"Invalid key policy",
		},
		{
			"Bad Policy - Key Prefix",
			SyntaxCurrent,
			`key_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid key_prefix policy",
		},
		{
			"Bad Policy - Node",
			SyntaxCurrent,
			`node "foo" { policy = "nope" }`,
			nil,
			"Invalid node policy",
		},
		{
			"Bad Policy - Node Prefix",
			SyntaxCurrent,
			`node_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid node_prefix policy",
		},
		{
			"Bad Policy - Service",
			SyntaxCurrent,
			`service "foo" { policy = "nope" }`,
			nil,
			"Invalid service policy",
		},
		{
			"Bad Policy - Service Prefix",
			SyntaxCurrent,
			`service_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid service_prefix policy",
		},
		{
			"Bad Policy - Session",
			SyntaxCurrent,
			`session "foo" { policy = "nope" }`,
			nil,
			"Invalid session policy",
		},
		{
			"Bad Policy - Session Prefix",
			SyntaxCurrent,
			`session_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid session_prefix policy",
		},
		{
			"Bad Policy - Event",
			SyntaxCurrent,
			`event "foo" { policy = "nope" }`,
			nil,
			"Invalid event policy",
		},
		{
			"Bad Policy - Event Prefix",
			SyntaxCurrent,
			`event_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid event_prefix policy",
		},
		{
			"Bad Policy - Prepared Query",
			SyntaxCurrent,
			`query "foo" { policy = "nope" }`,
			nil,
			"Invalid query policy",
		},
		{
			"Bad Policy - Prepared Query Prefix",
			SyntaxCurrent,
			`query_prefix "foo" { policy = "nope" }`,
			nil,
			"Invalid query_prefix policy",
		},
		{
			"Bad Policy - Keyring",
			SyntaxCurrent,
			`keyring = "nope"`,
			nil,
			"Invalid keyring policy",
		},
		{
			"Bad Policy - Operator",
			SyntaxCurrent,
			`operator = "nope"`,
			nil,
			"Invalid operator policy",
		},
		{
			"Keyring Empty",
			SyntaxCurrent,
			`keyring = ""`,
			&Policy{Keyring: ""},
			"",
		},
		{
			"Operator Empty",
			SyntaxCurrent,
			`operator = ""`,
			&Policy{Operator: ""},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := require.New(t)
			actual, err := NewPolicyFromSource("", 0, tc.Rules, tc.Syntax, nil)
			if tc.Err != "" {
				errStartsWith(t, err, tc.Err)
			} else {
				req.Equal(tc.Expected, actual)
			}
		})
	}
}
