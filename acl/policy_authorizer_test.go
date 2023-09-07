// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"testing"

	"github.com/armon/go-radix"
	"github.com/stretchr/testify/require"
)

// Note that many of the policy authorizer tests still live in acl_test.go. These utilize a default policy or layer
// up multiple authorizers just like before the latest overhaul of the ACL package. To reduce the code diff and to
// ensure compatibility from version to version those tests have been only minimally altered. The tests in this
// file are specific to the newer functionality.
func TestPolicyAuthorizer(t *testing.T) {
	type aclCheck struct {
		name   string
		prefix string
		check  func(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext)
	}

	type aclTest struct {
		policy       *Policy
		authzContext *AuthorizerContext
		checks       []aclCheck
	}

	cases := map[string]aclTest{
		// This test ensures that if the policy doesn't define a rule then the policy authorizer will
		// return no concrete enforcement decision. This allows deferring to some defaults in another
		// authorizer including usage of a default overall policy of "deny"
		"Defaults": {
			policy: &Policy{},
			checks: []aclCheck{
				{name: "DefaultACLRead", prefix: "foo", check: checkDefaultACLRead},
				{name: "DefaultACLWrite", prefix: "foo", check: checkDefaultACLWrite},
				{name: "DefaultAgentRead", prefix: "foo", check: checkDefaultAgentRead},
				{name: "DefaultAgentWrite", prefix: "foo", check: checkDefaultAgentWrite},
				{name: "DefaultEventRead", prefix: "foo", check: checkDefaultEventRead},
				{name: "DefaultEventWrite", prefix: "foo", check: checkDefaultEventWrite},
				{name: "DefaultIntentionDefaultAllow", prefix: "foo", check: checkDefaultIntentionDefaultAllow},
				{name: "DefaultIntentionRead", prefix: "foo", check: checkDefaultIntentionRead},
				{name: "DefaultIntentionWrite", prefix: "foo", check: checkDefaultIntentionWrite},
				{name: "DefaultKeyRead", prefix: "foo", check: checkDefaultKeyRead},
				{name: "DefaultKeyList", prefix: "foo", check: checkDefaultKeyList},
				{name: "DefaultKeyringRead", prefix: "foo", check: checkDefaultKeyringRead},
				{name: "DefaultKeyringWrite", prefix: "foo", check: checkDefaultKeyringWrite},
				{name: "DefaultKeyWrite", prefix: "foo", check: checkDefaultKeyWrite},
				{name: "DefaultKeyWritePrefix", prefix: "foo", check: checkDefaultKeyWritePrefix},
				{name: "DefaultNodeRead", prefix: "foo", check: checkDefaultNodeRead},
				{name: "DefaultNodeWrite", prefix: "foo", check: checkDefaultNodeWrite},
				{name: "DefaultMeshRead", prefix: "foo", check: checkDefaultMeshRead},
				{name: "DefaultMeshWrite", prefix: "foo", check: checkDefaultMeshWrite},
				{name: "DefaultPeeringRead", prefix: "foo", check: checkDefaultPeeringRead},
				{name: "DefaultPeeringWrite", prefix: "foo", check: checkDefaultPeeringWrite},
				{name: "DefaultOperatorRead", prefix: "foo", check: checkDefaultOperatorRead},
				{name: "DefaultOperatorWrite", prefix: "foo", check: checkDefaultOperatorWrite},
				{name: "DefaultPreparedQueryRead", prefix: "foo", check: checkDefaultPreparedQueryRead},
				{name: "DefaultPreparedQueryWrite", prefix: "foo", check: checkDefaultPreparedQueryWrite},
				{name: "DefaultServiceRead", prefix: "foo", check: checkDefaultServiceRead},
				{name: "DefaultServiceWrite", prefix: "foo", check: checkDefaultServiceWrite},
				{name: "DefaultServiceWriteAny", prefix: "", check: checkDefaultServiceWriteAny},
				{name: "DefaultSessionRead", prefix: "foo", check: checkDefaultSessionRead},
				{name: "DefaultSessionWrite", prefix: "foo", check: checkDefaultSessionWrite},
				{name: "DefaultSnapshot", prefix: "foo", check: checkDefaultSnapshot},
			},
		},
		"Defaults - from peer": {
			policy:       &Policy{},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "DefaultNodeRead", prefix: "foo", check: checkDefaultNodeRead},
				{name: "DefaultServiceRead", prefix: "foo", check: checkDefaultServiceRead},
			},
		},
		"Peering - ServiceRead allowed with service:write": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "ServiceWriteAny", prefix: "imported-svc", check: checkAllowServiceRead},
			},
		},
		"Peering - ServiceRead allowed with service:read on all": {
			policy: &Policy{PolicyRules: PolicyRules{
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "ServiceReadAll", prefix: "imported-svc", check: checkAllowServiceRead},
			},
		},
		"Peering - ServiceRead not allowed with service:read on single service": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "same-name-as-imported",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "ServiceReadAll", prefix: "same-name-as-imported", check: checkDefaultServiceRead},
			},
		},
		"Peering - NodeRead allowed with service:write": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:   "foo",
						Policy: PolicyWrite,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "ServiceWriteAny", prefix: "imported-svc", check: checkAllowNodeRead},
			},
		},
		"Peering - NodeRead allowed with node:read on all": {
			policy: &Policy{PolicyRules: PolicyRules{
				NodePrefixes: []*NodeRule{
					{
						Name:   "",
						Policy: PolicyRead,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "NodeReadAll", prefix: "imported-svc", check: checkAllowNodeRead},
			},
		},
		"Peering - NodeRead not allowed with node:read on single service": {
			policy: &Policy{PolicyRules: PolicyRules{
				Nodes: []*NodeRule{
					{
						Name:   "same-name-as-imported",
						Policy: PolicyRead,
					},
				},
			}},
			authzContext: &AuthorizerContext{Peer: "some-peer"},
			checks: []aclCheck{
				{name: "NodeReadAll", prefix: "same-name-as-imported", check: checkDefaultNodeRead},
			},
		},
		"Prefer Exact Matches": {
			policy: &Policy{PolicyRules: PolicyRules{
				Agents: []*AgentRule{
					{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					{
						Node:   "football",
						Policy: PolicyDeny,
					},
				},
				AgentPrefixes: []*AgentRule{
					{
						Node:   "foot",
						Policy: PolicyRead,
					},
					{
						Node:   "fo",
						Policy: PolicyRead,
					},
				},
				Keys: []*KeyRule{
					{
						Prefix: "foo",
						Policy: PolicyWrite,
					},
					{
						Prefix: "football",
						Policy: PolicyDeny,
					},
				},
				KeyPrefixes: []*KeyRule{
					{
						Prefix: "foot",
						Policy: PolicyRead,
					},
					{
						Prefix: "fo",
						Policy: PolicyRead,
					},
				},
				Nodes: []*NodeRule{
					{
						Name:   "foo",
						Policy: PolicyWrite,
					},
					{
						Name:   "football",
						Policy: PolicyDeny,
					},
				},
				NodePrefixes: []*NodeRule{
					{
						Name:   "foot",
						Policy: PolicyRead,
					},
					{
						Name:   "fo",
						Policy: PolicyRead,
					},
				},
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
					{
						Name:   "football",
						Policy: PolicyDeny,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "foot",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
					{
						Name:       "fo",
						Policy:     PolicyRead,
						Intentions: PolicyRead,
					},
				},
				Sessions: []*SessionRule{
					{
						Node:   "foo",
						Policy: PolicyWrite,
					},
					{
						Node:   "football",
						Policy: PolicyDeny,
					},
				},
				SessionPrefixes: []*SessionRule{
					{
						Node:   "foot",
						Policy: PolicyRead,
					},
					{
						Node:   "fo",
						Policy: PolicyRead,
					},
				},
				Events: []*EventRule{
					{
						Event:  "foo",
						Policy: PolicyWrite,
					},
					{
						Event:  "football",
						Policy: PolicyDeny,
					},
				},
				EventPrefixes: []*EventRule{
					{
						Event:  "foot",
						Policy: PolicyRead,
					},
					{
						Event:  "fo",
						Policy: PolicyRead,
					},
				},
				PreparedQueries: []*PreparedQueryRule{
					{
						Prefix: "foo",
						Policy: PolicyWrite,
					},
					{
						Prefix: "football",
						Policy: PolicyDeny,
					},
				},
				PreparedQueryPrefixes: []*PreparedQueryRule{
					{
						Prefix: "foot",
						Policy: PolicyRead,
					},
					{
						Prefix: "fo",
						Policy: PolicyRead,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AgentReadPrefixAllowed", prefix: "fo", check: checkAllowAgentRead},
				{name: "AgentWritePrefixDenied", prefix: "fo", check: checkDenyAgentWrite},
				{name: "AgentReadPrefixAllowed", prefix: "for", check: checkAllowAgentRead},
				{name: "AgentWritePrefixDenied", prefix: "for", check: checkDenyAgentWrite},
				{name: "AgentReadAllowed", prefix: "foo", check: checkAllowAgentRead},
				{name: "AgentWriteAllowed", prefix: "foo", check: checkAllowAgentWrite},
				{name: "AgentReadPrefixAllowed", prefix: "foot", check: checkAllowAgentRead},
				{name: "AgentWritePrefixDenied", prefix: "foot", check: checkDenyAgentWrite},
				{name: "AgentReadPrefixAllowed", prefix: "foot2", check: checkAllowAgentRead},
				{name: "AgentWritePrefixDenied", prefix: "foot2", check: checkDenyAgentWrite},
				{name: "AgentReadPrefixAllowed", prefix: "food", check: checkAllowAgentRead},
				{name: "AgentWritePrefixDenied", prefix: "food", check: checkDenyAgentWrite},
				{name: "AgentReadDenied", prefix: "football", check: checkDenyAgentRead},
				{name: "AgentWriteDenied", prefix: "football", check: checkDenyAgentWrite},

				{name: "KeyReadPrefixAllowed", prefix: "fo", check: checkAllowKeyRead},
				{name: "KeyWritePrefixDenied", prefix: "fo", check: checkDenyKeyWrite},
				{name: "KeyReadPrefixAllowed", prefix: "for", check: checkAllowKeyRead},
				{name: "KeyWritePrefixDenied", prefix: "for", check: checkDenyKeyWrite},
				{name: "KeyReadAllowed", prefix: "foo", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "foo", check: checkAllowKeyWrite},
				{name: "KeyReadPrefixAllowed", prefix: "foot", check: checkAllowKeyRead},
				{name: "KeyWritePrefixDenied", prefix: "foot", check: checkDenyKeyWrite},
				{name: "KeyReadPrefixAllowed", prefix: "foot2", check: checkAllowKeyRead},
				{name: "KeyWritePrefixDenied", prefix: "foot2", check: checkDenyKeyWrite},
				{name: "KeyReadPrefixAllowed", prefix: "food", check: checkAllowKeyRead},
				{name: "KeyWritePrefixDenied", prefix: "food", check: checkDenyKeyWrite},
				{name: "KeyReadDenied", prefix: "football", check: checkDenyKeyRead},
				{name: "KeyWriteDenied", prefix: "football", check: checkDenyKeyWrite},

				{name: "NodeReadPrefixAllowed", prefix: "fo", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "fo", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "for", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "for", check: checkDenyNodeWrite},
				{name: "NodeReadAllowed", prefix: "foo", check: checkAllowNodeRead},
				{name: "NodeWriteAllowed", prefix: "foo", check: checkAllowNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "foot", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "foot", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "foot2", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "foot2", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "food", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "food", check: checkDenyNodeWrite},
				{name: "NodeReadDenied", prefix: "football", check: checkDenyNodeRead},
				{name: "NodeWriteDenied", prefix: "football", check: checkDenyNodeWrite},

				{name: "ServiceReadPrefixAllowed", prefix: "fo", check: checkAllowServiceRead},
				{name: "ServiceWritePrefixDenied", prefix: "fo", check: checkDenyServiceWrite},
				{name: "ServiceReadPrefixAllowed", prefix: "for", check: checkAllowServiceRead},
				{name: "ServiceWritePrefixDenied", prefix: "for", check: checkDenyServiceWrite},
				{name: "ServiceReadAllowed", prefix: "foo", check: checkAllowServiceRead},
				{name: "ServiceWriteAllowed", prefix: "foo", check: checkAllowServiceWrite},
				{name: "ServiceReadPrefixAllowed", prefix: "foot", check: checkAllowServiceRead},
				{name: "ServiceWritePrefixDenied", prefix: "foot", check: checkDenyServiceWrite},
				{name: "ServiceReadPrefixAllowed", prefix: "foot2", check: checkAllowServiceRead},
				{name: "ServiceWritePrefixDenied", prefix: "foot2", check: checkDenyServiceWrite},
				{name: "ServiceReadPrefixAllowed", prefix: "food", check: checkAllowServiceRead},
				{name: "ServiceWritePrefixDenied", prefix: "food", check: checkDenyServiceWrite},
				{name: "ServiceReadDenied", prefix: "football", check: checkDenyServiceRead},
				{name: "ServiceWriteDenied", prefix: "football", check: checkDenyServiceWrite},
				{name: "ServiceWriteAnyAllowed", prefix: "", check: checkAllowServiceWriteAny},

				{name: "NodeReadPrefixAllowed", prefix: "fo", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "fo", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "for", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "for", check: checkDenyNodeWrite},
				{name: "NodeReadAllowed", prefix: "foo", check: checkAllowNodeRead},
				{name: "NodeWriteAllowed", prefix: "foo", check: checkAllowNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "foot", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "foot", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "foot2", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "foot2", check: checkDenyNodeWrite},
				{name: "NodeReadPrefixAllowed", prefix: "food", check: checkAllowNodeRead},
				{name: "NodeWritePrefixDenied", prefix: "food", check: checkDenyNodeWrite},
				{name: "NodeReadDenied", prefix: "football", check: checkDenyNodeRead},
				{name: "NodeWriteDenied", prefix: "football", check: checkDenyNodeWrite},

				{name: "IntentionReadPrefixAllowed", prefix: "fo", check: checkAllowIntentionRead},
				{name: "IntentionWritePrefixDenied", prefix: "fo", check: checkDenyIntentionWrite},
				{name: "IntentionReadPrefixAllowed", prefix: "for", check: checkAllowIntentionRead},
				{name: "IntentionWritePrefixDenied", prefix: "for", check: checkDenyIntentionWrite},
				{name: "IntentionReadAllowed", prefix: "foo", check: checkAllowIntentionRead},
				{name: "IntentionWriteAllowed", prefix: "foo", check: checkAllowIntentionWrite},
				{name: "IntentionReadPrefixAllowed", prefix: "foot", check: checkAllowIntentionRead},
				{name: "IntentionWritePrefixDenied", prefix: "foot", check: checkDenyIntentionWrite},
				{name: "IntentionReadPrefixAllowed", prefix: "foot2", check: checkAllowIntentionRead},
				{name: "IntentionWritePrefixDenied", prefix: "foot2", check: checkDenyIntentionWrite},
				{name: "IntentionReadPrefixAllowed", prefix: "food", check: checkAllowIntentionRead},
				{name: "IntentionWritePrefixDenied", prefix: "food", check: checkDenyIntentionWrite},
				{name: "IntentionReadDenied", prefix: "football", check: checkDenyIntentionRead},
				{name: "IntentionWriteDenied", prefix: "football", check: checkDenyIntentionWrite},

				{name: "SessionReadPrefixAllowed", prefix: "fo", check: checkAllowSessionRead},
				{name: "SessionWritePrefixDenied", prefix: "fo", check: checkDenySessionWrite},
				{name: "SessionReadPrefixAllowed", prefix: "for", check: checkAllowSessionRead},
				{name: "SessionWritePrefixDenied", prefix: "for", check: checkDenySessionWrite},
				{name: "SessionReadAllowed", prefix: "foo", check: checkAllowSessionRead},
				{name: "SessionWriteAllowed", prefix: "foo", check: checkAllowSessionWrite},
				{name: "SessionReadPrefixAllowed", prefix: "foot", check: checkAllowSessionRead},
				{name: "SessionWritePrefixDenied", prefix: "foot", check: checkDenySessionWrite},
				{name: "SessionReadPrefixAllowed", prefix: "foot2", check: checkAllowSessionRead},
				{name: "SessionWritePrefixDenied", prefix: "foot2", check: checkDenySessionWrite},
				{name: "SessionReadPrefixAllowed", prefix: "food", check: checkAllowSessionRead},
				{name: "SessionWritePrefixDenied", prefix: "food", check: checkDenySessionWrite},
				{name: "SessionReadDenied", prefix: "football", check: checkDenySessionRead},
				{name: "SessionWriteDenied", prefix: "football", check: checkDenySessionWrite},

				{name: "EventReadPrefixAllowed", prefix: "fo", check: checkAllowEventRead},
				{name: "EventWritePrefixDenied", prefix: "fo", check: checkDenyEventWrite},
				{name: "EventReadPrefixAllowed", prefix: "for", check: checkAllowEventRead},
				{name: "EventWritePrefixDenied", prefix: "for", check: checkDenyEventWrite},
				{name: "EventReadAllowed", prefix: "foo", check: checkAllowEventRead},
				{name: "EventWriteAllowed", prefix: "foo", check: checkAllowEventWrite},
				{name: "EventReadPrefixAllowed", prefix: "foot", check: checkAllowEventRead},
				{name: "EventWritePrefixDenied", prefix: "foot", check: checkDenyEventWrite},
				{name: "EventReadPrefixAllowed", prefix: "foot2", check: checkAllowEventRead},
				{name: "EventWritePrefixDenied", prefix: "foot2", check: checkDenyEventWrite},
				{name: "EventReadPrefixAllowed", prefix: "food", check: checkAllowEventRead},
				{name: "EventWritePrefixDenied", prefix: "food", check: checkDenyEventWrite},
				{name: "EventReadDenied", prefix: "football", check: checkDenyEventRead},
				{name: "EventWriteDenied", prefix: "football", check: checkDenyEventWrite},

				{name: "PreparedQueryReadPrefixAllowed", prefix: "fo", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWritePrefixDenied", prefix: "fo", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadPrefixAllowed", prefix: "for", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWritePrefixDenied", prefix: "for", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "foo", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteAllowed", prefix: "foo", check: checkAllowPreparedQueryWrite},
				{name: "PreparedQueryReadPrefixAllowed", prefix: "foot", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWritePrefixDenied", prefix: "foot", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadPrefixAllowed", prefix: "foot2", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWritePrefixDenied", prefix: "foot2", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadPrefixAllowed", prefix: "food", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWritePrefixDenied", prefix: "food", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "football", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "football", check: checkDenyPreparedQueryWrite},
			},
		},
		"Intention Wildcards - prefix denied": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "",
						Policy:     PolicyDeny,
						Intentions: PolicyDeny,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AnyAllowed", prefix: "*", check: checkAllowIntentionRead},
				{name: "AllDenied", prefix: "*", check: checkDenyIntentionWrite},
			},
		},
		"Intention Wildcards - prefix allowed": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyDeny,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AnyAllowed", prefix: "*", check: checkAllowIntentionRead},
				{name: "AllDenied", prefix: "*", check: checkDenyIntentionWrite},
			},
		},
		"Intention Wildcards - all allowed": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
				ServicePrefixes: []*ServiceRule{
					{
						Name:       "",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AnyAllowed", prefix: "*", check: checkAllowIntentionRead},
				{name: "AllAllowed", prefix: "*", check: checkAllowIntentionWrite},
			},
		},
		"Intention Wildcards - all default": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyWrite,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AnyAllowed", prefix: "*", check: checkAllowIntentionRead},
				{name: "AllDefault", prefix: "*", check: checkDefaultIntentionWrite},
			},
		},
		"Intention Wildcards - any default": {
			policy: &Policy{PolicyRules: PolicyRules{
				Services: []*ServiceRule{
					{
						Name:       "foo",
						Policy:     PolicyWrite,
						Intentions: PolicyDeny,
					},
				},
			}},
			checks: []aclCheck{
				{name: "AnyDefault", prefix: "*", check: checkDefaultIntentionRead},
				{name: "AllDenied", prefix: "*", check: checkDenyIntentionWrite},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase
		t.Run(name, func(t *testing.T) {
			authz, err := NewPolicyAuthorizer([]*Policy{tcase.policy}, nil)
			require.NoError(t, err)

			for _, check := range tcase.checks {
				checkName := check.name
				if check.prefix != "" {
					checkName = fmt.Sprintf("%s.Prefix(%s)", checkName, check.prefix)
				}
				t.Run(checkName, func(t *testing.T) {
					check := check

					check.check(t, authz, check.prefix, tcase.authzContext)
				})
			}
		})
	}
}

func TestAnyAllowed(t *testing.T) {
	type radixInsertion struct {
		segment string
		value   *policyAuthorizerRadixLeaf
	}

	type testCase struct {
		insertions []radixInsertion

		readEnforcement  EnforcementDecision
		listEnforcement  EnforcementDecision
		writeEnforcement EnforcementDecision
	}

	cases := map[string]testCase{
		"no-rules-default": {
			readEnforcement:  Default,
			listEnforcement:  Default,
			writeEnforcement: Default,
		},
		"prefix-write-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				// this shouldn't affect whether anyAllowed returns things are allowed
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-list-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-read-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-write-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-deny-other-write-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessWrite},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-deny-other-list-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-list-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-read-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-read-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-deny-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny-other-deny-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			tree := radix.New()

			for _, insertion := range tcase.insertions {
				tree.Insert(insertion.segment, insertion.value)
			}

			var authz policyAuthorizer
			require.Equal(t, tcase.readEnforcement, authz.anyAllowed(tree, AccessRead))
			require.Equal(t, tcase.listEnforcement, authz.anyAllowed(tree, AccessList))
			require.Equal(t, tcase.writeEnforcement, authz.anyAllowed(tree, AccessWrite))
		})
	}
}

