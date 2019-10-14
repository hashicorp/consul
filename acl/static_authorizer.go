package acl

var (
	// allowAll is a singleton policy which allows all
	// non-management actions
	allowAll Authorizer = &StaticAuthorizer{
		allowManage:  false,
		defaultAllow: true,
	}

	// denyAll is a singleton policy which denies all actions
	denyAll Authorizer = &StaticAuthorizer{
		allowManage:  false,
		defaultAllow: false,
	}

	// manageAll is a singleton policy which allows all
	// actions, including management
	// TODO (acls) - Do we need to keep this around? Our config parsing doesn't allow
	// specifying a default "manage" policy so I believe nothing will every use this.
	manageAll Authorizer = &StaticAuthorizer{
		allowManage:  true,
		defaultAllow: true,
	}
)

// StaticAuthorizer is used to implement a base ACL policy. It either
// allows or denies all requests. This can be used as a parent
// ACL to act in a blacklist or whitelist mode.
type StaticAuthorizer struct {
	allowManage  bool
	defaultAllow bool
}

func (s *StaticAuthorizer) ACLRead(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) ACLWrite(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) AgentRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) AgentWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) EventRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) EventWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) IntentionDefaultAllow(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) IntentionRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) IntentionWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyList(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyWritePrefix(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyringRead(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) KeyringWrite(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) NodeRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) NodeWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) OperatorRead(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) OperatorWrite(*EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) PreparedQueryRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) PreparedQueryWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) ServiceRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) ServiceWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) SessionRead(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) SessionWrite(string, *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.defaultAllow {
		return Allow
	}
	return Deny
}

func (s *StaticAuthorizer) Snapshot(_ *EnterpriseAuthorizerContext) EnforcementDecision {
	if s.allowManage {
		return Allow
	}
	return Deny
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

// RootAuthorizer returns a possible Authorizer if the ID matches a root policy
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
