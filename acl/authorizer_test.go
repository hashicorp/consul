package acl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestACL_Enforce(t *testing.T) {
	type testCase struct {
		method   string
		resource Resource
		segment  string
		access   string
		ret      EnforcementDecision
		err      string
	}

	testName := func(t testCase) string {
		if t.segment != "" {
			return fmt.Sprintf("%s/%s/%s/%s", t.resource, t.segment, t.access, t.ret.String())
		}
		return fmt.Sprintf("%s/%s/%s", t.resource, t.access, t.ret.String())
	}

	cases := []testCase{
		{
			method:   "ACLRead",
			resource: ResourceACL,
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "ACLRead",
			resource: ResourceACL,
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "ACLWrite",
			resource: ResourceACL,
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "ACLWrite",
			resource: ResourceACL,
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceACL,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "OperatorRead",
			resource: ResourceOperator,
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "OperatorRead",
			resource: ResourceOperator,
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "OperatorWrite",
			resource: ResourceOperator,
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "OperatorWrite",
			resource: ResourceOperator,
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceOperator,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "KeyringRead",
			resource: ResourceKeyring,
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "KeyringRead",
			resource: ResourceKeyring,
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "KeyringWrite",
			resource: ResourceKeyring,
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "KeyringWrite",
			resource: ResourceKeyring,
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceKeyring,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "AgentRead",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "AgentRead",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "AgentWrite",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "AgentWrite",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceAgent,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "EventRead",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "EventRead",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "EventWrite",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "EventWrite",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceEvent,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "IntentionRead",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "IntentionRead",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "IntentionWrite",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "IntentionWrite",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceIntention,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "NodeRead",
			resource: ResourceNode,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "NodeRead",
			resource: ResourceNode,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "NodeWrite",
			resource: ResourceNode,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "NodeWrite",
			resource: ResourceNode,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceNode,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "PeeringRead",
			resource: ResourcePeering,
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "PeeringRead",
			resource: ResourcePeering,
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "PeeringWrite",
			resource: ResourcePeering,
			access:   "write",
			ret:      Allow,
		},
		{
			method:   "PeeringWrite",
			resource: ResourcePeering,
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "PreparedQueryRead",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "PreparedQueryRead",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "PreparedQueryWrite",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "PreparedQueryWrite",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceQuery,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "ServiceRead",
			resource: ResourceService,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "ServiceRead",
			resource: ResourceService,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "ServiceWrite",
			resource: ResourceService,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "ServiceWrite",
			resource: ResourceService,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceSession,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "SessionRead",
			resource: ResourceSession,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "SessionRead",
			resource: ResourceSession,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "SessionWrite",
			resource: ResourceSession,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "SessionWrite",
			resource: ResourceSession,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			resource: ResourceSession,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			method:   "KeyRead",
			resource: ResourceKey,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		{
			method:   "KeyRead",
			resource: ResourceKey,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		{
			method:   "KeyWrite",
			resource: ResourceKey,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		{
			method:   "KeyWrite",
			resource: ResourceKey,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		{
			method:   "KeyList",
			resource: ResourceKey,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
		},
		{
			method:   "KeyList",
			resource: ResourceKey,
			segment:  "foo",
			access:   "list",
			ret:      Allow,
		},
		{
			resource: ResourceKey,
			segment:  "foo",
			access:   "deny",
			ret:      Deny,
			err:      "Invalid access level",
		},
		{
			resource: "not-a-real-resource",
			access:   "read",
			ret:      Deny,
			err:      "Invalid ACL resource requested:",
		},
	}

	for _, tcase := range cases {
		t.Run(testName(tcase), func(t *testing.T) {
			m := &MockAuthorizer{}

			if tcase.err == "" {
				var nilCtx *AuthorizerContext
				if tcase.segment != "" {
					m.On(tcase.method, tcase.segment, nilCtx).Return(tcase.ret)
				} else {
					m.On(tcase.method, nilCtx).Return(tcase.ret)
				}
			}

			ret, err := Enforce(m, tcase.resource, tcase.segment, tcase.access, nil)
			if tcase.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tcase.err)
			}
			require.Equal(t, tcase.ret, ret)
			m.AssertExpectations(t)
		})
	}
}
