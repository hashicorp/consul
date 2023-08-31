// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"github.com/armon/go-radix"
)

type policyAuthorizer struct {
	// aclRule contains the acl management policy.
	aclRule *policyAuthorizerRule

	// agentRules contain the exact-match agent policies
	agentRules *radix.Tree

	// intentionRules contains the service intention exact-match policies
	intentionRules *radix.Tree

	// keyRules contains the key exact-match policies
	keyRules *radix.Tree

	// nodeRules contains the node exact-match policies
	nodeRules *radix.Tree

	// serviceRules contains the service exact-match policies
	serviceRules *radix.Tree

	// sessionRules contains the session exact-match policies
	sessionRules *radix.Tree

	// eventRules contains the user event exact-match policies
	eventRules *radix.Tree

	// preparedQueryRules contains the prepared query exact-match policies
	preparedQueryRules *radix.Tree

	// keyringRule contains the keyring policies. The keyring has
	// a very simple yes/no without prefix matching, so here we
	// don't need to use a radix tree.
	keyringRule *policyAuthorizerRule

	// operatorRule contains the operator policies.
	operatorRule *policyAuthorizerRule

	// meshRule contains the mesh policies.
	meshRule *policyAuthorizerRule

	// peeringRule contains the peering policies.
	peeringRule *policyAuthorizerRule

	// embedded enterprise policy authorizer
	enterprisePolicyAuthorizer
}

// policyAuthorizerRule is a struct to hold an ACL policy decision along
// with extra Consul Enterprise specific policy
type policyAuthorizerRule struct {
	// decision is the enforcement decision for this rule
	access AccessLevel

	// Embedded Consul Enterprise specific policy
	EnterpriseRule
}

// policyAuthorizerRadixLeaf is used as the main
// structure for storing in the radix.Tree's within the
// PolicyAuthorizer
type policyAuthorizerRadixLeaf struct {
	exact  *policyAuthorizerRule
	prefix *policyAuthorizerRule
}

// getPolicy first attempts to get an exact match for the segment from the "exact" tree and then falls
// back to getting the policy for the longest prefix from the "prefix" tree
func getPolicy(segment string, tree *radix.Tree) (policy *policyAuthorizerRule, found bool) {
	found = false

	tree.WalkPath(segment, func(path string, leaf interface{}) bool {
		policies := leaf.(*policyAuthorizerRadixLeaf)
		if policies.exact != nil && path == segment {
			found = true
			policy = policies.exact
			return true
		}

		if policies.prefix != nil {
			found = true
			policy = policies.prefix
		}
		return false
	})
	return
}

// insertPolicyIntoRadix will insert or update part of the leaf node within the radix tree corresponding to the
// given segment. To update only one of the exact match or prefix match policy, set the value you want to leave alone
// to nil when calling the function.
func insertPolicyIntoRadix(segment string, policy string, ent *EnterpriseRule, tree *radix.Tree, prefix bool) error {
	al, err := AccessLevelFromString(policy)
	if err != nil {
		return err
	}
	policyRule := policyAuthorizerRule{
		access: al,
	}

	if ent != nil {
		policyRule.EnterpriseRule = *ent
	}

	var policyLeaf *policyAuthorizerRadixLeaf
	leaf, found := tree.Get(segment)
	if found {
		policyLeaf = leaf.(*policyAuthorizerRadixLeaf)
	} else {
		policyLeaf = &policyAuthorizerRadixLeaf{}
		tree.Insert(segment, policyLeaf)
	}

	if prefix {
		policyLeaf.prefix = &policyRule
	} else {
		policyLeaf.exact = &policyRule
	}

	return nil
}

// enforce is a convenience function to
func enforce(rule AccessLevel, requiredPermission AccessLevel) EnforcementDecision {
	switch rule {
	case AccessWrite:
		// grants read, list and write permissions
		return Allow
	case AccessList:
		// grants read and list permissions
		if requiredPermission == AccessList || requiredPermission == AccessRead {
			return Allow
		} else {
			return Deny
		}
	case AccessRead:
		// grants just read permissions
		if requiredPermission == AccessRead {
			return Allow
		} else {
			return Deny
		}
	case AccessDeny:
		// explicit denial - do not recurse
		return Deny
	default:
		// need to recurse as there was no specific access level set
		return Default
	}
}

