// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	cases := []struct {
		Name      string
		Rules     string
		RulesJSON string
		Expected  *Policy
		Err       string
	}{
		{
			Name: "Basic",
			Rules: `
				agent_prefix "bar" {
					policy = "write"
				}
				agent "foo" {
					policy = "read"
				}
				event_prefix "" {
					policy = "read"
				}
				event "foo" {
					policy = "write"
				}
				event "bar" {
					policy = "deny"
				}
				key_prefix "" {
					policy = "read"
				}
				key_prefix "foo/" {
					policy = "write"
				}
				key_prefix "foo/bar/" {
					policy = "read"
				}
				key "foo/bar/baz" {
					policy = "deny"
				}
				keyring = "deny"
				node_prefix "" {
					policy = "read"
				}
				node "foo" {
					policy = "write"
				}
				node "bar" {
					policy = "deny"
				}
				operator = "deny"
				mesh = "deny"
				peering = "deny"
				service_prefix "" {
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
				session_prefix "baz" {
					policy = "deny"
				}
				query_prefix "" {
					policy = "read"
				}
				query "foo" {
					policy = "write"
				}
				query "bar" {
					policy = "deny"
				}
				`,
			RulesJSON: `
				{
				  "agent_prefix": {
					"bar": {
					  "policy": "write"
					}
				  },
				  "agent": {
					"foo": {
					  "policy": "read"
					}
				  },
				  "event_prefix": {
					"": {
					  "policy": "read"
					}
				  },
				  "event": {
					"foo": {
					  "policy": "write"
					},
					"bar": {
					  "policy": "deny"
					}
				  },
				  "key_prefix": {
					"": {
					  "policy": "read"
					},
					"foo/": {
					  "policy": "write"
					},
					"foo/bar/": {
					  "policy": "read"
					}
				  },
				  "key": {
					"foo/bar/baz": {
					  "policy": "deny"
					}
				  },
				  "keyring": "deny",
				  "node_prefix": {
					"": {
					  "policy": "read"
					}
				  },
				  "node": {
					"foo": {
					  "policy": "write"
					},
					"bar": {
					  "policy": "deny"
					}
				  },
				  "operator": "deny",
				  "mesh": "deny",
				  "peering": "deny",
				  "service_prefix": {
					"": {
					  "policy": "write"
					}
				  },
				  "service": {
					"foo": {
					  "policy": "read"
					}
				  },
				  "session_prefix": {
					"baz": {
					  "policy": "deny"
					}
				  },
				  "session": {
					"foo": {
					  "policy": "write"
					},
					"bar": {
					  "policy": "deny"
					}
				  },
				  "query_prefix": {
					"": {
					  "policy": "read"
					}
				  },
				  "query": {
					"foo": {
					  "policy": "write"
					},
					"bar": {
					  "policy": "deny"
					}
				  }
				}
				`,
			Expected: &Policy{PolicyRules: PolicyRules{
				AgentPrefixes: []*AgentRule{
					{
						Node:   "bar",
						Policy: PolicyWrite,
					},
				},
				Agents: []*AgentRule{
					{
						Node:   "foo",
						Policy: PolicyRead,
					},
				},
				EventPrefixes: []*EventRule{
					{
						Event:  "",
						Policy: PolicyRead,
					},
				},
				Events: []*EventRule{
					{
						Event:  "foo",
						Policy: PolicyWrite,
					},
					{
						Event:  "bar",
						Policy: PolicyDeny,
					},
				},
				Keyring: PolicyDeny,
				KeyPrefixes: []*KeyRule{
					{
						Prefix: "",
						Policy: PolicyRead,
					},
					{
						Prefix: "foo/",
						Policy: PolicyWrite,
					},
					{
						Prefix: "foo/bar/",
						Policy: PolicyRead,
					},
				},
				Keys: []*KeyRule{
					{
						Prefix: "foo/bar/baz",
						Policy: PolicyDeny,
					},
				},
				NodePrefixes: []*NodeRule{
					{
						Name:   "",
						Policy: PolicyRead,
					},
				},
				Nodes: []*NodeRule{
					{
						Name:   "foo",
						Policy: PolicyWrite,
					},
					{
						Name:   "bar",
						Policy: PolicyDeny,
					},
				},
				Operator: PolicyDeny,
				Mesh:     PolicyDeny,
				Peering:  PolicyDeny,
				PreparedQueryPrefixes: []*PreparedQueryRule{
					{
						Prefix: "",
						Policy: PolicyRead,
					},
				},
				PreparedQueries: []*PreparedQueryRule{
					{
						Prefix: "foo",
						Policy: PolicyWrite,
					},
					{
						Prefix: "bar",
						Policy: PolicyDeny,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:   "",
						Policy: PolicyWrite,
					},
				},
				Services: []*ServiceRule{
					{
						Name:   "foo",
						Policy: PolicyRead,
					},
				},
				SessionPrefixes: []*SessionRule{
					{
						Node:   "baz",
						Policy: PolicyDeny,
					},
				},
				Sessions: []*SessionRule{
					{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					{
						Node:   "bar",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			Name:      "Service No Intentions",
			Rules:     `service "foo" { policy = "write" }`,
			RulesJSON: `{ "service": { "foo": { "policy": "write" }}}`,
			Expected: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:   "foo",
						Policy: "write",
					},
				},
			}},
		},
		{
			Name:      "Service Intentions",
			Rules:     `service "foo" { policy = "write" intentions = "read" }`,
			RulesJSON: `{ "service": { "foo": { "policy": "write", "intentions": "read" }}}`,
			Expected: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     "write",
						Intentions: "read",
					},
				},
			}},
		},
		{
			Name:      "Service Intention: invalid value",
			Rules:     `service "foo" { policy = "write" intentions = "foo" }`,
			RulesJSON: `{ "service": { "foo": { "policy": "write", "intentions": "foo" }}}`,
			Err:       "Invalid service intentions policy",
		},
		{
			Name:      "Bad Policy - ACL",
			Rules:     `acl = "list"`,      // there is no list policy but this helps to exercise another check in isPolicyValid
			RulesJSON: `{ "acl": "list" }`, // there is no list policy but this helps to exercise another check in isPolicyValid
			Err:       "Invalid acl policy",
		},
		{
			Name:      "Bad Policy - Agent",
			Rules:     `agent "foo" { policy = "nope" }`,
			RulesJSON: `{ "agent": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid agent policy",
		},
		{
			Name:      "Bad Policy - Agent Prefix",
			Rules:     `agent_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "agent_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid agent_prefix policy",
		},
		{
			Name:      "Bad Policy - Key",
			Rules:     `key "foo" { policy = "nope" }`,
			RulesJSON: `{ "key": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid key policy",
		},
		{
			Name:      "Bad Policy - Key Prefix",
			Rules:     `key_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "key_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid key_prefix policy",
		},
		{
			Name:      "Bad Policy - Node",
			Rules:     `node "foo" { policy = "nope" }`,
			RulesJSON: `{ "node": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid node policy",
		},
		{
			Name:      "Bad Policy - Node Prefix",
			Rules:     `node_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "node_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid node_prefix policy",
		},
		{
			Name:      "Bad Policy - Service",
			Rules:     `service "foo" { policy = "nope" }`,
			RulesJSON: `{ "service": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid service policy",
		},
		{
			Name:      "Bad Policy - Service Prefix",
			Rules:     `service_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "service_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid service_prefix policy",
		},
		{
			Name:      "Bad Policy - Session",
			Rules:     `session "foo" { policy = "nope" }`,
			RulesJSON: `{ "session": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid session policy",
		},
		{
			Name:      "Bad Policy - Session Prefix",
			Rules:     `session_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "session_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid session_prefix policy",
		},
		{
			Name:      "Bad Policy - Event",
			Rules:     `event "foo" { policy = "nope" }`,
			RulesJSON: `{ "event": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid event policy",
		},
		{
			Name:      "Bad Policy - Event Prefix",
			Rules:     `event_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "event_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid event_prefix policy",
		},
		{
			Name:      "Bad Policy - Prepared Query",
			Rules:     `query "foo" { policy = "nope" }`,
			RulesJSON: `{ "query": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid query policy",
		},
		{
			Name:      "Bad Policy - Prepared Query Prefix",
			Rules:     `query_prefix "foo" { policy = "nope" }`,
			RulesJSON: `{ "query_prefix": { "foo": { "policy": "nope" }}}`,
			Err:       "Invalid query_prefix policy",
		},
		{
			Name:      "Bad Policy - Keyring",
			Rules:     `keyring = "nope"`,
			RulesJSON: `{ "keyring": "nope" }`,
			Err:       "Invalid keyring policy",
		},
		{
			Name:      "Bad Policy - Operator",
			Rules:     `operator = "nope"`,
			RulesJSON: `{ "operator": "nope" }`,
			Err:       "Invalid operator policy",
		},
		{
			Name:      "Bad Policy - Mesh",
			Rules:     `mesh = "nope"`,
			RulesJSON: `{ "mesh": "nope" }`,
			Err:       "Invalid mesh policy",
		},
		{
			Name:      "Bad Policy - Peering",
			Rules:     `peering = "nope"`,
			RulesJSON: `{ "peering": "nope" }`,
			Err:       "Invalid peering policy",
		},
		{
			Name:      "Keyring Empty",
			Rules:     `keyring = ""`,
			RulesJSON: `{ "keyring": "" }`,
			Expected:  &Policy{PolicyRules: PolicyRules{Keyring: ""}},
		},
		{
			Name:      "Operator Empty",
			Rules:     `operator = ""`,
			RulesJSON: `{ "operator": "" }`,
			Expected:  &Policy{PolicyRules: PolicyRules{Operator: ""}},
		},
		{
			Name:      "Mesh Empty",
			Rules:     `mesh = ""`,
			RulesJSON: `{ "mesh": "" }`,
			Expected:  &Policy{PolicyRules: PolicyRules{Mesh: ""}},
		},
		{
			Name:      "Peering Empty",
			Rules:     `peering = ""`,
			RulesJSON: `{ "peering": "" }`,
			Expected:  &Policy{PolicyRules: PolicyRules{Peering: ""}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require.True(t, tc.Rules != "" || tc.RulesJSON != "")
			if tc.Rules != "" {
				t.Run("hcl", func(t *testing.T) {
					actual, err := NewPolicyFromSource(tc.Rules, nil, nil)
					if tc.Err != "" {
						errStartsWith(t, err, tc.Err)
					} else {
						require.Equal(t, tc.Expected, actual)
					}
				})
			}
			if tc.RulesJSON != "" {
				t.Run("json", func(t *testing.T) {
					actual, err := NewPolicyFromSource(tc.RulesJSON, nil, nil)
					if tc.Err != "" {
						errStartsWith(t, err, tc.Err)
					} else {
						require.Equal(t, tc.Expected, actual)
					}
				})
			}
		})
	}
}

func TestMergePolicies(t *testing.T) {
	type mergeTest struct {
		name     string
		input    []*Policy
		expected *Policy
	}

	tests := []mergeTest{
		{
			name: "Agents",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Agents: []*AgentRule{
						{
							Node:   "foo",
							Policy: PolicyWrite,
						},
						{
							Node:   "bar",
							Policy: PolicyRead,
						},
						{
							Node:   "baz",
							Policy: PolicyWrite,
						},
					},
					AgentPrefixes: []*AgentRule{
						{
							Node:   "000",
							Policy: PolicyWrite,
						},
						{
							Node:   "111",
							Policy: PolicyRead,
						},
						{
							Node:   "222",
							Policy: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Agents: []*AgentRule{
						{
							Node:   "foo",
							Policy: PolicyRead,
						},
						{
							Node:   "baz",
							Policy: PolicyDeny,
						},
					},
					AgentPrefixes: []*AgentRule{
						{
							Node:   "000",
							Policy: PolicyRead,
						},
						{
							Node:   "222",
							Policy: PolicyDeny,
						},
					},
				},
				}},
			expected: &Policy{PolicyRules: PolicyRules{
				Agents: []*AgentRule{
					{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					{
						Node:   "bar",
						Policy: PolicyRead,
					},
					{
						Node:   "baz",
						Policy: PolicyDeny,
					},
				},
				AgentPrefixes: []*AgentRule{
					{
						Node:   "000",
						Policy: PolicyWrite,
					},
					{
						Node:   "111",
						Policy: PolicyRead,
					},
					{
						Node:   "222",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Events",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Events: []*EventRule{
						{
							Event:  "foo",
							Policy: PolicyWrite,
						},
						{
							Event:  "bar",
							Policy: PolicyRead,
						},
						{
							Event:  "baz",
							Policy: PolicyWrite,
						},
					},
					EventPrefixes: []*EventRule{
						{
							Event:  "000",
							Policy: PolicyWrite,
						},
						{
							Event:  "111",
							Policy: PolicyRead,
						},
						{
							Event:  "222",
							Policy: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Events: []*EventRule{
						{
							Event:  "foo",
							Policy: PolicyRead,
						},
						{
							Event:  "baz",
							Policy: PolicyDeny,
						},
					},
					EventPrefixes: []*EventRule{
						{
							Event:  "000",
							Policy: PolicyRead,
						},
						{
							Event:  "222",
							Policy: PolicyDeny,
						},
					},
				}},
			},
			expected: &Policy{PolicyRules: PolicyRules{
				Events: []*EventRule{
					{
						Event:  "foo",
						Policy: PolicyWrite,
					},
					{
						Event:  "bar",
						Policy: PolicyRead,
					},
					{
						Event:  "baz",
						Policy: PolicyDeny,
					},
				},
				EventPrefixes: []*EventRule{
					{
						Event:  "000",
						Policy: PolicyWrite,
					},
					{
						Event:  "111",
						Policy: PolicyRead,
					},
					{
						Event:  "222",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Node",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Nodes: []*NodeRule{
						{
							Name:   "foo",
							Policy: PolicyWrite,
						},
						{
							Name:   "bar",
							Policy: PolicyRead,
						},
						{
							Name:   "baz",
							Policy: PolicyWrite,
						},
					},
					NodePrefixes: []*NodeRule{
						{
							Name:   "000",
							Policy: PolicyWrite,
						},
						{
							Name:   "111",
							Policy: PolicyRead,
						},
						{
							Name:   "222",
							Policy: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Nodes: []*NodeRule{
						{
							Name:   "foo",
							Policy: PolicyRead,
						},
						{
							Name:   "baz",
							Policy: PolicyDeny,
						},
					},
					NodePrefixes: []*NodeRule{
						{
							Name:   "000",
							Policy: PolicyRead,
						},
						{
							Name:   "222",
							Policy: PolicyDeny,
						},
					},
				},
				}},
			expected: &Policy{PolicyRules: PolicyRules{
				Nodes: []*NodeRule{
					{
						Name:   "foo",
						Policy: PolicyWrite,
					},
					{
						Name:   "bar",
						Policy: PolicyRead,
					},
					{
						Name:   "baz",
						Policy: PolicyDeny,
					},
				},
				NodePrefixes: []*NodeRule{
					{
						Name:   "000",
						Policy: PolicyWrite,
					},
					{
						Name:   "111",
						Policy: PolicyRead,
					},
					{
						Name:   "222",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Keys",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Keys: []*KeyRule{
						{
							Prefix: "foo",
							Policy: PolicyWrite,
						},
						{
							Prefix: "bar",
							Policy: PolicyRead,
						},
						{
							Prefix: "baz",
							Policy: PolicyWrite,
						},
						{
							Prefix: "zoo",
							Policy: PolicyList,
						},
					},
					KeyPrefixes: []*KeyRule{
						{
							Prefix: "000",
							Policy: PolicyWrite,
						},
						{
							Prefix: "111",
							Policy: PolicyRead,
						},
						{
							Prefix: "222",
							Policy: PolicyWrite,
						},
						{
							Prefix: "333",
							Policy: PolicyList,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Keys: []*KeyRule{
						{
							Prefix: "foo",
							Policy: PolicyRead,
						},
						{
							Prefix: "baz",
							Policy: PolicyDeny,
						},
						{
							Prefix: "zoo",
							Policy: PolicyRead,
						},
					},
					KeyPrefixes: []*KeyRule{
						{
							Prefix: "000",
							Policy: PolicyRead,
						},
						{
							Prefix: "222",
							Policy: PolicyDeny,
						},
						{
							Prefix: "333",
							Policy: PolicyRead,
						},
					},
				}},
			},
			expected: &Policy{PolicyRules: PolicyRules{
				Keys: []*KeyRule{
					{
						Prefix: "foo",
						Policy: PolicyWrite,
					},
					{
						Prefix: "bar",
						Policy: PolicyRead,
					},
					{
						Prefix: "baz",
						Policy: PolicyDeny,
					},
					{
						Prefix: "zoo",
						Policy: PolicyList,
					},
				},
				KeyPrefixes: []*KeyRule{
					{
						Prefix: "000",
						Policy: PolicyWrite,
					},
					{
						Prefix: "111",
						Policy: PolicyRead,
					},
					{
						Prefix: "222",
						Policy: PolicyDeny,
					},
					{
						Prefix: "333",
						Policy: PolicyList,
					},
				},
			}},
		},
		{
			name: "Services",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Services: []*ServiceRule{
						{
							Name:       "foo",
							Policy:     PolicyWrite,
							Intentions: PolicyWrite,
						},
						{
							Name:       "bar",
							Policy:     PolicyRead,
							Intentions: PolicyRead,
						},
						{
							Name:       "baz",
							Policy:     PolicyWrite,
							Intentions: PolicyWrite,
						},
					},
					ServicePrefixes: []*ServiceRule{
						{
							Name:       "000",
							Policy:     PolicyWrite,
							Intentions: PolicyWrite,
						},
						{
							Name:       "111",
							Policy:     PolicyRead,
							Intentions: PolicyRead,
						},
						{
							Name:       "222",
							Policy:     PolicyWrite,
							Intentions: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Services: []*ServiceRule{
						{
							Name:       "foo",
							Policy:     PolicyRead,
							Intentions: PolicyRead,
						},
						{
							Name:       "baz",
							Policy:     PolicyDeny,
							Intentions: PolicyDeny,
						},
					},
					ServicePrefixes: []*ServiceRule{
						{
							Name:       "000",
							Policy:     PolicyRead,
							Intentions: PolicyRead,
						},
						{
							Name:       "222",
							Policy:     PolicyDeny,
							Intentions: PolicyDeny,
						},
					},
				}},
			},
			expected: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
					{
						Name:       "bar",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
					{
						Name:       "baz",
						Policy:     PolicyDeny,
						Intentions: PolicyDeny,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "000",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
					{
						Name:       "111",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
					{
						Name:       "222",
						Policy:     PolicyDeny,
						Intentions: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Sessions",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					Sessions: []*SessionRule{
						{
							Node:   "foo",
							Policy: PolicyWrite,
						},
						{
							Node:   "bar",
							Policy: PolicyRead,
						},
						{
							Node:   "baz",
							Policy: PolicyWrite,
						},
					},
					SessionPrefixes: []*SessionRule{
						{
							Node:   "000",
							Policy: PolicyWrite,
						},
						{
							Node:   "111",
							Policy: PolicyRead,
						},
						{
							Node:   "222",
							Policy: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					Sessions: []*SessionRule{
						{
							Node:   "foo",
							Policy: PolicyRead,
						},
						{
							Node:   "baz",
							Policy: PolicyDeny,
						},
					},
					SessionPrefixes: []*SessionRule{
						{
							Node:   "000",
							Policy: PolicyRead,
						},
						{
							Node:   "222",
							Policy: PolicyDeny,
						},
					},
				}},
			},
			expected: &Policy{PolicyRules: PolicyRules{
				Sessions: []*SessionRule{
					{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					{
						Node:   "bar",
						Policy: PolicyRead,
					},
					{
						Node:   "baz",
						Policy: PolicyDeny,
					},
				},
				SessionPrefixes: []*SessionRule{
					{
						Node:   "000",
						Policy: PolicyWrite,
					},
					{
						Node:   "111",
						Policy: PolicyRead,
					},
					{
						Node:   "222",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Prepared Queries",
			input: []*Policy{
				{PolicyRules: PolicyRules{
					PreparedQueries: []*PreparedQueryRule{
						{
							Prefix: "foo",
							Policy: PolicyWrite,
						},
						{
							Prefix: "bar",
							Policy: PolicyRead,
						},
						{
							Prefix: "baz",
							Policy: PolicyWrite,
						},
					},
					PreparedQueryPrefixes: []*PreparedQueryRule{
						{
							Prefix: "000",
							Policy: PolicyWrite,
						},
						{
							Prefix: "111",
							Policy: PolicyRead,
						},
						{
							Prefix: "222",
							Policy: PolicyWrite,
						},
					},
				}},
				{PolicyRules: PolicyRules{
					PreparedQueries: []*PreparedQueryRule{
						{
							Prefix: "foo",
							Policy: PolicyRead,
						},
						{
							Prefix: "baz",
							Policy: PolicyDeny,
						},
					},
					PreparedQueryPrefixes: []*PreparedQueryRule{
						{
							Prefix: "000",
							Policy: PolicyRead,
						},
						{
							Prefix: "222",
							Policy: PolicyDeny,
						},
					},
				}},
			},
			expected: &Policy{PolicyRules: PolicyRules{
				PreparedQueries: []*PreparedQueryRule{
					{
						Prefix: "foo",
						Policy: PolicyWrite,
					},
					{
						Prefix: "bar",
						Policy: PolicyRead,
					},
					{
						Prefix: "baz",
						Policy: PolicyDeny,
					},
				},
				PreparedQueryPrefixes: []*PreparedQueryRule{
					{
						Prefix: "000",
						Policy: PolicyWrite,
					},
					{
						Prefix: "111",
						Policy: PolicyRead,
					},
					{
						Prefix: "222",
						Policy: PolicyDeny,
					},
				},
			}},
		},
		{
			name: "Write Precedence",
			input: []*Policy{
				{
					PolicyRules: PolicyRules{
						ACL:      PolicyRead,
						Keyring:  PolicyRead,
						Operator: PolicyRead,
						Mesh:     PolicyRead,
						Peering:  PolicyRead,
					},
				},
				{
					PolicyRules: PolicyRules{
						ACL:      PolicyWrite,
						Keyring:  PolicyWrite,
						Operator: PolicyWrite,
						Mesh:     PolicyWrite,
						Peering:  PolicyWrite,
					},
				},
			},
			expected: &Policy{
				PolicyRules: PolicyRules{
					ACL:      PolicyWrite,
					Keyring:  PolicyWrite,
					Operator: PolicyWrite,
					Mesh:     PolicyWrite,
					Peering:  PolicyWrite,
				},
			},
		},
		{
			name: "Deny Precedence",
			input: []*Policy{
				{
					PolicyRules: PolicyRules{
						ACL:      PolicyWrite,
						Keyring:  PolicyWrite,
						Operator: PolicyWrite,
						Mesh:     PolicyWrite,
						Peering:  PolicyWrite,
					},
				},
				{
					PolicyRules: PolicyRules{
						ACL:      PolicyDeny,
						Keyring:  PolicyDeny,
						Operator: PolicyDeny,
						Mesh:     PolicyDeny,
						Peering:  PolicyDeny,
					},
				},
			},
			expected: &Policy{
				PolicyRules: PolicyRules{
					ACL:      PolicyDeny,
					Keyring:  PolicyDeny,
					Operator: PolicyDeny,
					Mesh:     PolicyDeny,
					Peering:  PolicyDeny,
				},
			},
		},
		{
			name: "Read Precedence",
			input: []*Policy{
				{
					PolicyRules: PolicyRules{
						ACL:      PolicyRead,
						Keyring:  PolicyRead,
						Operator: PolicyRead,
						Mesh:     PolicyRead,
						Peering:  PolicyRead,
					},
				},
				{},
			},
			expected: &Policy{
				PolicyRules: PolicyRules{
					ACL:      PolicyRead,
					Keyring:  PolicyRead,
					Operator: PolicyRead,
					Mesh:     PolicyRead,
					Peering:  PolicyRead,
				},
			},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			act := MergePolicies(tcase.input)
			exp := tcase.expected
			require.Equal(t, exp.ACL, act.ACL)
			require.Equal(t, exp.Keyring, act.Keyring)
			require.Equal(t, exp.Operator, act.Operator)
			require.Equal(t, exp.Mesh, act.Mesh)
			require.Equal(t, exp.Peering, act.Peering)
			require.ElementsMatch(t, exp.Agents, act.Agents)
			require.ElementsMatch(t, exp.AgentPrefixes, act.AgentPrefixes)
			require.ElementsMatch(t, exp.Events, act.Events)
			require.ElementsMatch(t, exp.EventPrefixes, act.EventPrefixes)
			require.ElementsMatch(t, exp.Keys, act.Keys)
			require.ElementsMatch(t, exp.KeyPrefixes, act.KeyPrefixes)
			require.ElementsMatch(t, exp.Nodes, act.Nodes)
			require.ElementsMatch(t, exp.NodePrefixes, act.NodePrefixes)
			require.ElementsMatch(t, exp.PreparedQueries, act.PreparedQueries)
			require.ElementsMatch(t, exp.PreparedQueryPrefixes, act.PreparedQueryPrefixes)
			require.ElementsMatch(t, exp.Services, act.Services)
			require.ElementsMatch(t, exp.ServicePrefixes, act.ServicePrefixes)
			require.ElementsMatch(t, exp.Sessions, act.Sessions)
			require.ElementsMatch(t, exp.SessionPrefixes, act.SessionPrefixes)
		})
	}

}

func TestPrecedence(t *testing.T) {
	type testCase struct {
		name     string
		a        string
		b        string
		expected bool
	}

	cases := []testCase{
		{
			name:     "Deny Over Write",
			a:        PolicyDeny,
			b:        PolicyWrite,
			expected: true,
		},
		{
			name:     "Deny Over List",
			a:        PolicyDeny,
			b:        PolicyList,
			expected: true,
		},
		{
			name:     "Deny Over Read",
			a:        PolicyDeny,
			b:        PolicyRead,
			expected: true,
		},
		{
			name:     "Deny Over Unknown",
			a:        PolicyDeny,
			b:        "not a policy",
			expected: true,
		},
		{
			name:     "Write Over List",
			a:        PolicyWrite,
			b:        PolicyList,
			expected: true,
		},
		{
			name:     "Write Over Read",
			a:        PolicyWrite,
			b:        PolicyRead,
			expected: true,
		},
		{
			name:     "Write Over Unknown",
			a:        PolicyWrite,
			b:        "not a policy",
			expected: true,
		},
		{
			name:     "List Over Read",
			a:        PolicyList,
			b:        PolicyRead,
			expected: true,
		},
		{
			name:     "List Over Unknown",
			a:        PolicyList,
			b:        "not a policy",
			expected: true,
		},
		{
			name:     "Read Over Unknown",
			a:        PolicyRead,
			b:        "not a policy",
			expected: true,
		},
		{
			name:     "Write Over Deny",
			a:        PolicyWrite,
			b:        PolicyDeny,
			expected: false,
		},
		{
			name:     "List Over Deny",
			a:        PolicyList,
			b:        PolicyDeny,
			expected: false,
		},
		{
			name:     "Read Over Deny",
			a:        PolicyRead,
			b:        PolicyDeny,
			expected: false,
		},
		{
			name:     "Deny Over Unknown",
			a:        PolicyDeny,
			b:        "not a policy",
			expected: true,
		},
		{
			name:     "List Over Write",
			a:        PolicyList,
			b:        PolicyWrite,
			expected: false,
		},
		{
			name:     "Read Over Write",
			a:        PolicyRead,
			b:        PolicyWrite,
			expected: false,
		},
		{
			name:     "Unknown Over Write",
			a:        "not a policy",
			b:        PolicyWrite,
			expected: false,
		},
		{
			name:     "Read Over List",
			a:        PolicyRead,
			b:        PolicyList,
			expected: false,
		},
		{
			name:     "Unknown Over List",
			a:        "not a policy",
			b:        PolicyList,
			expected: false,
		},
		{
			name:     "Unknown Over Read",
			a:        "not a policy",
			b:        PolicyRead,
			expected: false,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			require.Equal(t, tcase.expected, takesPrecedenceOver(tcase.a, tcase.b))
		})
	}
}
