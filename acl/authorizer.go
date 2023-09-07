// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"strings"
)

type EnforcementDecision int

const (
	// Deny returned from an Authorizer enforcement method indicates
	// that a corresponding rule was found and that access should be denied
	Deny EnforcementDecision = iota
	// Allow returned from an Authorizer enforcement method indicates
	// that a corresponding rule was found and that access should be allowed
	Allow
	// Default returned from an Authorizer enforcement method indicates
	// that a corresponding rule was not found and that whether access
	// should be granted or denied should be deferred to the default
	// access level
	Default
)

func (d EnforcementDecision) String() string {
	switch d {
	case Allow:
		return "Allow"
	case Deny:
		return "Deny"
	case Default:
		return "Default"
	default:
		return "Unknown"
	}
}

type Resource string

const (
	ResourceACL       Resource = "acl"
	ResourceAgent     Resource = "agent"
	ResourceEvent     Resource = "event"
	ResourceIntention Resource = "intention"
	ResourceKey       Resource = "key"
	ResourceKeyring   Resource = "keyring"
	ResourceNode      Resource = "node"
	ResourceOperator  Resource = "operator"
	ResourceMesh      Resource = "mesh"
	ResourceQuery     Resource = "query"
	ResourceService   Resource = "service"
	ResourceSession   Resource = "session"
	ResourcePeering   Resource = "peering"
)

// Authorizer is the interface for policy enforcement.
type Authorizer interface {
	// ACLRead checks for permission to list all the ACLs
	ACLRead(*AuthorizerContext) EnforcementDecision

	// ACLWrite checks for permission to manipulate ACLs
	ACLWrite(*AuthorizerContext) EnforcementDecision

	// AgentRead checks for permission to read from agent endpoints for a
	// given node.
	AgentRead(string, *AuthorizerContext) EnforcementDecision

	// AgentWrite checks for permission to make changes via agent endpoints
	// for a given node.
	AgentWrite(string, *AuthorizerContext) EnforcementDecision

	// EventRead determines if a specific event can be queried.
	EventRead(string, *AuthorizerContext) EnforcementDecision

	// EventWrite determines if a specific event may be fired.
	EventWrite(string, *AuthorizerContext) EnforcementDecision

	// IntentionDefaultAllow determines the default authorized behavior
	// when no intentions match a Connect request.
	IntentionDefaultAllow(*AuthorizerContext) EnforcementDecision

	// IntentionRead determines if a specific intention can be read.
	IntentionRead(string, *AuthorizerContext) EnforcementDecision

	// IntentionWrite determines if a specific intention can be
	// created, modified, or deleted.
	IntentionWrite(string, *AuthorizerContext) EnforcementDecision

	// KeyList checks for permission to list keys under a prefix
	KeyList(string, *AuthorizerContext) EnforcementDecision

	// KeyRead checks for permission to read a given key
	KeyRead(string, *AuthorizerContext) EnforcementDecision

	// KeyWrite checks for permission to write a given key
	KeyWrite(string, *AuthorizerContext) EnforcementDecision

	// KeyWritePrefix checks for permission to write to an
	// entire key prefix. This means there must be no sub-policies
	// that deny a write.
	KeyWritePrefix(string, *AuthorizerContext) EnforcementDecision

	// KeyringRead determines if the encryption keyring used in
	// the gossip layer can be read.
	KeyringRead(*AuthorizerContext) EnforcementDecision

	// KeyringWrite determines if the keyring can be manipulated
	KeyringWrite(*AuthorizerContext) EnforcementDecision

	// MeshRead determines if the read-only Consul mesh functions
	// can be used.
	MeshRead(*AuthorizerContext) EnforcementDecision

	// MeshWrite determines if the state-changing Consul mesh
	// functions can be used.
	MeshWrite(*AuthorizerContext) EnforcementDecision

	// PeeringRead determines if the read-only Consul peering functions
	// can be used.
	PeeringRead(*AuthorizerContext) EnforcementDecision

	// PeeringWrite determines if the stage-changing Consul peering
	// functions can be used.
	PeeringWrite(*AuthorizerContext) EnforcementDecision

	// NodeRead checks for permission to read (discover) a given node.
	NodeRead(string, *AuthorizerContext) EnforcementDecision

	// NodeReadAll checks for permission to read (discover) all nodes.
	NodeReadAll(*AuthorizerContext) EnforcementDecision

	// NodeWrite checks for permission to create or update (register) a
	// given node.
	NodeWrite(string, *AuthorizerContext) EnforcementDecision

	// OperatorRead determines if the read-only Consul operator functions
	// can be used.
	OperatorRead(*AuthorizerContext) EnforcementDecision

	// OperatorWrite determines if the state-changing Consul operator
	// functions can be used.
	OperatorWrite(*AuthorizerContext) EnforcementDecision

	// PreparedQueryRead determines if a specific prepared query can be read
	// to show its contents (this is not used for execution).
	PreparedQueryRead(string, *AuthorizerContext) EnforcementDecision

	// PreparedQueryWrite determines if a specific prepared query can be
	// created, modified, or deleted.
	PreparedQueryWrite(string, *AuthorizerContext) EnforcementDecision

	// ServiceRead checks for permission to read a given service
	ServiceRead(string, *AuthorizerContext) EnforcementDecision

	// ServiceReadAll checks for permission to read all services
	ServiceReadAll(*AuthorizerContext) EnforcementDecision

	// ServiceWrite checks for permission to create or update a given
	// service
	ServiceWrite(string, *AuthorizerContext) EnforcementDecision

	// ServiceWriteAny checks for write permission on any service
	ServiceWriteAny(*AuthorizerContext) EnforcementDecision

	// SessionRead checks for permission to read sessions for a given node.
	SessionRead(string, *AuthorizerContext) EnforcementDecision

	// SessionWrite checks for permission to create sessions for a given
	// node.
	SessionWrite(string, *AuthorizerContext) EnforcementDecision

	// Snapshot checks for permission to take and restore snapshots.
	Snapshot(*AuthorizerContext) EnforcementDecision

	// Embedded Interface for Consul Enterprise specific ACL enforcement
	enterpriseAuthorizer

	// ToAllowAuthorizer is needed until we can use ResolveResult in all the places this interface is used.
	ToAllowAuthorizer() AllowAuthorizer
}