func defaultIsAllow(decision EnforcementDecision) EnforcementDecision {
	switch decision {
	case Allow, Default:
		return Allow
	default:
		return Deny
	}
}

func (p *policyAuthorizer) loadRules(policy *PolicyRules) error {
	// Load the agent policy (exact matches)
	for _, ap := range policy.Agents {
		if err := insertPolicyIntoRadix(ap.Node, ap.Policy, nil, p.agentRules, false); err != nil {
			return err
		}
	}

	// Load the agent policy (prefix matches)
	for _, ap := range policy.AgentPrefixes {
		if err := insertPolicyIntoRadix(ap.Node, ap.Policy, nil, p.agentRules, true); err != nil {
			return err
		}
	}

	// Load the key policy (exact matches)
	for _, kp := range policy.Keys {
		if err := insertPolicyIntoRadix(kp.Prefix, kp.Policy, &kp.EnterpriseRule, p.keyRules, false); err != nil {
			return err
		}
	}

	// Load the key policy (prefix matches)
	for _, kp := range policy.KeyPrefixes {
		if err := insertPolicyIntoRadix(kp.Prefix, kp.Policy, &kp.EnterpriseRule, p.keyRules, true); err != nil {
			return err
		}
	}

	// Load the node policy (exact matches)
	for _, np := range policy.Nodes {
		if err := insertPolicyIntoRadix(np.Name, np.Policy, &np.EnterpriseRule, p.nodeRules, false); err != nil {
			return err
		}
	}

	// Load the node policy (prefix matches)
	for _, np := range policy.NodePrefixes {
		if err := insertPolicyIntoRadix(np.Name, np.Policy, &np.EnterpriseRule, p.nodeRules, true); err != nil {
			return err
		}
	}

	// Load the service policy (exact matches)
	for _, sp := range policy.Services {
		if err := insertPolicyIntoRadix(sp.Name, sp.Policy, &sp.EnterpriseRule, p.serviceRules, false); err != nil {
			return err
		}

		intention := sp.Intentions
		if intention == "" {
			switch sp.Policy {
			case PolicyRead, PolicyWrite:
				intention = PolicyRead
			default:
				intention = PolicyDeny
			}
		}

		if err := insertPolicyIntoRadix(sp.Name, intention, &sp.EnterpriseRule, p.intentionRules, false); err != nil {
			return err
		}
	}

	// Load the service policy (prefix matches)
	for _, sp := range policy.ServicePrefixes {
		if err := insertPolicyIntoRadix(sp.Name, sp.Policy, &sp.EnterpriseRule, p.serviceRules, true); err != nil {
			return err
		}

		intention := sp.Intentions
		if intention == "" {
			switch sp.Policy {
			case PolicyRead, PolicyWrite:
				intention = PolicyRead
			default:
				intention = PolicyDeny
			}
		}

		if err := insertPolicyIntoRadix(sp.Name, intention, &sp.EnterpriseRule, p.intentionRules, true); err != nil {
			return err
		}
	}

	// Load the session policy (exact matches)
	for _, sp := range policy.Sessions {
		if err := insertPolicyIntoRadix(sp.Node, sp.Policy, nil, p.sessionRules, false); err != nil {
			return err
		}
	}

	// Load the session policy (prefix matches)
	for _, sp := range policy.SessionPrefixes {
		if err := insertPolicyIntoRadix(sp.Node, sp.Policy, nil, p.sessionRules, true); err != nil {
			return err
		}
	}

	// Load the event policy (exact matches)
	for _, ep := range policy.Events {
		if err := insertPolicyIntoRadix(ep.Event, ep.Policy, nil, p.eventRules, false); err != nil {
			return err
		}
	}

	// Load the event policy (prefix matches)
	for _, ep := range policy.EventPrefixes {
		if err := insertPolicyIntoRadix(ep.Event, ep.Policy, nil, p.eventRules, true); err != nil {
			return err
		}
	}

	// Load the prepared query policy (exact matches)
	for _, qp := range policy.PreparedQueries {
		if err := insertPolicyIntoRadix(qp.Prefix, qp.Policy, nil, p.preparedQueryRules, false); err != nil {
			return err
		}
	}

	// Load the prepared query policy (prefix matches)
	for _, qp := range policy.PreparedQueryPrefixes {
		if err := insertPolicyIntoRadix(qp.Prefix, qp.Policy, nil, p.preparedQueryRules, true); err != nil {
			return err
		}
	}

	// Load the acl policy
	if policy.ACL != "" {
		access, err := AccessLevelFromString(policy.ACL)
		if err != nil {
			return err
		}
		p.aclRule = &policyAuthorizerRule{access: access}
	}

	// Load the keyring policy
	if policy.Keyring != "" {
		access, err := AccessLevelFromString(policy.Keyring)
		if err != nil {
			return err
		}
		p.keyringRule = &policyAuthorizerRule{access: access}
	}

	// Load the operator policy
	if policy.Operator != "" {
		access, err := AccessLevelFromString(policy.Operator)
		if err != nil {
			return err
		}
		p.operatorRule = &policyAuthorizerRule{access: access}
	}

	// Load the mesh policy
	if policy.Mesh != "" {
		access, err := AccessLevelFromString(policy.Mesh)
		if err != nil {
			return err
		}
		p.meshRule = &policyAuthorizerRule{access: access}
	}

	// Load the peering policy
	if policy.Peering != "" {
		access, err := AccessLevelFromString(policy.Peering)
		if err != nil {
			return err
		}
		p.peeringRule = &policyAuthorizerRule{access: access}
	}

	return nil
}

