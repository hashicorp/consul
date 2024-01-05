// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import "github.com/stretchr/testify/mock"

type MockAuthorizer struct {
	mock.Mock
}

func (m *MockAuthorizer) NamespaceRead(s string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(s, ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (m *MockAuthorizer) NamespaceWrite(s string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(s, ctx)
	return ret.Get(0).(EnforcementDecision)
}

var _ Authorizer = (*MockAuthorizer)(nil)

// ACLRead checks for permission to list all the ACLs
func (m *MockAuthorizer) ACLRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ACLWrite checks for permission to manipulate ACLs
func (m *MockAuthorizer) ACLWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// AgentRead checks for permission to read from agent endpoints for a
// given node.
func (m *MockAuthorizer) AgentRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// AgentWrite checks for permission to make changes via agent endpoints
// for a given node.
func (m *MockAuthorizer) AgentWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// EventRead determines if a specific event can be queried.
func (m *MockAuthorizer) EventRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// EventWrite determines if a specific event may be fired.
func (m *MockAuthorizer) EventWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionDefaultAllow determines the default authorized behavior
// when no intentions match a Connect request.
func (m *MockAuthorizer) IntentionDefaultAllow(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionRead determines if a specific intention can be read.
func (m *MockAuthorizer) IntentionRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// IntentionWrite determines if a specific intention can be
// created, modified, or deleted.
func (m *MockAuthorizer) IntentionWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyList checks for permission to list keys under a prefix
func (m *MockAuthorizer) KeyList(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyRead checks for permission to read a given key
func (m *MockAuthorizer) KeyRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyWrite checks for permission to write a given key
func (m *MockAuthorizer) KeyWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyWritePrefix checks for permission to write to an
// entire key prefix. This means there must be no sub-policies
// that deny a write.
func (m *MockAuthorizer) KeyWritePrefix(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyringRead determines if the encryption keyring used in
// the gossip layer can be read.
func (m *MockAuthorizer) KeyringRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// KeyringWrite determines if the keyring can be manipulated
func (m *MockAuthorizer) KeyringWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// NodeRead checks for permission to read (discover) a given node.
func (m *MockAuthorizer) NodeRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (m *MockAuthorizer) NodeReadAll(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// NodeWrite checks for permission to create or update (register) a
// given node.
func (m *MockAuthorizer) NodeWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (m *MockAuthorizer) MeshRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (m *MockAuthorizer) MeshWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PeeringRead determines if the read-only Consul peering functions
// can be used.
func (m *MockAuthorizer) PeeringRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PeeringWrite determines if the state-changing Consul peering
// functions can be used.
func (m *MockAuthorizer) PeeringWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// OperatorRead determines if the read-only Consul operator functions
// can be used.	ret := m.Called(segment, ctx)
func (m *MockAuthorizer) OperatorRead(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// OperatorWrite determines if the state-changing Consul operator
// functions can be used.
func (m *MockAuthorizer) OperatorWrite(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PreparedQueryRead determines if a specific prepared query can be read
// to show its contents (this is not used for execution).
func (m *MockAuthorizer) PreparedQueryRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// PreparedQueryWrite determines if a specific prepared query can be
// created, modified, or deleted.
func (m *MockAuthorizer) PreparedQueryWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ServiceRead checks for permission to read a given service
func (m *MockAuthorizer) ServiceRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (m *MockAuthorizer) ServiceReadAll(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ServiceWrite checks for permission to create or update a given
// service
func (m *MockAuthorizer) ServiceWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// ServiceWriteAny checks for service:write on any service
func (m *MockAuthorizer) ServiceWriteAny(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

// SessionRead checks for permission to read sessions for a given node.
func (m *MockAuthorizer) SessionRead(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// SessionWrite checks for permission to create sessions for a given
// node.
func (m *MockAuthorizer) SessionWrite(segment string, ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(segment, ctx)
	return ret.Get(0).(EnforcementDecision)
}

// Snapshot checks for permission to take and restore snapshots.
func (m *MockAuthorizer) Snapshot(ctx *AuthorizerContext) EnforcementDecision {
	ret := m.Called(ctx)
	return ret.Get(0).(EnforcementDecision)
}

func (p *MockAuthorizer) ToAllowAuthorizer() AllowAuthorizer {
	return AllowAuthorizer{Authorizer: p}
}
