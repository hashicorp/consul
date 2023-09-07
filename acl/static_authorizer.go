// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

var (
	// allowAll is a singleton policy which allows all
	// non-management actions
	allowAll Authorizer = &staticAuthorizer{
		allowManage:  false,
		defaultAllow: true,
	}

	// denyAll is a singleton policy which denies all actions
	denyAll Authorizer = &staticAuthorizer{
		allowManage:  false,
		defaultAllow: false,
	}

	// manageAll is a singleton policy which allows all
	// actions, including management
	manageAll Authorizer = &staticAuthorizer{
		allowManage:  true,
		defaultAllow: true,
	}
)

// StaticAuthorizer is used to implement a base ACL policy. It either
// allows or denies all requests. This can be used as a parent
// ACL to act in a denylist or allowlist mode.
type staticAuthorizer struct {
	allowManage  bool
	defaultAllow bool
}

func (s *staticAuthorizer) ACLRead(*AuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ACLWrite(*AuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) AgentRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) AgentWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) EventRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) EventWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) IntentionDefaultAllow(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) IntentionRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) IntentionWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyList(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyWritePrefix(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyringRead(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) KeyringWrite(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) NodeRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) NodeReadAll(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) NodeWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) MeshRead(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) MeshWrite(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) PeeringRead(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) PeeringWrite(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) OperatorRead(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) OperatorWrite(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) PreparedQueryRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) PreparedQueryWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ServiceRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ServiceReadAll(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ServiceWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ServiceWriteAny(*AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) SessionRead(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) SessionWrite(string, *AuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) Snapshot(_ *AuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
}

func (s *staticAuthorizer) ToAllowAuthorizer() AllowAuthorizer {
	return AllowAuthorizer{Authorizer: s}
}

// AllowAll returns an Authorizer that allows all operations
func AllowAll() Authorizer {
	return allowAll
}

// DenyAll returns an Authorizer that denies all operations
func DenyAll() Authorizer {
	return denyAll
}

// ManageAll returns an Authorizer that can manage all resources
func ManageAll() Authorizer {
	return manageAll
}

// RootAuthorizer returns a possible Authorizer if the ID matches a root policy.
//
// TODO: rename this function. While the returned authorizer is used as a root
// authorizer in some cases, in others it is not. A more appropriate name might
// be NewAuthorizerFromPolicyName.
func RootAuthorizer(id string) Authorizer {
	switch id {
	case "allow":
		return allowAll
	case "deny":
		return denyAll
	case "manage":
		return manageAll
	default:
		return nil
	}
}