func newPolicyAuthorizer(policies []*Policy, ent *Config) (*policyAuthorizer, error) {
	policy := MergePolicies(policies)

	return newPolicyAuthorizerFromRules(&policy.PolicyRules, ent)
}

func newPolicyAuthorizerFromRules(rules *PolicyRules, ent *Config) (*policyAuthorizer, error) {
	p := &policyAuthorizer{
		agentRules:         radix.New(),
		intentionRules:     radix.New(),
		keyRules:           radix.New(),
		nodeRules:          radix.New(),
		serviceRules:       radix.New(),
		sessionRules:       radix.New(),
		eventRules:         radix.New(),
		preparedQueryRules: radix.New(),
	}

	p.enterprisePolicyAuthorizer.init(ent)

	if err := p.loadRules(rules); err != nil {
		return nil, err
	}

	return p, nil
}

// enforceCallbacks are to be passed to anyAllowed or allAllowed. The interface{}
// parameter will be a value stored in the radix.Tree passed to those functions.
// prefixOnly indicates that only we only want to consider the prefix matching rule
// if any. The return value indicates whether this one leaf node in the tree would
// allow, deny or make no decision regarding some authorization.
type enforceCallback func(raw interface{}, prefixOnly bool) EnforcementDecision

func anyAllowed(tree *radix.Tree, enforceFn enforceCallback) EnforcementDecision {
	decision := Default

	// special case for handling a catch-all prefix rule. If the rule would Deny access then our default decision
	// should be to Deny, but this decision should still be overridable with other more specific rules.
	if raw, found := tree.Get(""); found {
		decision = enforceFn(raw, true)
		if decision == Allow {
			return Allow
		}
	}

	tree.Walk(func(path string, raw interface{}) bool {
		if enforceFn(raw, false) == Allow {
			decision = Allow
			return true
		}

		return false
	})

	return decision
}

func allAllowed(tree *radix.Tree, enforceFn enforceCallback) EnforcementDecision {
	decision := Default

	// look for a "" prefix rule
	if raw, found := tree.Get(""); found {
		// ensure that the empty prefix rule would allow the access
		// if it does allow it we still must check all the other rules to ensure
		// nothing overrides the top level grant with a different access level
		// if not we can return early
		decision = enforceFn(raw, true)

		// the top level prefix rule denied access so we can return early.
		if decision == Deny {
			return Deny
		}
	}

	tree.Walk(func(path string, raw interface{}) bool {
		if enforceFn(raw, false) == Deny {
			decision = Deny
			return true
		}
		return false
	})

	return decision
}

func (authz *policyAuthorizer) anyAllowed(tree *radix.Tree, requiredPermission AccessLevel) EnforcementDecision {
	return anyAllowed(tree, func(raw interface{}, prefixOnly bool) EnforcementDecision {
		leaf := raw.(*policyAuthorizerRadixLeaf)
		decision := Default

		if leaf.prefix != nil {
			decision = enforce(leaf.prefix.access, requiredPermission)
		}

		if prefixOnly || decision == Allow || leaf.exact == nil {
			return decision
		}

		return enforce(leaf.exact.access, requiredPermission)
	})
}