func TestAllAllowed(t *testing.T) {
	type radixInsertion struct {
		segment string
		value   *policyAuthorizerRadixLeaf
	}

	type testCase struct {
		insertions []radixInsertion

		readEnforcement  EnforcementDecision
		listEnforcement  EnforcementDecision
		writeEnforcement EnforcementDecision
	}

	cases := map[string]testCase{
		"no-rules-default": {
			readEnforcement:  Default,
			listEnforcement:  Default,
			writeEnforcement: Default,
		},
		"prefix-write-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-list-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-read-allowed": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-deny": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-write-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-allow-other-write-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessWrite},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Allow,
		},
		"prefix-allow-other-list-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-list-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessList},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Allow,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-read-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-read-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessRead},
					},
				},
			},
			readEnforcement:  Allow,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-deny-prefix": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
		"prefix-allow-other-deny-exact": {
			insertions: []radixInsertion{
				{
					segment: "",
					value: &policyAuthorizerRadixLeaf{
						prefix: &policyAuthorizerRule{access: AccessWrite},
					},
				},
				{
					segment: "foo",
					value: &policyAuthorizerRadixLeaf{
						exact: &policyAuthorizerRule{access: AccessDeny},
					},
				},
			},
			readEnforcement:  Deny,
			listEnforcement:  Deny,
			writeEnforcement: Deny,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			tree := radix.New()

			for _, insertion := range tcase.insertions {
				tree.Insert(insertion.segment, insertion.value)
			}

			var authz policyAuthorizer
			require.Equal(t, tcase.readEnforcement, authz.allAllowed(tree, AccessRead))
			require.Equal(t, tcase.listEnforcement, authz.allAllowed(tree, AccessList))
			require.Equal(t, tcase.writeEnforcement, authz.allAllowed(tree, AccessWrite))
		})
	}
}
