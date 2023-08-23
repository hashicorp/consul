package testutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
)

func TestAuthorizerAllowAll(t *testing.T) resolver.Result {
	t.Helper()

	return resolver.Result{Authorizer: acl.AllowAll()}
}

func TestAuthorizerDenyAll(t *testing.T) resolver.Result {
	t.Helper()

	return resolver.Result{Authorizer: acl.DenyAll()}
}

func TestAuthorizerServiceWriteAny(t *testing.T) resolver.Result {
	t.Helper()

	policy, err := acl.NewPolicyFromSource(`
		service "foo" {
			policy = "write"
		}
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	return resolver.Result{Authorizer: authz}
}

func TestAuthorizerServiceRead(t *testing.T, serviceName string) resolver.Result {
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

	return resolver.Result{Authorizer: authz}
}
