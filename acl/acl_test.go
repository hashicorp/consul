package acl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func legacyPolicy(policy *Policy) *Policy {
	return &Policy{
		PolicyRules: PolicyRules{
			Agents:                policy.Agents,
			AgentPrefixes:         policy.Agents,
			Nodes:                 policy.Nodes,
			NodePrefixes:          policy.Nodes,
			Keys:                  policy.Keys,
			KeyPrefixes:           policy.Keys,
			Services:              policy.Services,
			ServicePrefixes:       policy.Services,
			Sessions:              policy.Sessions,
			SessionPrefixes:       policy.Sessions,
			Events:                policy.Events,
			EventPrefixes:         policy.Events,
			PreparedQueries:       policy.PreparedQueries,
			PreparedQueryPrefixes: policy.PreparedQueries,
			Keyring:               policy.Keyring,
			Operator:              policy.Operator,
		},
	}
}

//
// The following 1 line functions are created to all conform to what
// can be stored in the aclCheck type to make defining ACL tests
// nicer in the embedded struct within TestACL
//

func checkAllowACLRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.ACLRead(entCtx))
}

func checkAllowACLWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.ACLWrite(entCtx))
}

func checkAllowAgentRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.AgentRead(prefix, entCtx))
}

func checkAllowAgentWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.AgentWrite(prefix, entCtx))
}

func checkAllowEventRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.EventRead(prefix, entCtx))
}

func checkAllowEventWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.EventWrite(prefix, entCtx))
}

func checkAllowIntentionDefaultAllow(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.IntentionDefaultAllow(entCtx))
}

func checkAllowIntentionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.IntentionRead(prefix, entCtx))
}

func checkAllowIntentionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.IntentionWrite(prefix, entCtx))
}

func checkAllowKeyRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyRead(prefix, entCtx))
}

func checkAllowKeyList(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyList(prefix, entCtx))
}

func checkAllowKeyringRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyringRead(entCtx))
}

func checkAllowKeyringWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyringWrite(entCtx))
}

func checkAllowKeyWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyWrite(prefix, entCtx))
}

func checkAllowKeyWritePrefix(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.KeyWritePrefix(prefix, entCtx))
}

func checkAllowNodeRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.NodeRead(prefix, entCtx))
}

func checkAllowNodeWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.NodeWrite(prefix, entCtx))
}

func checkAllowOperatorRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.OperatorRead(entCtx))
}

func checkAllowOperatorWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.OperatorWrite(entCtx))
}

func checkAllowPreparedQueryRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.PreparedQueryRead(prefix, entCtx))
}

func checkAllowPreparedQueryWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.PreparedQueryWrite(prefix, entCtx))
}

func checkAllowServiceRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.ServiceRead(prefix, entCtx))
}

func checkAllowServiceWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.ServiceWrite(prefix, entCtx))
}

func checkAllowSessionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.SessionRead(prefix, entCtx))
}

func checkAllowSessionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.SessionWrite(prefix, entCtx))
}

func checkAllowSnapshot(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Allow, authz.Snapshot(entCtx))
}

func checkDenyACLRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.ACLRead(entCtx))
}

func checkDenyACLWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.ACLWrite(entCtx))
}

func checkDenyAgentRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.AgentRead(prefix, entCtx))
}

func checkDenyAgentWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.AgentWrite(prefix, entCtx))
}

func checkDenyEventRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.EventRead(prefix, entCtx))
}

func checkDenyEventWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.EventWrite(prefix, entCtx))
}

func checkDenyIntentionDefaultAllow(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.IntentionDefaultAllow(entCtx))
}

func checkDenyIntentionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.IntentionRead(prefix, entCtx))
}

func checkDenyIntentionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.IntentionWrite(prefix, entCtx))
}

func checkDenyKeyRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyRead(prefix, entCtx))
}

func checkDenyKeyList(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyList(prefix, entCtx))
}

func checkDenyKeyringRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyringRead(entCtx))
}

func checkDenyKeyringWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyringWrite(entCtx))
}

func checkDenyKeyWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyWrite(prefix, entCtx))
}

func checkDenyKeyWritePrefix(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.KeyWritePrefix(prefix, entCtx))
}

func checkDenyNodeRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.NodeRead(prefix, entCtx))
}

func checkDenyNodeWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.NodeWrite(prefix, entCtx))
}

func checkDenyOperatorRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.OperatorRead(entCtx))
}

func checkDenyOperatorWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.OperatorWrite(entCtx))
}

func checkDenyPreparedQueryRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.PreparedQueryRead(prefix, entCtx))
}

func checkDenyPreparedQueryWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.PreparedQueryWrite(prefix, entCtx))
}

func checkDenyServiceRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.ServiceRead(prefix, entCtx))
}

func checkDenyServiceWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.ServiceWrite(prefix, entCtx))
}

func checkDenySessionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.SessionRead(prefix, entCtx))
}

func checkDenySessionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.SessionWrite(prefix, entCtx))
}

func checkDenySnapshot(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Deny, authz.Snapshot(entCtx))
}

func checkDefaultACLRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.ACLRead(entCtx))
}

func checkDefaultACLWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.ACLWrite(entCtx))
}

func checkDefaultAgentRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.AgentRead(prefix, entCtx))
}

func checkDefaultAgentWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.AgentWrite(prefix, entCtx))
}

func checkDefaultEventRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.EventRead(prefix, entCtx))
}

func checkDefaultEventWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.EventWrite(prefix, entCtx))
}

func checkDefaultIntentionDefaultAllow(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.IntentionDefaultAllow(entCtx))
}

func checkDefaultIntentionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.IntentionRead(prefix, entCtx))
}

func checkDefaultIntentionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.IntentionWrite(prefix, entCtx))
}

func checkDefaultKeyRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyRead(prefix, entCtx))
}

func checkDefaultKeyList(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyList(prefix, entCtx))
}

func checkDefaultKeyringRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyringRead(entCtx))
}

func checkDefaultKeyringWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyringWrite(entCtx))
}

func checkDefaultKeyWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyWrite(prefix, entCtx))
}

func checkDefaultKeyWritePrefix(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.KeyWritePrefix(prefix, entCtx))
}

func checkDefaultNodeRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.NodeRead(prefix, entCtx))
}

func checkDefaultNodeWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.NodeWrite(prefix, entCtx))
}

func checkDefaultOperatorRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.OperatorRead(entCtx))
}

func checkDefaultOperatorWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.OperatorWrite(entCtx))
}

func checkDefaultPreparedQueryRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.PreparedQueryRead(prefix, entCtx))
}

func checkDefaultPreparedQueryWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.PreparedQueryWrite(prefix, entCtx))
}

func checkDefaultServiceRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.ServiceRead(prefix, entCtx))
}

func checkDefaultServiceWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.ServiceWrite(prefix, entCtx))
}

func checkDefaultSessionRead(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.SessionRead(prefix, entCtx))
}

func checkDefaultSessionWrite(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.SessionWrite(prefix, entCtx))
}

func checkDefaultSnapshot(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext) {
	require.Equal(t, Default, authz.Snapshot(entCtx))
}

func TestACL(t *testing.T) {
	type aclCheck struct {
		name   string
		prefix string
		check  func(t *testing.T, authz Authorizer, prefix string, entCtx *AuthorizerContext)
	}

	type aclTest struct {
		name          string
		defaultPolicy Authorizer
		policyStack   []*Policy
		checks        []aclCheck
	}

	tests := []aclTest{
		{
			name:          "DenyAll",
			defaultPolicy: DenyAll(),
			checks: []aclCheck{
				{name: "DenyACLRead", check: checkDenyACLRead},
				{name: "DenyACLWrite", check: checkDenyACLWrite},
				{name: "DenyAgentRead", check: checkDenyAgentRead},
				{name: "DenyAgentWrite", check: checkDenyAgentWrite},
				{name: "DenyEventRead", check: checkDenyEventRead},
				{name: "DenyEventWrite", check: checkDenyEventWrite},
				{name: "DenyIntentionDefaultAllow", check: checkDenyIntentionDefaultAllow},
				{name: "DenyIntentionRead", check: checkDenyIntentionRead},
				{name: "DenyIntentionWrite", check: checkDenyIntentionWrite},
				{name: "DenyKeyRead", check: checkDenyKeyRead},
				{name: "DenyKeyringRead", check: checkDenyKeyringRead},
				{name: "DenyKeyringWrite", check: checkDenyKeyringWrite},
				{name: "DenyKeyWrite", check: checkDenyKeyWrite},
				{name: "DenyNodeRead", check: checkDenyNodeRead},
				{name: "DenyNodeWrite", check: checkDenyNodeWrite},
				{name: "DenyOperatorRead", check: checkDenyOperatorRead},
				{name: "DenyOperatorWrite", check: checkDenyOperatorWrite},
				{name: "DenyPreparedQueryRead", check: checkDenyPreparedQueryRead},
				{name: "DenyPreparedQueryWrite", check: checkDenyPreparedQueryWrite},
				{name: "DenyServiceRead", check: checkDenyServiceRead},
				{name: "DenyServiceWrite", check: checkDenyServiceWrite},
				{name: "DenySessionRead", check: checkDenySessionRead},
				{name: "DenySessionWrite", check: checkDenySessionWrite},
				{name: "DenySnapshot", check: checkDenySnapshot},
			},
		},
		{
			name:          "AllowAll",
			defaultPolicy: AllowAll(),
			checks: []aclCheck{
				{name: "DenyACLRead", check: checkDenyACLRead},
				{name: "DenyACLWrite", check: checkDenyACLWrite},
				{name: "AllowAgentRead", check: checkAllowAgentRead},
				{name: "AllowAgentWrite", check: checkAllowAgentWrite},
				{name: "AllowEventRead", check: checkAllowEventRead},
				{name: "AllowEventWrite", check: checkAllowEventWrite},
				{name: "AllowIntentionDefaultAllow", check: checkAllowIntentionDefaultAllow},
				{name: "AllowIntentionRead", check: checkAllowIntentionRead},
				{name: "AllowIntentionWrite", check: checkAllowIntentionWrite},
				{name: "AllowKeyRead", check: checkAllowKeyRead},
				{name: "AllowKeyringRead", check: checkAllowKeyringRead},
				{name: "AllowKeyringWrite", check: checkAllowKeyringWrite},
				{name: "AllowKeyWrite", check: checkAllowKeyWrite},
				{name: "AllowNodeRead", check: checkAllowNodeRead},
				{name: "AllowNodeWrite", check: checkAllowNodeWrite},
				{name: "AllowOperatorRead", check: checkAllowOperatorRead},
				{name: "AllowOperatorWrite", check: checkAllowOperatorWrite},
				{name: "AllowPreparedQueryRead", check: checkAllowPreparedQueryRead},
				{name: "AllowPreparedQueryWrite", check: checkAllowPreparedQueryWrite},
				{name: "AllowServiceRead", check: checkAllowServiceRead},
				{name: "AllowServiceWrite", check: checkAllowServiceWrite},
				{name: "AllowSessionRead", check: checkAllowSessionRead},
				{name: "AllowSessionWrite", check: checkAllowSessionWrite},
				{name: "DenySnapshot", check: checkDenySnapshot},
			},
		},
		{
			name:          "ManageAll",
			defaultPolicy: ManageAll(),
			checks: []aclCheck{
				{name: "AllowACLRead", check: checkAllowACLRead},
				{name: "AllowACLWrite", check: checkAllowACLWrite},
				{name: "AllowAgentRead", check: checkAllowAgentRead},
				{name: "AllowAgentWrite", check: checkAllowAgentWrite},
				{name: "AllowEventRead", check: checkAllowEventRead},
				{name: "AllowEventWrite", check: checkAllowEventWrite},
				{name: "AllowIntentionDefaultAllow", check: checkAllowIntentionDefaultAllow},
				{name: "AllowIntentionRead", check: checkAllowIntentionRead},
				{name: "AllowIntentionWrite", check: checkAllowIntentionWrite},
				{name: "AllowKeyRead", check: checkAllowKeyRead},
				{name: "AllowKeyringRead", check: checkAllowKeyringRead},
				{name: "AllowKeyringWrite", check: checkAllowKeyringWrite},
				{name: "AllowKeyWrite", check: checkAllowKeyWrite},
				{name: "AllowNodeRead", check: checkAllowNodeRead},
				{name: "AllowNodeWrite", check: checkAllowNodeWrite},
				{name: "AllowOperatorRead", check: checkAllowOperatorRead},
				{name: "AllowOperatorWrite", check: checkAllowOperatorWrite},
				{name: "AllowPreparedQueryRead", check: checkAllowPreparedQueryRead},
				{name: "AllowPreparedQueryWrite", check: checkAllowPreparedQueryWrite},
				{name: "AllowServiceRead", check: checkAllowServiceRead},
				{name: "AllowServiceWrite", check: checkAllowServiceWrite},
				{name: "AllowSessionRead", check: checkAllowSessionRead},
				{name: "AllowSessionWrite", check: checkAllowSessionWrite},
				{name: "AllowSnapshot", check: checkAllowSnapshot},
			},
		},
		{
			name:          "AgentBasicDefaultDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "root",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadDenied", prefix: "ro", check: checkDenyAgentRead},
				{name: "DefaultWriteDenied", prefix: "ro", check: checkDenyAgentWrite},
				{name: "ROReadAllowed", prefix: "root", check: checkAllowAgentRead},
				{name: "ROWriteDenied", prefix: "root", check: checkDenyAgentWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro", check: checkAllowAgentRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro", check: checkDenyAgentWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowAgentRead},
				{name: "RWWriteDenied", prefix: "root-rw", check: checkAllowAgentWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-sub", check: checkAllowAgentRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-sub", check: checkAllowAgentWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyAgentRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyAgentWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-sub", check: checkDenyAgentRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-sub", check: checkDenyAgentWrite},
			},
		},
		{
			name:          "AgentBasicDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "root",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadDenied", prefix: "ro", check: checkAllowAgentRead},
				{name: "DefaultWriteDenied", prefix: "ro", check: checkAllowAgentWrite},
				{name: "ROReadAllowed", prefix: "root", check: checkAllowAgentRead},
				{name: "ROWriteDenied", prefix: "root", check: checkDenyAgentWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro", check: checkAllowAgentRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro", check: checkDenyAgentWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowAgentRead},
				{name: "RWWriteDenied", prefix: "root-rw", check: checkAllowAgentWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-sub", check: checkAllowAgentRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-sub", check: checkAllowAgentWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyAgentRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyAgentWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-sub", check: checkDenyAgentRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-sub", check: checkDenyAgentWrite},
			},
		},
		{
			name:          "PreparedQueryDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						PreparedQueries: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "other",
								Policy: PolicyDeny,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				// in version 1.2.1 and below this would have failed
				{name: "ReadAllowed", prefix: "foo", check: checkAllowPreparedQueryRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteAllowed", prefix: "foo", check: checkAllowPreparedQueryWrite},
				{name: "ReadDenied", prefix: "other", check: checkDenyPreparedQueryRead},
				{name: "WriteDenied", prefix: "other", check: checkDenyPreparedQueryWrite},
			},
		},
		{
			name:          "AgentNestedDefaultDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "root-ro",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
							&AgentRule{
								Node:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "child-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "child-ro",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "child-rw",
								Policy: PolicyWrite,
							},
							&AgentRule{
								Node:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadDenied", prefix: "nope", check: checkDenyAgentRead},
				{name: "DefaultWriteDenied", prefix: "nope", check: checkDenyAgentWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyAgentRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyAgentWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowAgentRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenyAgentWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowAgentRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowAgentWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenyAgentRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenyAgentWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowAgentRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenyAgentWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowAgentRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowAgentWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenyAgentRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenyAgentWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowAgentRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenyAgentWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowAgentRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowAgentWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenyAgentRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenyAgentWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowAgentRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenyAgentWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowAgentRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowAgentWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowAgentRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowAgentWrite},
			},
		},
		{
			name:          "AgentNestedDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "root-ro",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
							&AgentRule{
								Node:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "child-nope",
								Policy: PolicyDeny,
							},
							&AgentRule{
								Node:   "child-ro",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "child-rw",
								Policy: PolicyWrite,
							},
							&AgentRule{
								Node:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadAllowed", prefix: "nope", check: checkAllowAgentRead},
				{name: "DefaultWriteAllowed", prefix: "nope", check: checkAllowAgentWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyAgentRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyAgentWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowAgentRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenyAgentWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowAgentRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowAgentWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenyAgentRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenyAgentWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowAgentRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenyAgentWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowAgentRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowAgentWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenyAgentRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenyAgentWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowAgentRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenyAgentWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowAgentRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowAgentWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenyAgentRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenyAgentWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowAgentRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenyAgentWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowAgentRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowAgentWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowAgentRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowAgentWrite},
			},
		},
		{
			name:          "KeyringDefaultAllowPolicyDeny",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyDeny,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyKeyringRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteDenied", check: checkDenyKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultAllowPolicyRead",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyRead,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowKeyringRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteDenied", check: checkDenyKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultAllowPolicyWrite",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyWrite,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowKeyringRead},
				{name: "WriteAllowed", check: checkAllowKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultAllowPolicyNone",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowKeyringRead},
				{name: "WriteAllowed", check: checkAllowKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultDenyPolicyDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyDeny,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyKeyringRead},
				{name: "WriteDenied", check: checkDenyKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultDenyPolicyRead",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyRead,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowKeyringRead},
				{name: "WriteDenied", check: checkDenyKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultDenyPolicyWrite",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keyring: PolicyWrite,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowKeyringRead},
				{name: "WriteAllowed", check: checkAllowKeyringWrite},
			},
		},
		{
			name:          "KeyringDefaultDenyPolicyNone",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyKeyringRead},
				{name: "WriteDenied", check: checkDenyKeyringWrite},
			},
		},
		{
			name:          "OperatorDefaultAllowPolicyDeny",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyDeny,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyOperatorRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteDenied", check: checkDenyOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultAllowPolicyRead",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyRead,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowOperatorRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteDenied", check: checkDenyOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultAllowPolicyWrite",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyWrite,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowOperatorRead},
				{name: "WriteAllowed", check: checkAllowOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultAllowPolicyNone",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowOperatorRead},
				{name: "WriteAllowed", check: checkAllowOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultDenyPolicyDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyDeny,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyOperatorRead},
				{name: "WriteDenied", check: checkDenyOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultDenyPolicyRead",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyRead,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowOperatorRead},
				{name: "WriteDenied", check: checkDenyOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultDenyPolicyWrite",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Operator: PolicyWrite,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowOperatorRead},
				{name: "WriteAllowed", check: checkAllowOperatorWrite},
			},
		},
		{
			name:          "OperatorDefaultDenyPolicyNone",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{},
			},
			checks: []aclCheck{
				{name: "ReadDenied", check: checkDenyOperatorRead},
				{name: "WriteDenied", check: checkDenyOperatorWrite},
			},
		},
		{
			name:          "NodeDefaultDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Nodes: []*NodeRule{
							&NodeRule{
								Name:   "root-nope",
								Policy: PolicyDeny,
							},
							&NodeRule{
								Name:   "root-ro",
								Policy: PolicyRead,
							},
							&NodeRule{
								Name:   "root-rw",
								Policy: PolicyWrite,
							},
							&NodeRule{
								Name:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Nodes: []*NodeRule{
							&NodeRule{
								Name:   "child-nope",
								Policy: PolicyDeny,
							},
							&NodeRule{
								Name:   "child-ro",
								Policy: PolicyRead,
							},
							&NodeRule{
								Name:   "child-rw",
								Policy: PolicyWrite,
							},
							&NodeRule{
								Name:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadDenied", prefix: "nope", check: checkDenyNodeRead},
				{name: "DefaultWriteDenied", prefix: "nope", check: checkDenyNodeWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyNodeRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyNodeWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowNodeRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenyNodeWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowNodeRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowNodeWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenyNodeRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenyNodeWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowNodeRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenyNodeWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowNodeRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowNodeWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenyNodeRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenyNodeWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowNodeRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenyNodeWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowNodeRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowNodeWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenyNodeRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenyNodeWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowNodeRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenyNodeWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowNodeRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowNodeWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowNodeRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowNodeWrite},
			},
		},
		{
			name:          "NodeDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Nodes: []*NodeRule{
							&NodeRule{
								Name:   "root-nope",
								Policy: PolicyDeny,
							},
							&NodeRule{
								Name:   "root-ro",
								Policy: PolicyRead,
							},
							&NodeRule{
								Name:   "root-rw",
								Policy: PolicyWrite,
							},
							&NodeRule{
								Name:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Nodes: []*NodeRule{
							&NodeRule{
								Name:   "child-nope",
								Policy: PolicyDeny,
							},
							&NodeRule{
								Name:   "child-ro",
								Policy: PolicyRead,
							},
							&NodeRule{
								Name:   "child-rw",
								Policy: PolicyWrite,
							},
							&NodeRule{
								Name:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadAllowed", prefix: "nope", check: checkAllowNodeRead},
				{name: "DefaultWriteAllowed", prefix: "nope", check: checkAllowNodeWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenyNodeRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenyNodeWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowNodeRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenyNodeWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowNodeRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowNodeWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenyNodeRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenyNodeWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowNodeRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenyNodeWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowNodeRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowNodeWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenyNodeRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenyNodeWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowNodeRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenyNodeWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowNodeRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowNodeWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenyNodeRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenyNodeWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowNodeRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenyNodeWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowNodeRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowNodeWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowNodeRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowNodeWrite},
			},
		},
		{
			name:          "SessionDefaultDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Sessions: []*SessionRule{
							&SessionRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&SessionRule{
								Node:   "root-ro",
								Policy: PolicyRead,
							},
							&SessionRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
							&SessionRule{
								Node:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Sessions: []*SessionRule{
							&SessionRule{
								Node:   "child-nope",
								Policy: PolicyDeny,
							},
							&SessionRule{
								Node:   "child-ro",
								Policy: PolicyRead,
							},
							&SessionRule{
								Node:   "child-rw",
								Policy: PolicyWrite,
							},
							&SessionRule{
								Node:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadDenied", prefix: "nope", check: checkDenySessionRead},
				{name: "DefaultWriteDenied", prefix: "nope", check: checkDenySessionWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenySessionRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenySessionWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowSessionRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenySessionWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowSessionRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowSessionWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenySessionRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenySessionWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowSessionRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenySessionWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowSessionRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowSessionWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenySessionRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenySessionWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowSessionRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenySessionWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowSessionRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowSessionWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenySessionRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenySessionWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowSessionRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenySessionWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowSessionRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowSessionWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowSessionRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowSessionWrite},
			},
		},
		{
			name:          "SessionDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Sessions: []*SessionRule{
							&SessionRule{
								Node:   "root-nope",
								Policy: PolicyDeny,
							},
							&SessionRule{
								Node:   "root-ro",
								Policy: PolicyRead,
							},
							&SessionRule{
								Node:   "root-rw",
								Policy: PolicyWrite,
							},
							&SessionRule{
								Node:   "override",
								Policy: PolicyDeny,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Sessions: []*SessionRule{
							&SessionRule{
								Node:   "child-nope",
								Policy: PolicyDeny,
							},
							&SessionRule{
								Node:   "child-ro",
								Policy: PolicyRead,
							},
							&SessionRule{
								Node:   "child-rw",
								Policy: PolicyWrite,
							},
							&SessionRule{
								Node:   "override",
								Policy: PolicyWrite,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "DefaultReadAllowed", prefix: "nope", check: checkAllowSessionRead},
				{name: "DefaultWriteAllowed", prefix: "nope", check: checkAllowSessionWrite},
				{name: "DenyReadDenied", prefix: "root-nope", check: checkDenySessionRead},
				{name: "DenyWriteDenied", prefix: "root-nope", check: checkDenySessionWrite},
				{name: "ROReadAllowed", prefix: "root-ro", check: checkAllowSessionRead},
				{name: "ROWriteDenied", prefix: "root-ro", check: checkDenySessionWrite},
				{name: "RWReadAllowed", prefix: "root-rw", check: checkAllowSessionRead},
				{name: "RWWriteAllowed", prefix: "root-rw", check: checkAllowSessionWrite},
				{name: "DenySuffixReadDenied", prefix: "root-nope-prefix", check: checkDenySessionRead},
				{name: "DenySuffixWriteDenied", prefix: "root-nope-prefix", check: checkDenySessionWrite},
				{name: "ROSuffixReadAllowed", prefix: "root-ro-prefix", check: checkAllowSessionRead},
				{name: "ROSuffixWriteDenied", prefix: "root-ro-prefix", check: checkDenySessionWrite},
				{name: "RWSuffixReadAllowed", prefix: "root-rw-prefix", check: checkAllowSessionRead},
				{name: "RWSuffixWriteAllowed", prefix: "root-rw-prefix", check: checkAllowSessionWrite},
				{name: "ChildDenyReadDenied", prefix: "child-nope", check: checkDenySessionRead},
				{name: "ChildDenyWriteDenied", prefix: "child-nope", check: checkDenySessionWrite},
				{name: "ChildROReadAllowed", prefix: "child-ro", check: checkAllowSessionRead},
				{name: "ChildROWriteDenied", prefix: "child-ro", check: checkDenySessionWrite},
				{name: "ChildRWReadAllowed", prefix: "child-rw", check: checkAllowSessionRead},
				{name: "ChildRWWriteAllowed", prefix: "child-rw", check: checkAllowSessionWrite},
				{name: "ChildDenySuffixReadDenied", prefix: "child-nope-prefix", check: checkDenySessionRead},
				{name: "ChildDenySuffixWriteDenied", prefix: "child-nope-prefix", check: checkDenySessionWrite},
				{name: "ChildROSuffixReadAllowed", prefix: "child-ro-prefix", check: checkAllowSessionRead},
				{name: "ChildROSuffixWriteDenied", prefix: "child-ro-prefix", check: checkDenySessionWrite},
				{name: "ChildRWSuffixReadAllowed", prefix: "child-rw-prefix", check: checkAllowSessionRead},
				{name: "ChildRWSuffixWriteAllowed", prefix: "child-rw-prefix", check: checkAllowSessionWrite},
				{name: "ChildOverrideReadAllowed", prefix: "override", check: checkAllowSessionRead},
				{name: "ChildOverrideWriteAllowed", prefix: "override", check: checkAllowSessionWrite},
			},
		},
		{
			name:          "Parent",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo/",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "bar/",
								Policy: PolicyRead,
							},
						},
						PreparedQueries: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "other",
								Policy: PolicyWrite,
							},
							&PreparedQueryRule{
								Prefix: "foo",
								Policy: PolicyRead,
							},
						},
						Services: []*ServiceRule{
							&ServiceRule{
								Name:   "other",
								Policy: PolicyWrite,
							},
							&ServiceRule{
								Name:   "foo",
								Policy: PolicyRead,
							},
						},
					},
				}),
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo/priv/",
								Policy: PolicyRead,
							},
							&KeyRule{
								Prefix: "bar/",
								Policy: PolicyDeny,
							},
							&KeyRule{
								Prefix: "zip/",
								Policy: PolicyRead,
							},
						},
						PreparedQueries: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "bar",
								Policy: PolicyDeny,
							},
						},
						Services: []*ServiceRule{
							&ServiceRule{
								Name:   "bar",
								Policy: PolicyDeny,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "KeyReadDenied", prefix: "other", check: checkDenyKeyRead},
				{name: "KeyWriteDenied", prefix: "other", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "other", check: checkDenyKeyWritePrefix},
				{name: "KeyReadAllowed", prefix: "foo/test", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "foo/test", check: checkAllowKeyWrite},
				{name: "KeyWritePrefixAllowed", prefix: "foo/test", check: checkAllowKeyWritePrefix},
				{name: "KeyReadAllowed", prefix: "foo/priv/test", check: checkAllowKeyRead},
				{name: "KeyWriteDenied", prefix: "foo/priv/test", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "foo/priv/test", check: checkDenyKeyWritePrefix},
				{name: "KeyReadDenied", prefix: "bar/any", check: checkDenyKeyRead},
				{name: "KeyWriteDenied", prefix: "bar/any", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "bar/any", check: checkDenyKeyWritePrefix},
				{name: "KeyReadAllowed", prefix: "zip/test", check: checkAllowKeyRead},
				{name: "KeyWriteDenied", prefix: "zip/test", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "zip/test", check: checkDenyKeyWritePrefix},
				{name: "ServiceReadDenied", prefix: "fail", check: checkDenyServiceRead},
				{name: "ServiceWriteDenied", prefix: "fail", check: checkDenyServiceWrite},
				{name: "ServiceReadAllowed", prefix: "other", check: checkAllowServiceRead},
				{name: "ServiceWriteAllowed", prefix: "other", check: checkAllowServiceWrite},
				{name: "ServiceReadAllowed", prefix: "foo", check: checkAllowServiceRead},
				{name: "ServiceWriteDenied", prefix: "foo", check: checkDenyServiceWrite},
				{name: "ServiceReadDenied", prefix: "bar", check: checkDenyServiceRead},
				{name: "ServiceWriteDenied", prefix: "bar", check: checkDenyServiceWrite},
				{name: "PreparedQueryReadAllowed", prefix: "foo", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "foo", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "foobar", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "foobar", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "bar", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "bar", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "barbaz", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "barbaz", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "baz", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "baz", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "nope", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "nope", check: checkDenyPreparedQueryWrite},
				{name: "ACLReadDenied", check: checkDenyACLRead},
				{name: "ACLWriteDenied", check: checkDenyACLWrite},
				{name: "SnapshotDenied", check: checkDenySnapshot},
				{name: "IntentionDefaultAllowDenied", check: checkDenyIntentionDefaultAllow},
			},
		},
		{
			name:          "ComplexDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				legacyPolicy(&Policy{
					PolicyRules: PolicyRules{
						Events: []*EventRule{
							&EventRule{
								Event:  "",
								Policy: PolicyRead,
							},
							&EventRule{
								Event:  "foo",
								Policy: PolicyWrite,
							},
							&EventRule{
								Event:  "bar",
								Policy: PolicyDeny,
							},
						},
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo/",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "foo/priv/",
								Policy: PolicyDeny,
							},
							&KeyRule{
								Prefix: "bar/",
								Policy: PolicyDeny,
							},
							&KeyRule{
								Prefix: "zip/",
								Policy: PolicyRead,
							},
							&KeyRule{
								Prefix: "zap/",
								Policy: PolicyList,
							},
						},
						PreparedQueries: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "",
								Policy: PolicyRead,
							},
							&PreparedQueryRule{
								Prefix: "foo",
								Policy: PolicyWrite,
							},
							&PreparedQueryRule{
								Prefix: "bar",
								Policy: PolicyDeny,
							},
							&PreparedQueryRule{
								Prefix: "zoo",
								Policy: PolicyWrite,
							},
						},
						Services: []*ServiceRule{
							&ServiceRule{
								Name:   "",
								Policy: PolicyWrite,
							},
							&ServiceRule{
								Name:   "foo",
								Policy: PolicyRead,
							},
							&ServiceRule{
								Name:   "bar",
								Policy: PolicyDeny,
							},
							&ServiceRule{
								Name:       "barfoo",
								Policy:     PolicyWrite,
								Intentions: PolicyWrite,
							},
							&ServiceRule{
								Name:       "intbaz",
								Policy:     PolicyWrite,
								Intentions: PolicyDeny,
							},
						},
					},
				}),
			},
			checks: []aclCheck{
				{name: "KeyReadAllowed", prefix: "other", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "other", check: checkAllowKeyWrite},
				{name: "KeyWritePrefixAllowed", prefix: "other", check: checkAllowKeyWritePrefix},
				{name: "KeyListAllowed", prefix: "other", check: checkAllowKeyList},
				{name: "KeyReadAllowed", prefix: "foo/test", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "foo/test", check: checkAllowKeyWrite},
				{name: "KeyWritePrefixAllowed", prefix: "foo/test", check: checkAllowKeyWritePrefix},
				{name: "KeyListAllowed", prefix: "foo/test", check: checkAllowKeyList},
				{name: "KeyReadDenied", prefix: "foo/priv/test", check: checkDenyKeyRead},
				{name: "KeyWriteDenied", prefix: "foo/priv/test", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "foo/priv/test", check: checkDenyKeyWritePrefix},
				{name: "KeyListDenied", prefix: "foo/priv/test", check: checkDenyKeyList},
				{name: "KeyReadDenied", prefix: "bar/any", check: checkDenyKeyRead},
				{name: "KeyWriteDenied", prefix: "bar/any", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "bar/any", check: checkDenyKeyWritePrefix},
				{name: "KeyListDenied", prefix: "bar/any", check: checkDenyKeyList},
				{name: "KeyReadAllowed", prefix: "zip/test", check: checkAllowKeyRead},
				{name: "KeyWriteDenied", prefix: "zip/test", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "zip/test", check: checkDenyKeyWritePrefix},
				{name: "KeyListDenied", prefix: "zip/test", check: checkDenyKeyList},
				{name: "KeyReadAllowed", prefix: "foo/", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "foo/", check: checkAllowKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "foo/", check: checkDenyKeyWritePrefix},
				{name: "KeyListAllowed", prefix: "foo/", check: checkAllowKeyList},
				{name: "KeyReadAllowed", prefix: "", check: checkAllowKeyRead},
				{name: "KeyWriteAllowed", prefix: "", check: checkAllowKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "", check: checkDenyKeyWritePrefix},
				{name: "KeyListAllowed", prefix: "", check: checkAllowKeyList},
				{name: "KeyReadAllowed", prefix: "zap/test", check: checkAllowKeyRead},
				{name: "KeyWriteDenied", prefix: "zap/test", check: checkDenyKeyWrite},
				{name: "KeyWritePrefixDenied", prefix: "zap/test", check: checkDenyKeyWritePrefix},
				{name: "KeyListAllowed", prefix: "zap/test", check: checkAllowKeyList},
				{name: "IntentionReadAllowed", prefix: "other", check: checkAllowIntentionRead},
				{name: "IntentionWriteDenied", prefix: "other", check: checkDenyIntentionWrite},
				{name: "IntentionReadAllowed", prefix: "foo", check: checkAllowIntentionRead},
				{name: "IntentionWriteDenied", prefix: "foo", check: checkDenyIntentionWrite},
				{name: "IntentionReadDenied", prefix: "bar", check: checkDenyIntentionRead},
				{name: "IntentionWriteDenied", prefix: "bar", check: checkDenyIntentionWrite},
				{name: "IntentionReadAllowed", prefix: "foobar", check: checkAllowIntentionRead},
				{name: "IntentionWriteDenied", prefix: "foobar", check: checkDenyIntentionWrite},
				{name: "IntentionReadDenied", prefix: "barfo", check: checkDenyIntentionRead},
				{name: "IntentionWriteDenied", prefix: "barfo", check: checkDenyIntentionWrite},
				{name: "IntentionReadAllowed", prefix: "barfoo", check: checkAllowIntentionRead},
				{name: "IntentionWriteAllowed", prefix: "barfoo", check: checkAllowIntentionWrite},
				{name: "IntentionReadAllowed", prefix: "barfoo2", check: checkAllowIntentionRead},
				{name: "IntentionWriteAllowed", prefix: "barfoo2", check: checkAllowIntentionWrite},
				{name: "IntentionReadDenied", prefix: "intbaz", check: checkDenyIntentionRead},
				{name: "IntentionWriteDenied", prefix: "intbaz", check: checkDenyIntentionWrite},
				{name: "IntentionDefaultAllowAllowed", check: checkAllowIntentionDefaultAllow},
				{name: "ServiceReadAllowed", prefix: "other", check: checkAllowServiceRead},
				{name: "ServiceWriteAllowed", prefix: "other", check: checkAllowServiceWrite},
				{name: "ServiceReadAllowed", prefix: "foo", check: checkAllowServiceRead},
				{name: "ServiceWriteDenied", prefix: "foo", check: checkDenyServiceWrite},
				{name: "ServiceReadDenied", prefix: "bar", check: checkDenyServiceRead},
				{name: "ServiceWriteDenied", prefix: "bar", check: checkDenyServiceWrite},
				{name: "ServiceReadAllowed", prefix: "foobar", check: checkAllowServiceRead},
				{name: "ServiceWriteDenied", prefix: "foobar", check: checkDenyServiceWrite},
				{name: "ServiceReadDenied", prefix: "barfo", check: checkDenyServiceRead},
				{name: "ServiceWriteDenied", prefix: "barfo", check: checkDenyServiceWrite},
				{name: "ServiceReadAllowed", prefix: "barfoo", check: checkAllowServiceRead},
				{name: "ServiceWriteAllowed", prefix: "barfoo", check: checkAllowServiceWrite},
				{name: "ServiceReadAllowed", prefix: "barfoo2", check: checkAllowServiceRead},
				{name: "ServiceWriteAllowed", prefix: "barfoo2", check: checkAllowServiceWrite},
				{name: "EventReadAllowed", prefix: "foo", check: checkAllowEventRead},
				{name: "EventWriteAllowed", prefix: "foo", check: checkAllowEventWrite},
				{name: "EventReadAllowed", prefix: "foobar", check: checkAllowEventRead},
				{name: "EventWriteAllowed", prefix: "foobar", check: checkAllowEventWrite},
				{name: "EventReadDenied", prefix: "bar", check: checkDenyEventRead},
				{name: "EventWriteDenied", prefix: "bar", check: checkDenyEventWrite},
				{name: "EventReadDenied", prefix: "barbaz", check: checkDenyEventRead},
				{name: "EventWriteDenied", prefix: "barbaz", check: checkDenyEventWrite},
				{name: "EventReadAllowed", prefix: "baz", check: checkAllowEventRead},
				{name: "EventWriteDenied", prefix: "baz", check: checkDenyEventWrite},
				{name: "PreparedQueryReadAllowed", prefix: "foo", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteAllowed", prefix: "foo", check: checkAllowPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "foobar", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteAllowed", prefix: "foobar", check: checkAllowPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "bar", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "bar", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadDenied", prefix: "barbaz", check: checkDenyPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "barbaz", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "baz", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "baz", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "nope", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteDenied", prefix: "nope", check: checkDenyPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "zoo", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteAllowed", prefix: "zoo", check: checkAllowPreparedQueryWrite},
				{name: "PreparedQueryReadAllowed", prefix: "zookeeper", check: checkAllowPreparedQueryRead},
				{name: "PreparedQueryWriteAllowed", prefix: "zookeeper", check: checkAllowPreparedQueryWrite},
			},
		},
		{
			name:          "ExactMatchPrecedence",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Agents: []*AgentRule{
							&AgentRule{
								Node:   "foo",
								Policy: PolicyWrite,
							},
							&AgentRule{
								Node:   "football",
								Policy: PolicyDeny,
							},
						},
						AgentPrefixes: []*AgentRule{
							&AgentRule{
								Node:   "foot",
								Policy: PolicyRead,
							},
							&AgentRule{
								Node:   "fo",
								Policy: PolicyRead,
							},
						},
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "football",
								Policy: PolicyDeny,
							},
						},
						KeyPrefixes: []*KeyRule{
							&KeyRule{
								Prefix: "foot",
								Policy: PolicyRead,
							},
							&KeyRule{
								Prefix: "fo",
								Policy: PolicyRead,
							},
						},
						Nodes: []*NodeRule{
							&NodeRule{
								Name:   "foo",
								Policy: PolicyWrite,
							},
							&NodeRule{
								Name:   "football",
								Policy: PolicyDeny,
							},
						},
						NodePrefixes: []*NodeRule{
							&NodeRule{
								Name:   "foot",
								Policy: PolicyRead,
							},
							&NodeRule{
								Name:   "fo",
								Policy: PolicyRead,
							},
						},
						Services: []*ServiceRule{
							&ServiceRule{
								Name:       "foo",
								Policy:     PolicyWrite,
								Intentions: PolicyWrite,
							},
							&ServiceRule{
								Name:   "football",
								Policy: PolicyDeny,
							},
						},
						ServicePrefixes: []*ServiceRule{
							&ServiceRule{
								Name:       "foot",
								Policy:     PolicyRead,
								Intentions: PolicyRead,
							},
							&ServiceRule{
								Name:       "fo",
								Policy:     PolicyRead,
								Intentions: PolicyRead,
							},
						},
						Sessions: []*SessionRule{
							&SessionRule{
								Node:   "foo",
								Policy: PolicyWrite,
							},
							&SessionRule{
								Node:   "football",
								Policy: PolicyDeny,
							},
						},
						SessionPrefixes: []*SessionRule{
							&SessionRule{
								Node:   "foot",
								Policy: PolicyRead,
							},
							&SessionRule{
								Node:   "fo",
								Policy: PolicyRead,
							},
						},
						Events: []*EventRule{
							&EventRule{
								Event:  "foo",
								Policy: PolicyWrite,
							},
							&EventRule{
								Event:  "football",
								Policy: PolicyDeny,
							},
						},
						EventPrefixes: []*EventRule{
							&EventRule{
								Event:  "foot",
								Policy: PolicyRead,
							},
							&EventRule{
								Event:  "fo",
								Policy: PolicyRead,
							},
						},
						PreparedQueries: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "foo",
								Policy: PolicyWrite,
							},
							&PreparedQueryRule{
								Prefix: "football",
								Policy: PolicyDeny,
							},
						},
						PreparedQueryPrefixes: []*PreparedQueryRule{
							&PreparedQueryRule{
								Prefix: "foot",
								Policy: PolicyRead,
							},
							&PreparedQueryRule{
								Prefix: "fo",
								Policy: PolicyRead,
							},
						},
					},
				},
			},
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
		{
			name:          "ACLRead",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						ACL: PolicyRead,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowACLRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteDenied", check: checkDenyACLWrite},
			},
		},
		{
			name:          "ACLRead",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						ACL: PolicyWrite,
					},
				},
			},
			checks: []aclCheck{
				{name: "ReadAllowed", check: checkAllowACLRead},
				// in version 1.2.1 and below this would have failed
				{name: "WriteAllowed", check: checkAllowACLWrite},
			},
		},
		{
			name:          "KeyWritePrefixDefaultDeny",
			defaultPolicy: DenyAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						KeyPrefixes: []*KeyRule{
							&KeyRule{
								Prefix: "fo",
								Policy: PolicyRead,
							},
							&KeyRule{
								Prefix: "foo/",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "bar/",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "baz/",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "test/",
								Policy: PolicyWrite,
							},
						},
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo/bar",
								Policy: PolicyWrite,
							},
							&KeyRule{
								Prefix: "bar/baz",
								Policy: PolicyRead,
							},
						},
					},
				},
			},
			checks: []aclCheck{
				// Ensure we deny access if the best match key_prefix rule does not grant
				// write access (disregards both the default policy and any other rules with
				// segments that may fall within the given kv prefix)
				{name: "DeniedTopLevelPrefix", prefix: "foo", check: checkDenyKeyWritePrefix},
				// Allow recursive KV writes when we have a prefix rule that allows it and no
				// other rules with segments that fall within the requested kv prefix to be written.
				{name: "AllowedTopLevelPrefix", prefix: "baz/", check: checkAllowKeyWritePrefix},
				// Ensure we allow recursive KV writes when we have a prefix rule that would allow it
				// and all other rules with segments prefixed by the kv prefix to be written also allow
				// write access.
				{name: "AllowedPrefixWithNestedWrite", prefix: "foo/", check: checkAllowKeyWritePrefix},
				// Ensure that we deny recursive KV writes when they would be allowed for a prefix but
				// denied by either an exact match rule or prefix match rule for a segment prefixed by
				// the kv prefix being checked against.
				{name: "DenyPrefixWithNestedRead", prefix: "bar/", check: checkDenyKeyWritePrefix},
				// Ensure that the default deny policy is used when there is no key_prefix rule
				// for the given kv segment regardless of any rules that would grant write access
				// to segments prefixed by the kv prefix being checked against.
				{name: "DenyNoPrefixMatch", prefix: "te", check: checkDenyKeyWritePrefix},
			},
		},
		{
			name:          "KeyWritePrefixDefaultAllow",
			defaultPolicy: AllowAll(),
			policyStack: []*Policy{
				&Policy{
					PolicyRules: PolicyRules{
						Keys: []*KeyRule{
							&KeyRule{
								Prefix: "foo/bar",
								Policy: PolicyRead,
							},
						},
					},
				},
			},
			checks: []aclCheck{
				// Ensure that we deny a key prefix write when a rule for a key within our prefix
				// doesn't allow writing and the default policy is to allow
				{name: "KeyWritePrefixDenied", prefix: "foo", check: checkDenyKeyWritePrefix},
				// Ensure that the default allow policy is used when there is no prefix rule
				// and there are no other rules regarding keys within that prefix to be written.
				{name: "KeyWritePrefixAllowed", prefix: "bar", check: checkAllowKeyWritePrefix},
			},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			acl := tcase.defaultPolicy
			for _, policy := range tcase.policyStack {
				newACL, err := NewPolicyAuthorizerWithDefaults(acl, []*Policy{policy}, nil)
				require.NoError(t, err)
				acl = newACL
			}

			for _, check := range tcase.checks {
				checkName := check.name
				if check.prefix != "" {
					checkName = fmt.Sprintf("%s.Prefix(%s)", checkName, check.prefix)
				}
				t.Run(checkName, func(t *testing.T) {
					check.check(t, acl, check.prefix, nil)
				})
			}
		})
	}
}

