package connectca

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

// testAuthorizer returns an ACL policy authorizer with `service:write` on an
// arbitrary service.
func testAuthorizer(t *testing.T) acl.Authorizer {
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
