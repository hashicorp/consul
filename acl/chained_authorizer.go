// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

// ChainedAuthorizer can combine multiple Authorizers into one.
// Each Authorizer in the chain is asked (in order) for an
// enforcement decision. The first non-Default decision that
// is rendered by an Authorizer in the chain will be used
// as the overall decision of the ChainedAuthorizer
type ChainedAuthorizer struct {
	chain []Authorizer
}

// NewChainedAuthorizer creates a ChainedAuthorizer with the provided
// chain of Authorizers. The slice provided should be in the order of
// most precedent Authorizer at the beginning and least precedent
// Authorizer at the end.
func NewChainedAuthorizer(chain []Authorizer) *ChainedAuthorizer {
	return &ChainedAuthorizer{
		chain: chain,
	}
}

func (c *ChainedAuthorizer) AuthorizerChain() []Authorizer {
	return c.chain
}

func (c *ChainedAuthorizer) executeChain(enforce func(authz Authorizer) EnforcementDecision) EnforcementDecision {
	for _, authz := range c.chain {
		decision := enforce(authz)
		if decision != Default {
			return decision
		}
	}
	return Deny
}

// ACLRead checks for permission to list all the ACLs
func (c *ChainedAuthorizer) ACLRead(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ACLRead(entCtx)
	})
}

// ACLWrite checks for permission to manipulate ACLs
func (c *ChainedAuthorizer) ACLWrite(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ACLWrite(entCtx)
	})
}

// AgentRead checks for permission to read from agent endpoints for a
// given node.
func (c *ChainedAuthorizer) AgentRead(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.AgentRead(node, entCtx)
	})
}

// AgentWrite checks for permission to make changes via agent endpoints
// for a given node.
func (c *ChainedAuthorizer) AgentWrite(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.AgentWrite(node, entCtx)
	})
}

// EventRead determines if a specific event can be queried.
func (c *ChainedAuthorizer) EventRead(name string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.EventRead(name, entCtx)
	})
}

// EventWrite determines if a specific event may be fired.
func (c *ChainedAuthorizer) EventWrite(name string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.EventWrite(name, entCtx)
	})
}

// IntentionDefaultAllow determines the default authorized behavior
// when no intentions match a Connect request.
func (c *ChainedAuthorizer) IntentionDefaultAllow(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.IntentionDefaultAllow(entCtx)
	})
}

// IntentionRead determines if a specific intention can be read.
func (c *ChainedAuthorizer) IntentionRead(prefix string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.IntentionRead(prefix, entCtx)
	})
}

// IntentionWrite determines if a specific intention can be
// created, modified, or deleted.
func (c *ChainedAuthorizer) IntentionWrite(prefix string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.IntentionWrite(prefix, entCtx)
	})
}

// KeyList checks for permission to list keys under a prefix
func (c *ChainedAuthorizer) KeyList(keyPrefix string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyList(keyPrefix, entCtx)
	})
}

// KeyRead checks for permission to read a given key
func (c *ChainedAuthorizer) KeyRead(key string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyRead(key, entCtx)
	})
}

// KeyWrite checks for permission to write a given key
func (c *ChainedAuthorizer) KeyWrite(key string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyWrite(key, entCtx)
	})
}

// KeyWritePrefix checks for permission to write to an
// entire key prefix. This means there must be no sub-policies
// that deny a write.
func (c *ChainedAuthorizer) KeyWritePrefix(keyPrefix string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyWritePrefix(keyPrefix, entCtx)
	})
}

// KeyringRead determines if the encryption keyring used in
// the gossip layer can be read.
func (c *ChainedAuthorizer) KeyringRead(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyringRead(entCtx)
	})
}

// KeyringWrite determines if the keyring can be manipulated
func (c *ChainedAuthorizer) KeyringWrite(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.KeyringWrite(entCtx)
	})
}

// MeshRead determines if the read-only Consul mesh functions
// can be used.
func (c *ChainedAuthorizer) MeshRead(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.MeshRead(entCtx)
	})
}

// MeshWrite determines if the state-changing Consul mesh
// functions can be used.
func (c *ChainedAuthorizer) MeshWrite(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.MeshWrite(entCtx)
	})
}

// PeeringRead determines if the read-only Consul peering functions
// can be used.
func (c *ChainedAuthorizer) PeeringRead(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.PeeringRead(entCtx)
	})
}

// PeeringWrite determines if the state-changing Consul peering
// functions can be used.
func (c *ChainedAuthorizer) PeeringWrite(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.PeeringWrite(entCtx)
	})
}

// NodeRead checks for permission to read (discover) a given node.
func (c *ChainedAuthorizer) NodeRead(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.NodeRead(node, entCtx)
	})
}

func (c *ChainedAuthorizer) NodeReadAll(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.NodeReadAll(entCtx)
	})
}

// NodeWrite checks for permission to create or update (register) a
// given node.
func (c *ChainedAuthorizer) NodeWrite(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.NodeWrite(node, entCtx)
	})
}

// OperatorRead determines if the read-only Consul operator functions
// can be used.
func (c *ChainedAuthorizer) OperatorRead(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.OperatorRead(entCtx)
	})
}

// OperatorWrite determines if the state-changing Consul operator
// functions can be used.
func (c *ChainedAuthorizer) OperatorWrite(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.OperatorWrite(entCtx)
	})
}

// PreparedQueryRead determines if a specific prepared query can be read
// to show its contents (this is not used for execution).
func (c *ChainedAuthorizer) PreparedQueryRead(query string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.PreparedQueryRead(query, entCtx)
	})
}

// PreparedQueryWrite determines if a specific prepared query can be
// created, modified, or deleted.
func (c *ChainedAuthorizer) PreparedQueryWrite(query string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.PreparedQueryWrite(query, entCtx)
	})
}

// ServiceRead checks for permission to read a given service
func (c *ChainedAuthorizer) ServiceRead(name string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ServiceRead(name, entCtx)
	})
}

func (c *ChainedAuthorizer) ServiceReadAll(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ServiceReadAll(entCtx)
	})
}

// ServiceWrite checks for permission to create or update a given
// service
func (c *ChainedAuthorizer) ServiceWrite(name string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ServiceWrite(name, entCtx)
	})
}

// ServiceWriteAny checks for write permission on any service
func (c *ChainedAuthorizer) ServiceWriteAny(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.ServiceWriteAny(entCtx)
	})
}

// SessionRead checks for permission to read sessions for a given node.
func (c *ChainedAuthorizer) SessionRead(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.SessionRead(node, entCtx)
	})
}

// SessionWrite checks for permission to create sessions for a given
// node.
func (c *ChainedAuthorizer) SessionWrite(node string, entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.SessionWrite(node, entCtx)
	})
}

// Snapshot checks for permission to take and restore snapshots.
func (c *ChainedAuthorizer) Snapshot(entCtx *AuthorizerContext) EnforcementDecision {
	return c.executeChain(func(authz Authorizer) EnforcementDecision {
		return authz.Snapshot(entCtx)
	})
}

func (c *ChainedAuthorizer) ToAllowAuthorizer() AllowAuthorizer {
	return AllowAuthorizer{Authorizer: c}
}