// AllowAuthorizer is a wrapper to expose the *Allowed methods.
// This and the ToAllowAuthorizer function exist to tide us over until the ResolveResult struct
// is moved into acl.
type AllowAuthorizer struct {
	Authorizer
	AccessorID string
}

// ACLReadAllowed checks for permission to list all the ACLs
func (a AllowAuthorizer) ACLReadAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.ACLRead(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceACL, AccessRead)
	}
	return nil
}

// ACLWriteAllowed checks for permission to manipulate ACLs
func (a AllowAuthorizer) ACLWriteAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.ACLWrite(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceACL, AccessWrite)
	}
	return nil
}

// AgentReadAllowed checks for permission to read from agent endpoints for a
// given node.
func (a AllowAuthorizer) AgentReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.AgentRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceAgent, AccessRead, name)
	}
	return nil
}

// AgentWriteAllowed checks for permission to make changes via agent endpoints
// for a given node.
func (a AllowAuthorizer) AgentWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.AgentWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceAgent, AccessWrite, name)
	}
	return nil
}

// EventReadAllowed determines if a specific event can be queried.
func (a AllowAuthorizer) EventReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.EventRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceEvent, AccessRead, name)
	}
	return nil
}

// EventWriteAllowed determines if a specific event may be fired.
func (a AllowAuthorizer) EventWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.EventWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceEvent, AccessWrite, name)
	}
	return nil
}

// IntentionDefaultAllowAllowed determines the default authorized behavior
// when no intentions match a Connect request.
func (a AllowAuthorizer) IntentionDefaultAllowAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.IntentionDefaultAllow(ctx) != Allow {
		// This is a bit nuanced, in that this isn't set by a rule, but inherited globally
		// TODO(acl-error-enhancements) revisit when we have full accessor info
		return PermissionDeniedError{Cause: "Denied by intention default"}
	}
	return nil
}

// IntentionReadAllowed determines if a specific intention can be read.
func (a AllowAuthorizer) IntentionReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.IntentionRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceIntention, AccessRead, name)
	}
	return nil
}

// IntentionWriteAllowed determines if a specific intention can be
// created, modified, or deleted.
func (a AllowAuthorizer) IntentionWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.IntentionWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceIntention, AccessWrite, name)
	}
	return nil
}

// KeyListAllowed checks for permission to list keys under a prefix
func (a AllowAuthorizer) KeyListAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.KeyList(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceKey, AccessList, name)
	}
	return nil
}

// KeyReadAllowed checks for permission to read a given key
func (a AllowAuthorizer) KeyReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.KeyRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceKey, AccessRead, name)
	}
	return nil
}

// KeyWriteAllowed checks for permission to write a given key
func (a AllowAuthorizer) KeyWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.KeyWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceKey, AccessWrite, name)
	}
	return nil
}

