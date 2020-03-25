package agentpb

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

func TestEventEnforceACL(t *testing.T) {
	cases := []struct {
		Name     string
		Event    Event
		ACLRules string
		Want     acl.EnforcementDecision
	}{
		{
			Name:  "service health reg, blanket allow",
			Event: TestEventServiceHealthRegister(t, 1, "web"),
			ACLRules: `service_prefix "" {
				policy = "read"
			}
			node_prefix "" {
				policy = "read"
			}`,
			Want: acl.Allow,
		},
		{
			Name:  "service health reg, deny node",
			Event: TestEventServiceHealthRegister(t, 1, "web"),
			ACLRules: `service_prefix "" {
				policy = "read"
			}`,
			Want: acl.Deny,
		},
		{
			Name:  "service health reg, deny service",
			Event: TestEventServiceHealthRegister(t, 1, "web"),
			ACLRules: `node_prefix "" {
				policy = "read"
			}`,
			Want: acl.Deny,
		},

		{
			Name:     "internal ACL token updates denied",
			Event:    TestEventACLTokenUpdate(t),
			ACLRules: `acl = "write"`,
			Want:     acl.Deny,
		},
		{
			Name:     "internal ACL policy updates denied",
			Event:    TestEventACLPolicyUpdate(t),
			ACLRules: `acl = "write"`,
			Want:     acl.Deny,
		},
		{
			Name:     "internal ACL role updates denied",
			Event:    TestEventACLRoleUpdate(t),
			ACLRules: `acl = "write"`,
			Want:     acl.Deny,
		},

		{
			Name:     "internal EoS allowed",
			Event:    TestEventEndOfSnapshot(t, Topic_ServiceHealth, 100),
			ACLRules: ``, // No access to anything
			Want:     acl.Allow,
		},
		{
			Name:     "internal Resume allowed",
			Event:    TestEventResumeStream(t, Topic_ServiceHealth, 100),
			ACLRules: ``, // No access to anything
			Want:     acl.Allow,
		},
		{
			Name:     "internal Reset allowed",
			Event:    TestEventResetStream(t, Topic_ServiceHealth, 100),
			ACLRules: ``, // No access to anything
			Want:     acl.Allow,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			// Create an acl authorizer from the policy
			policy, err := acl.NewPolicyFromSource("", 0, tc.ACLRules, acl.SyntaxCurrent, nil, nil)
			require.NoError(t, err)

			authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"),
				[]*acl.Policy{policy}, nil)
			require.NoError(t, err)

			got := tc.Event.EnforceACL(authz)
			require.Equal(t, tc.Want, got)
		})
	}
}

func TestEventEnforceACLCoversAllTypes(t *testing.T) {
	authz := acl.RootAuthorizer("deny")
	for _, payload := range allEventTypes {
		e := Event{
			Topic:   Topic_ServiceHealth, // Just pick any topic for now.
			Index:   1234,
			Payload: payload,
		}

		// We don't actually care about the return type here - that's handled above,
		// just that it doesn't panic because of a undefined event type.
		e.EnforceACL(authz)
	}
}