func (authz *policyAuthorizer) allAllowed(tree *radix.Tree, requiredPermission AccessLevel) EnforcementDecision {
	return allAllowed(tree, func(raw interface{}, prefixOnly bool) EnforcementDecision {
		leaf := raw.(*policyAuthorizerRadixLeaf)
		prefixDecision := Default

		if leaf.prefix != nil {
			prefixDecision = enforce(leaf.prefix.access, requiredPermission)
		}

		if prefixOnly || prefixDecision == Deny || leaf.exact == nil {
			return prefixDecision
		}

		decision := enforce(leaf.exact.access, requiredPermission)

		if decision == Default {
			// basically this means defer to the prefix decision as the
			// authorizer rule made no decision with an exact match rule
			return prefixDecision
		}

		return decision
	})
}

// ACLRead checks if listing of ACLs is allowed
func (p *policyAuthorizer) ACLRead(*AuthorizerContext) EnforcementDecision {
	if p.aclRule != nil {
		return enforce(p.aclRule.access, AccessRead)
	}
	return Default
}

// ACLWrite checks if modification of ACLs is allowed
func (p *policyAuthorizer) ACLWrite(*AuthorizerContext) EnforcementDecision {
	if p.aclRule != nil {
		return enforce(p.aclRule.access, AccessWrite)
	}
	return Default
}

