package acl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAuthorizer struct {
	mock.Mock
}

// ACLRead checks for permission to list all the ACLs
func (m *mockAuthorizer) ACLRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ACLWrite checks for permission to manipulate ACLs
func (m *mockAuthorizer) ACLWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// AgentRead checks for permission to read from agent endpoints for a
// given node.
func (m *mockAuthorizer) AgentRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// AgentWrite checks for permission to make changes via agent endpoints
// for a given node.
func (m *mockAuthorizer) AgentWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// EventRead determines if a specific event can be queried.
func (m *mockAuthorizer) EventRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// EventWrite determines if a specific event may be fired.
func (m *mockAuthorizer) EventWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionDefaultAllow determines the default authorized behavior
// when no intentions match a Connect request.
func (m *mockAuthorizer) IntentionDefaultAllow(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionRead determines if a specific intention can be read.
func (m *mockAuthorizer) IntentionRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionWrite determines if a specific intention can be
// created, modified, or deleted.
func (m *mockAuthorizer) IntentionWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyList checks for permission to list keys under a prefix
func (m *mockAuthorizer) KeyList(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyRead checks for permission to read a given key
func (m *mockAuthorizer) KeyRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyWrite checks for permission to write a given key
func (m *mockAuthorizer) KeyWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyWritePrefix checks for permission to write to an
// entire key prefix. This means there must be no sub-policies
// that deny a write.
func (m *mockAuthorizer) KeyWritePrefix(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyringRead determines if the encryption keyring used in
// the gossip layer can be read.
func (m *mockAuthorizer) KeyringRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyringWrite determines if the keyring can be manipulated
func (m *mockAuthorizer) KeyringWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// NodeRead checks for permission to read (discover) a given node.
func (m *mockAuthorizer) NodeRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// NodeWrite checks for permission to create or update (register) a
// given node.
func (m *mockAuthorizer) NodeWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// OperatorRead determines if the read-only Consul operator functions
// can be used.	ret := m.Called(segment, ctx)
func (m *mockAuthorizer) OperatorRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// OperatorWrite determines if the state-changing Consul operator
// functions can be used.
func (m *mockAuthorizer) OperatorWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PreparedQueryRead determines if a specific prepared query can be read
// to show its contents (this is not used for execution).
func (m *mockAuthorizer) PreparedQueryRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PreparedQueryWrite determines if a specific prepared query can be
// created, modified, or deleted.
func (m *mockAuthorizer) PreparedQueryWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ServiceRead checks for permission to read a given service
func (m *mockAuthorizer) ServiceRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ServiceWrite checks for permission to create or update a given
// service
func (m *mockAuthorizer) ServiceWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// SessionRead checks for permission to read sessions for a given node.
func (m *mockAuthorizer) SessionRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// SessionWrite checks for permission to create sessions for a given
// node.
func (m *mockAuthorizer) SessionWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// Snapshot checks for permission to take and restore snapshots.
func (m *mockAuthorizer) Snapshot(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

func TestACL_Enforce(t *testing.T) {
	t.Parallel()

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
		testCase{
			method:   "ACLRead",
			resource: ResourceACL,
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "ACLRead",
			resource: ResourceACL,
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "ACLWrite",
			resource: ResourceACL,
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "ACLWrite",
			resource: ResourceACL,
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceACL,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "OperatorRead",
			resource: ResourceOperator,
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "OperatorRead",
			resource: ResourceOperator,
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "OperatorWrite",
			resource: ResourceOperator,
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "OperatorWrite",
			resource: ResourceOperator,
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceOperator,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "KeyringRead",
			resource: ResourceKeyring,
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "KeyringRead",
			resource: ResourceKeyring,
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "KeyringWrite",
			resource: ResourceKeyring,
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "KeyringWrite",
			resource: ResourceKeyring,
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceKeyring,
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "AgentRead",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "AgentRead",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "AgentWrite",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "AgentWrite",
			resource: ResourceAgent,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceAgent,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "EventRead",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "EventRead",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "EventWrite",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "EventWrite",
			resource: ResourceEvent,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceEvent,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "IntentionRead",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "IntentionRead",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "IntentionWrite",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "IntentionWrite",
			resource: ResourceIntention,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceIntention,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "NodeRead",
			resource: ResourceNode,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "NodeRead",
			resource: ResourceNode,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "NodeWrite",
			resource: ResourceNode,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "NodeWrite",
			resource: ResourceNode,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceNode,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "PreparedQueryRead",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "PreparedQueryRead",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "PreparedQueryWrite",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "PreparedQueryWrite",
			resource: ResourceQuery,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceQuery,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "ServiceRead",
			resource: ResourceService,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "ServiceRead",
			resource: ResourceService,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "ServiceWrite",
			resource: ResourceService,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "ServiceWrite",
			resource: ResourceService,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceSession,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "SessionRead",
			resource: ResourceSession,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "SessionRead",
			resource: ResourceSession,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "SessionWrite",
			resource: ResourceSession,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "SessionWrite",
			resource: ResourceSession,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			resource: ResourceSession,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			method:   "KeyRead",
			resource: ResourceKey,
			segment:  "foo",
			access:   "read",
			ret:      Deny,
		},
		testCase{
			method:   "KeyRead",
			resource: ResourceKey,
			segment:  "foo",
			access:   "read",
			ret:      Allow,
		},
		testCase{
			method:   "KeyWrite",
			resource: ResourceKey,
			segment:  "foo",
			access:   "write",
			ret:      Deny,
		},
		testCase{
			method:   "KeyWrite",
			resource: ResourceKey,
			segment:  "foo",
			access:   "write",
			ret:      Allow,
		},
		testCase{
			method:   "KeyList",
			resource: ResourceKey,
			segment:  "foo",
			access:   "list",
			ret:      Deny,
		},
		testCase{
			method:   "KeyList",
			resource: ResourceKey,
			segment:  "foo",
			access:   "list",
			ret:      Allow,
		},
		testCase{
			resource: ResourceKey,
			segment:  "foo",
			access:   "deny",
			ret:      Deny,
			err:      "Invalid access level",
		},
		testCase{
			resource: "not-a-real-resource",
			access:   "read",
			ret:      Deny,
			err:      "Invalid ACL resource requested:",
		},
	}

	for _, tcase := range cases {
		t.Run(testName(tcase), func(t *testing.T) {
			m := &mockAuthorizer{}

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