// KeyWritePrefixAllowed checks for permission to write to an
// entire key prefix. This means there must be no sub-policies
// that deny a write.
func (a AllowAuthorizer) KeyWritePrefixAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.KeyWritePrefix(name, ctx) != Allow {
		// TODO(acl-error-enhancements) revisit this message; we may need to do some extra plumbing inside of KeyWritePrefix to
		// return properly detailed information.
		return PermissionDeniedByACL(a, ctx, ResourceKey, AccessWrite, name)
	}
	return nil
}

// KeyringReadAllowed determines if the encryption keyring used in
// the gossip layer can be read.
func (a AllowAuthorizer) KeyringReadAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.KeyringRead(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceKeyring, AccessRead)
	}
	return nil
}

// KeyringWriteAllowed determines if the keyring can be manipulated
func (a AllowAuthorizer) KeyringWriteAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.KeyringWrite(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceKeyring, AccessWrite)
	}
	return nil
}

// MeshReadAllowed determines if the read-only Consul mesh functions
// can be used.
func (a AllowAuthorizer) MeshReadAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.MeshRead(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceMesh, AccessRead)
	}
	return nil
}

// MeshWriteAllowed determines if the state-changing Consul mesh
// functions can be used.
func (a AllowAuthorizer) MeshWriteAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.MeshWrite(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceMesh, AccessWrite)
	}
	return nil
}

// PeeringReadAllowed determines if the read-only Consul peering functions
// can be used.
func (a AllowAuthorizer) PeeringReadAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.PeeringRead(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourcePeering, AccessRead)
	}
	return nil
}

// PeeringWriteAllowed determines if the state-changing Consul peering
// functions can be used.
func (a AllowAuthorizer) PeeringWriteAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.PeeringWrite(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourcePeering, AccessWrite)
	}
	return nil
}

// NodeReadAllowed checks for permission to read (discover) a given node.
func (a AllowAuthorizer) NodeReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.NodeRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceNode, AccessRead, name)
	}
	return nil
}

// NodeReadAllAllowed checks for permission to read (discover) all nodes.
func (a AllowAuthorizer) NodeReadAllAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.NodeReadAll(ctx) != Allow {
		// This is only used to gate certain UI functions right now (e.g metrics)
		return PermissionDeniedByACL(a, ctx, ResourceNode, AccessRead, "all nodes")
	}
	return nil
}

// NodeWriteAllowed checks for permission to create or update (register) a
// given node.
func (a AllowAuthorizer) NodeWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.NodeWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceNode, AccessWrite, name)
	}
	return nil
}

// OperatorReadAllowed determines if the read-only Consul operator functions
// can be used.
func (a AllowAuthorizer) OperatorReadAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.OperatorRead(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceOperator, AccessRead)
	}
	return nil
}

// OperatorWriteAllowed determines if the state-changing Consul operator
// functions can be used.
func (a AllowAuthorizer) OperatorWriteAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.OperatorWrite(ctx) != Allow {
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceOperator, AccessWrite)
	}
	return nil
}

// PreparedQueryReadAllowed determines if a specific prepared query can be read
// to show its contents (this is not used for execution).
func (a AllowAuthorizer) PreparedQueryReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.PreparedQueryRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceQuery, AccessRead, name)
	}
	return nil
}

// PreparedQueryWriteAllowed determines if a specific prepared query can be
// created, modified, or deleted.
func (a AllowAuthorizer) PreparedQueryWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.PreparedQueryWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceQuery, AccessWrite, name)
	}
	return nil
}

// ServiceReadAllowed checks for permission to read a given service
func (a AllowAuthorizer) ServiceReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.ServiceRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceService, AccessRead, name)
	}
	return nil
}

// ServiceReadAllAllowed checks for permission to read all services
func (a AllowAuthorizer) ServiceReadAllAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.ServiceReadAll(ctx) != Allow {
		// This is only used to gate certain UI functions right now (e.g metrics)
		return PermissionDeniedByACL(a, ctx, ResourceService, AccessRead, "all services") // read
	}
	return nil
}

// ServiceWriteAllowed checks for permission to create or update a given
// service
func (a AllowAuthorizer) ServiceWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.ServiceWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceService, AccessWrite, name)
	}
	return nil
}

// ServiceWriteAnyAllowed checks for write permission on any service
func (a AllowAuthorizer) ServiceWriteAnyAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.ServiceWriteAny(ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceService, AccessWrite, "any service")
	}
	return nil
}

// SessionReadAllowed checks for permission to read sessions for a given node.
func (a AllowAuthorizer) SessionReadAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.SessionRead(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceSession, AccessRead, name)
	}
	return nil
}