// AgentRead checks for permission to read from agent endpoints for a given
// node.
func (p *policyAuthorizer) AgentRead(node string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(node, p.agentRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// AgentWrite checks for permission to make changes via agent endpoints for a
// given node.
func (p *policyAuthorizer) AgentWrite(node string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(node, p.agentRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

// Snapshot checks if taking and restoring snapshots is allowed.
func (p *policyAuthorizer) Snapshot(_ *AuthorizerContext) EnforcementDecision {
	if p.aclRule != nil {
		return enforce(p.aclRule.access, AccessWrite)
	}
	return Default
}

// EventRead is used to determine if the policy allows for a
// specific user event to be read.
func (p *policyAuthorizer) EventRead(name string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(name, p.eventRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// EventWrite is used to determine if new events can be created
// (fired) by the policy.
func (p *policyAuthorizer) EventWrite(name string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(name, p.eventRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

// IntentionDefaultAllow returns whether the default behavior when there are
// no matching intentions is to allow or deny.
func (p *policyAuthorizer) IntentionDefaultAllow(_ *AuthorizerContext) EnforcementDecision {
	// We always go up, this can't be determined by a policy.
	return Default
}

// IntentionRead checks if writing (creating, updating, or deleting) of an
// intention is allowed.
func (p *policyAuthorizer) IntentionRead(prefix string, _ *AuthorizerContext) EnforcementDecision {
	if prefix == "*" {
		return p.anyAllowed(p.intentionRules, AccessRead)
	}

	if rule, ok := getPolicy(prefix, p.intentionRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// IntentionWrite checks if writing (creating, updating, or deleting) of an
// intention is allowed.
func (p *policyAuthorizer) IntentionWrite(prefix string, _ *AuthorizerContext) EnforcementDecision {
	if prefix == "*" {
		return p.allAllowed(p.intentionRules, AccessWrite)
	}

	if rule, ok := getPolicy(prefix, p.intentionRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

// KeyRead returns if a key is allowed to be read
func (p *policyAuthorizer) KeyRead(key string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(key, p.keyRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// KeyList returns if a key is allowed to be listed
func (p *policyAuthorizer) KeyList(key string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(key, p.keyRules); ok {
		return enforce(rule.access, AccessList)
	}
	return Default
}

// KeyWrite returns if a key is allowed to be written
func (p *policyAuthorizer) KeyWrite(key string, entCtx *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(key, p.keyRules); ok {
		decision := enforce(rule.access, AccessWrite)
		if decision == Allow {
			return defaultIsAllow(p.enterprisePolicyAuthorizer.enforce(&rule.EnterpriseRule, entCtx))
		}
		return decision
	}
	return Default
}

// KeyWritePrefix returns if a prefix is allowed to be written
//
// This is mainly used to detect whether a whole tree within
// the KV can be removed. For that reason we must be able to
// delete everything under the prefix. First we must have "write"
// on the prefix itself
func (p *policyAuthorizer) KeyWritePrefix(prefix string, _ *AuthorizerContext) EnforcementDecision {
	// Conditions for Allow:
	//   * The longest prefix match rule that would apply to the given prefix
	//     grants AccessWrite
	//   AND
	//   * There are no rules (exact or prefix match) within/under the given prefix
	//     that would NOT grant AccessWrite.
	//
	// Conditions for Deny:
	//   * The longest prefix match rule that would apply to the given prefix
	//     does not grant AccessWrite.
	//   OR
	//   * There is 1+ rules (exact or prefix match) within/under the given prefix
	//     that do NOT grant AccessWrite.
	//
	// Conditions for Default:
	//   * There is no prefix match rule that would appy to the given prefix.
	//   AND
	//   * There are no rules (exact or prefix match) within/under the given prefix
	//     that would NOT grant AccessWrite.

	baseAccess := Default

	// Look for a prefix rule that would apply to the prefix we are checking
	// WalkPath starts at the root and walks down to the given prefix.
	// Therefore the last prefix rule we see is the one that matters
	p.keyRules.WalkPath(prefix, func(path string, leaf interface{}) bool {
		rule := leaf.(*policyAuthorizerRadixLeaf)

		if rule.prefix != nil {
			if rule.prefix.access != AccessWrite {
				baseAccess = Deny
			} else {
				baseAccess = Allow
			}
		}
		return false
	})

	// baseAccess will be Deny only when a prefix rule was found and it didn't
	// grant AccessWrite. Otherwise the access level will be Default or Allow
	// neither of which should be returned right now.
	if baseAccess == Deny {
		return baseAccess
	}

	// Look if any of our children do not allow write access. This loop takes
	// into account both prefix and exact match rules.
	withinPrefixAccess := Default
	p.keyRules.WalkPrefix(prefix, func(path string, leaf interface{}) bool {
		rule := leaf.(*policyAuthorizerRadixLeaf)

		if rule.prefix != nil && rule.prefix.access != AccessWrite {
			withinPrefixAccess = Deny
			return true
		}
		if rule.exact != nil && rule.exact.access != AccessWrite {
			withinPrefixAccess = Deny
			return true
		}

		return false
	})

	// Deny the write if any sub-rules may be violated. If none are violated then
	// we can defer to the baseAccess.
	if withinPrefixAccess == Deny {
		return Deny
	}

	// either Default or Allow at this point. Allow if there was a prefix rule
	// that was applicable and it granted write access. Default if there was
	// no applicable rule.
	return baseAccess
}

// KeyringRead is used to determine if the keyring can be
// read by the current ACL token.
func (p *policyAuthorizer) KeyringRead(*AuthorizerContext) EnforcementDecision {
	if p.keyringRule != nil {
		return enforce(p.keyringRule.access, AccessRead)
	}
	return Default
}

// KeyringWrite determines if the keyring can be manipulated.
func (p *policyAuthorizer) KeyringWrite(*AuthorizerContext) EnforcementDecision {
	if p.keyringRule != nil {
		return enforce(p.keyringRule.access, AccessWrite)
	}
	return Default
}

// MeshRead determines if the read-only mesh functions are allowed.
func (p *policyAuthorizer) MeshRead(ctx *AuthorizerContext) EnforcementDecision {
	if p.meshRule != nil {
		return enforce(p.meshRule.access, AccessRead)
	}
	// default to OperatorRead access
	return p.OperatorRead(ctx)
}

// MeshWrite determines if the state-changing mesh functions are
// allowed.
func (p *policyAuthorizer) MeshWrite(ctx *AuthorizerContext) EnforcementDecision {
	if p.meshRule != nil {
		return enforce(p.meshRule.access, AccessWrite)
	}
	// default to OperatorWrite access
	return p.OperatorWrite(ctx)
}

// PeeringRead determines if the read-only peering functions are allowed.
func (p *policyAuthorizer) PeeringRead(ctx *AuthorizerContext) EnforcementDecision {
	if p.peeringRule != nil {
		return enforce(p.peeringRule.access, AccessRead)
	}
	// default to OperatorRead access
	return p.OperatorRead(ctx)
}

// PeeringWrite determines if the state-changing peering functions are
// allowed.
func (p *policyAuthorizer) PeeringWrite(ctx *AuthorizerContext) EnforcementDecision {
	if p.peeringRule != nil {
		return enforce(p.peeringRule.access, AccessWrite)
	}
	// default to OperatorWrite access
	return p.OperatorWrite(ctx)
}

// OperatorRead determines if the read-only operator functions are allowed.
func (p *policyAuthorizer) OperatorRead(*AuthorizerContext) EnforcementDecision {
	if p.operatorRule != nil {
		return enforce(p.operatorRule.access, AccessRead)
	}
	return Default
}

// OperatorWrite determines if the state-changing operator functions are
// allowed.
func (p *policyAuthorizer) OperatorWrite(*AuthorizerContext) EnforcementDecision {
	if p.operatorRule != nil {
		return enforce(p.operatorRule.access, AccessWrite)
	}
	return Default
}

// NodeRead checks if reading (discovery) of a node is allowed
func (p *policyAuthorizer) NodeRead(name string, ctx *AuthorizerContext) EnforcementDecision {
	// When reading a node imported from a peer we consider it to be allowed when:
	//  - The request comes from a locally authenticated service, meaning that it
	//    has service:write permissions on some name.
	//  - The requester has permissions to read all nodes in its local cluster,
	//    therefore it can also read imported nodes.
	if ctx.PeerOrEmpty() != "" {
		if p.ServiceWriteAny(nil) == Allow {
			return Allow
		}
		return p.NodeReadAll(nil)
	}
	if rule, ok := getPolicy(name, p.nodeRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

func (p *policyAuthorizer) NodeReadAll(_ *AuthorizerContext) EnforcementDecision {
	return p.allAllowed(p.nodeRules, AccessRead)
}

// NodeWrite checks if writing (registering) a node is allowed
func (p *policyAuthorizer) NodeWrite(name string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(name, p.nodeRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

// PreparedQueryRead checks if reading (listing) of a prepared query is
// allowed - this isn't execution, just listing its contents.
func (p *policyAuthorizer) PreparedQueryRead(prefix string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(prefix, p.preparedQueryRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// PreparedQueryWrite checks if writing (creating, updating, or deleting) of a
// prepared query is allowed.
func (p *policyAuthorizer) PreparedQueryWrite(prefix string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(prefix, p.preparedQueryRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

// ServiceRead checks if reading (discovery) of a service is allowed
func (p *policyAuthorizer) ServiceRead(name string, ctx *AuthorizerContext) EnforcementDecision {
	// When reading a service imported from a peer we consider it to be allowed when:
	//  - The request comes from a locally authenticated service, meaning that it
	//    has service:write permissions on some name.
	//  - The requester has permissions to read all services in its local cluster,
	//    therefore it can also read imported services.
	if ctx.PeerOrEmpty() != "" {
		if p.ServiceWriteAny(nil) == Allow {
			return Allow
		}
		return p.ServiceReadAll(nil)
	}
	if rule, ok := getPolicy(name, p.serviceRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

func (p *policyAuthorizer) ServiceReadAll(_ *AuthorizerContext) EnforcementDecision {
	return p.allAllowed(p.serviceRules, AccessRead)
}

// ServiceWrite checks if writing (registering) a service is allowed
func (p *policyAuthorizer) ServiceWrite(name string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(name, p.serviceRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

func (p *policyAuthorizer) ServiceWriteAny(_ *AuthorizerContext) EnforcementDecision {
	return p.anyAllowed(p.serviceRules, AccessWrite)
}

// SessionRead checks for permission to read sessions for a given node.
func (p *policyAuthorizer) SessionRead(node string, _ *AuthorizerContext) EnforcementDecision {
	if rule, ok := getPolicy(node, p.sessionRules); ok {
		return enforce(rule.access, AccessRead)
	}
	return Default
}

// SessionWrite checks for permission to create sessions for a given node.
func (p *policyAuthorizer) SessionWrite(node string, _ *AuthorizerContext) EnforcementDecision {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(node, p.sessionRules); ok {
		return enforce(rule.access, AccessWrite)
	}
	return Default
}

func (p *policyAuthorizer) ToAllowAuthorizer() AllowAuthorizer {
	return AllowAuthorizer{Authorizer: p}
}
