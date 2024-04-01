// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutils

import (
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func ACLAnonymous(t testutil.TestingTB) resolver.Result {
	t.Helper()

	return resolver.Result{
		Authorizer: acl.DenyAll(),
		ACLIdentity: &structs.ACLToken{
			AccessorID: acl.AnonymousTokenID,
		},
	}
}

func ACLsDisabled(t testutil.TestingTB) resolver.Result {
	t.Helper()

	return resolver.Result{
		Authorizer: acl.ManageAll(),
	}
}

func ACLNoPermissions(t testutil.TestingTB) resolver.Result {
	t.Helper()

	return resolver.Result{
		Authorizer:  acl.DenyAll(),
		ACLIdentity: randomACLIdentity(t),
	}
}

func ACLServiceWriteAny(t testutil.TestingTB) resolver.Result {
	t.Helper()

	policy, err := acl.NewPolicyFromSource(`
		service "foo" {
			policy = "write"
		}
	`, nil, nil)
	require.NoError(t, err)

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

func ACLServiceRead(t testutil.TestingTB, serviceName string) resolver.Result {
	t.Helper()

	aclRule := &acl.Policy{
		PolicyRules: acl.PolicyRules{
			Services: []*acl.ServiceRule{
				{
					Name:   serviceName,
					Policy: acl.PolicyRead,
				},
			},
		},
	}
	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{aclRule}, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

func ACLUseProvidedPolicy(t testutil.TestingTB, aclPolicy *acl.Policy) resolver.Result {
	t.Helper()

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{aclPolicy}, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

func ACLOperatorRead(t testutil.TestingTB) resolver.Result {
	t.Helper()

	aclRule := &acl.Policy{
		PolicyRules: acl.PolicyRules{
			Operator: acl.PolicyRead,
		},
	}
	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{aclRule}, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

func ACLOperatorWrite(t testutil.TestingTB) resolver.Result {
	t.Helper()

	aclRule := &acl.Policy{
		PolicyRules: acl.PolicyRules{
			Operator: acl.PolicyWrite,
		},
	}
	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{aclRule}, nil)
	require.NoError(t, err)

	return resolver.Result{
		Authorizer:  authz,
		ACLIdentity: randomACLIdentity(t),
	}
}

func randomACLIdentity(t testutil.TestingTB) structs.ACLIdentity {
	id, err := uuid.GenerateUUID()
	require.NoError(t, err)

	return &structs.ACLToken{AccessorID: id}
}