// SessionWriteAllowed checks for permission to create sessions for a given
// node.
func (a AllowAuthorizer) SessionWriteAllowed(name string, ctx *AuthorizerContext) error {
	if a.Authorizer.SessionWrite(name, ctx) != Allow {
		return PermissionDeniedByACL(a, ctx, ResourceSession, AccessWrite, name)
	}
	return nil
}

// SnapshotAllowed checks for permission to take and restore snapshots.
func (a AllowAuthorizer) SnapshotAllowed(ctx *AuthorizerContext) error {
	if a.Authorizer.Snapshot(ctx) != Allow {
		// Implementation of this currently just checks acl write
		return PermissionDeniedByACLUnnamed(a, ctx, ResourceACL, AccessWrite)
	}
	return nil
}

func Enforce(authz Authorizer, rsc Resource, segment string, access string, ctx *AuthorizerContext) (EnforcementDecision, error) {
	lowerAccess := strings.ToLower(access)

	switch rsc {
	case ResourceACL:
		switch lowerAccess {
		case "read":
			return authz.ACLRead(ctx), nil
		case "write":
			return authz.ACLWrite(ctx), nil
		}
	case ResourceAgent:
		switch lowerAccess {
		case "read":
			return authz.AgentRead(segment, ctx), nil
		case "write":
			return authz.AgentWrite(segment, ctx), nil
		}
	case ResourceEvent:
		switch lowerAccess {
		case "read":
			return authz.EventRead(segment, ctx), nil
		case "write":
			return authz.EventWrite(segment, ctx), nil
		}
	case ResourceIntention:
		switch lowerAccess {
		case "read":
			return authz.IntentionRead(segment, ctx), nil
		case "write":
			return authz.IntentionWrite(segment, ctx), nil
		}
	case ResourceKey:
		switch lowerAccess {
		case "read":
			return authz.KeyRead(segment, ctx), nil
		case "list":
			return authz.KeyList(segment, ctx), nil
		case "write":
			return authz.KeyWrite(segment, ctx), nil
		case "write-prefix":
			return authz.KeyWritePrefix(segment, ctx), nil
		}
	case ResourceKeyring:
		switch lowerAccess {
		case "read":
			return authz.KeyringRead(ctx), nil
		case "write":
			return authz.KeyringWrite(ctx), nil
		}
	case ResourceMesh:
		switch lowerAccess {
		case "read":
			return authz.MeshRead(ctx), nil
		case "write":
			return authz.MeshWrite(ctx), nil
		}
	case ResourceNode:
		switch lowerAccess {
		case "read":
			return authz.NodeRead(segment, ctx), nil
		case "write":
			return authz.NodeWrite(segment, ctx), nil
		}
	case ResourceOperator:
		switch lowerAccess {
		case "read":
			return authz.OperatorRead(ctx), nil
		case "write":
			return authz.OperatorWrite(ctx), nil
		}
	case ResourceQuery:
		switch lowerAccess {
		case "read":
			return authz.PreparedQueryRead(segment, ctx), nil
		case "write":
			return authz.PreparedQueryWrite(segment, ctx), nil
		}
	case ResourceService:
		switch lowerAccess {
		case "read":
			return authz.ServiceRead(segment, ctx), nil
		case "write":
			return authz.ServiceWrite(segment, ctx), nil
		}
	case ResourceSession:
		switch lowerAccess {
		case "read":
			return authz.SessionRead(segment, ctx), nil
		case "write":
			return authz.SessionWrite(segment, ctx), nil
		}
	case ResourcePeering:
		switch lowerAccess {
		case "read":
			return authz.PeeringRead(ctx), nil
		case "write":
			return authz.PeeringWrite(ctx), nil
		}
	default:
		if processed, decision, err := enforceEnterprise(authz, rsc, segment, lowerAccess, ctx); processed {
			return decision, err
		}
		return Deny, fmt.Errorf("Invalid ACL resource requested: %q", rsc)
	}

	return Deny, fmt.Errorf("Invalid access level for %s resource: %s", rsc, access)
}

// NewAuthorizerFromRules is a convenience function to invoke NewPolicyFromSource followed by NewPolicyAuthorizer with
// the parse policy.
func NewAuthorizerFromRules(rules string, conf *Config, meta *EnterprisePolicyMeta) (Authorizer, error) {
	policy, err := NewPolicyFromSource(rules, conf, meta)
	if err != nil {
		return nil, err
	}

	return NewPolicyAuthorizer([]*Policy{policy}, conf)
}