func TestRootAuthorizer(t *testing.T) {
	require.Equal(t, AllowAll(), RootAuthorizer("allow"))
	require.Equal(t, DenyAll(), RootAuthorizer("deny"))
	require.Equal(t, ManageAll(), RootAuthorizer("manage"))
	require.Nil(t, RootAuthorizer("foo"))
}

func TestACLEnforce(t *testing.T) {
	type enforceTest struct {
		name     string
		rule     AccessLevel
		required AccessLevel
		expected EnforcementDecision
	}

	tests := []enforceTest{
		{
			name:     "RuleNoneRequireRead",
			rule:     AccessUnknown,
			required: AccessRead,
			expected: Default,
		},
		{
			name:     "RuleNoneRequireWrite",
			rule:     AccessUnknown,
			required: AccessWrite,
			expected: Default,
		},
		{
			name:     "RuleNoneRequireList",
			rule:     AccessUnknown,
			required: AccessList,
			expected: Default,
		},
		{
			name:     "RuleReadRequireRead",
			rule:     AccessRead,
			required: AccessRead,
			expected: Allow,
		},
		{
			name:     "RuleReadRequireWrite",
			rule:     AccessRead,
			required: AccessWrite,
			expected: Deny,
		},
		{
			name:     "RuleReadRequireList",
			rule:     AccessRead,
			required: AccessList,
			expected: Deny,
		},
		{
			name:     "RuleListRequireRead",
			rule:     AccessList,
			required: AccessRead,
			expected: Allow,
		},
		{
			name:     "RuleListRequireWrite",
			rule:     AccessList,
			required: AccessWrite,
			expected: Deny,
		},
		{
			name:     "RuleListRequireList",
			rule:     AccessList,
			required: AccessList,
			expected: Allow,
		},
		{
			name:     "RuleWritetRequireRead",
			rule:     AccessWrite,
			required: AccessRead,
			expected: Allow,
		},
		{
			name:     "RuleWritetRequireWrite",
			rule:     AccessWrite,
			required: AccessWrite,
			expected: Allow,
		},
		{
			name:     "RuleWritetRequireList",
			rule:     AccessWrite,
			required: AccessList,
			expected: Allow,
		},
		{
			name:     "RuleDenyRequireRead",
			rule:     AccessDeny,
			required: AccessRead,
			expected: Deny,
		},
		{
			name:     "RuleDenyRequireWrite",
			rule:     AccessDeny,
			required: AccessWrite,
			expected: Deny,
		},
		{
			name:     "RuleDenyRequireList",
			rule:     AccessDeny,
			required: AccessList,
			expected: Deny,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			require.Equal(t, tcase.expected, enforce(tcase.rule, tcase.required))
		})
	}
}
