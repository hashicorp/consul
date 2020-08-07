package pbreplication

func (rs Status) String() string {
	switch rs {
	case Status_Ok:
		return "ok"
	case Status_Error:
		return "error"
	default:
		return "UNKNOWN"
	}
}

func (rt Type) String() string {
	switch rt {
	case Type_ACLTokens:
		return "acl-tokens"
	case Type_ACLPolicies:
		return "acl-policies"
	case Type_ACLRoles:
		return "acl-roles"
	case Type_LegacyACLs:
		return "legacy-acls"
	case Type_ConfigEntries:
		return "config-entries"
	case Type_Intentions:
		return "intentions"
	case Type_FederationState:
		return "federation-state"
	default:
		return "UNKNOWN"
	}
}
