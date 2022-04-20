package testutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

func TestAuthorizerServiceWriteAny(t *testing.T) acl.Authorizer {
	t.Helper()

	policy, err := acl.NewPolicyFromSource(`
		service "foo" {
			policy = "write"
		}
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	return authz
}

func TestAuthorizerServiceRead(t *testing.T, serviceName string) acl.Authorizer {
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

	return authz
}
