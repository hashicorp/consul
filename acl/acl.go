package acl

import (
	"github.com/armon/go-radix"
	"github.com/hashicorp/consul/sentinel"
)

var (
	// allowAll is a singleton policy which allows all
	// non-management actions
	allowAll Authorizer

	// denyAll is a singleton policy which denies all actions
	denyAll Authorizer

	// manageAll is a singleton policy which allows all
	// actions, including management
	manageAll Authorizer
)

// DefaultPolicyEnforcementLevel will be used if the user leaves the level
// blank when configuring an ACL.
const DefaultPolicyEnforcementLevel = "hard-mandatory"

func init() {
	// Setup the singletons
	allowAll = &StaticAuthorizer{
		allowManage:  false,
		defaultAllow: true,
	}
	denyAll = &StaticAuthorizer{
		allowManage:  false,
		defaultAllow: false,
	}
	manageAll = &StaticAuthorizer{
		allowManage:  true,
		defaultAllow: true,
	}
}

// Authorizer is the interface for policy enforcement.
type Authorizer interface {
	// ACLRead checks for permission to list all the ACLs
	ACLRead() bool

	// ACLWrite checks for permission to manipulate ACLs
	ACLWrite() bool

	// AgentRead checks for permission to read from agent endpoints for a
	// given node.
	AgentRead(string) bool

	// AgentWrite checks for permission to make changes via agent endpoints
	// for a given node.
	AgentWrite(string) bool

	// EventRead determines if a specific event can be queried.
	EventRead(string) bool

	// EventWrite determines if a specific event may be fired.
	EventWrite(string) bool

	// IntentionDefaultAllow determines the default authorized behavior
	// when no intentions match a Connect request.
	IntentionDefaultAllow() bool

	// IntentionRead determines if a specific intention can be read.
	IntentionRead(string) bool

	// IntentionWrite determines if a specific intention can be
	// created, modified, or deleted.
	IntentionWrite(string) bool

	// KeyList checks for permission to list keys under a prefix
	KeyList(string) bool

	// KeyRead checks for permission to read a given key
	KeyRead(string) bool

	// KeyWrite checks for permission to write a given key
	KeyWrite(string, sentinel.ScopeFn) bool

	// KeyWritePrefix checks for permission to write to an
	// entire key prefix. This means there must be no sub-policies
	// that deny a write.
	KeyWritePrefix(string) bool

	// KeyringRead determines if the encryption keyring used in
	// the gossip layer can be read.
	KeyringRead() bool

	// KeyringWrite determines if the keyring can be manipulated
	KeyringWrite() bool

	// NodeRead checks for permission to read (discover) a given node.
	NodeRead(string) bool

	// NodeWrite checks for permission to create or update (register) a
	// given node.
	NodeWrite(string, sentinel.ScopeFn) bool

	// OperatorRead determines if the read-only Consul operator functions
	// can be used.
	OperatorRead() bool

	// OperatorWrite determines if the state-changing Consul operator
	// functions can be used.
	OperatorWrite() bool

	// PreparedQueryRead determines if a specific prepared query can be read
	// to show its contents (this is not used for execution).
	PreparedQueryRead(string) bool

	// PreparedQueryWrite determines if a specific prepared query can be
	// created, modified, or deleted.
	PreparedQueryWrite(string) bool

	// ServiceRead checks for permission to read a given service
	ServiceRead(string) bool

	// ServiceWrite checks for permission to create or update a given
	// service
	ServiceWrite(string, sentinel.ScopeFn) bool

	// SessionRead checks for permission to read sessions for a given node.
	SessionRead(string) bool

	// SessionWrite checks for permission to create sessions for a given
	// node.
	SessionWrite(string) bool

	// Snapshot checks for permission to take and restore snapshots.
	Snapshot() bool
}

// StaticAuthorizer is used to implement a base ACL policy. It either
// allows or denies all requests. This can be used as a parent
// ACL to act in a blacklist or whitelist mode.
type StaticAuthorizer struct {
	allowManage  bool
	defaultAllow bool
}

func (s *StaticAuthorizer) ACLRead() bool {
	return s.allowManage
}

func (s *StaticAuthorizer) ACLWrite() bool {
	return s.allowManage
}

func (s *StaticAuthorizer) AgentRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) AgentWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) EventRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) EventWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) IntentionDefaultAllow() bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) IntentionRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) IntentionWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyList(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyWrite(string, sentinel.ScopeFn) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyWritePrefix(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyringRead() bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) KeyringWrite() bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) NodeRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) NodeWrite(string, sentinel.ScopeFn) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) OperatorRead() bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) OperatorWrite() bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) PreparedQueryRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) PreparedQueryWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) ServiceRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) ServiceWrite(string, sentinel.ScopeFn) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) SessionRead(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) SessionWrite(string) bool {
	return s.defaultAllow
}

func (s *StaticAuthorizer) Snapshot() bool {
	return s.allowManage
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

// RulePolicy binds a regular ACL policy along with an optional piece of
// code to execute.
type RulePolicy struct {
	// aclPolicy is used for simple acl rules(allow/deny/manage)
	aclPolicy string

	// sentinelPolicy has the code part of a policy
	sentinelPolicy Sentinel
}

// PolicyAuthorizer is used to wrap a set of ACL policies to provide
// the Authorizer interface.
//
type PolicyAuthorizer struct {
	// parent is used to resolve policy if we have
	// no matching rule.
	parent Authorizer

	// sentinel is an interface for validating and executing sentinel code
	// policies.
	sentinel sentinel.Evaluator

	// aclRule contains the acl management policy.
	aclRule string

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
	keyringRule string

	// operatorRule contains the operator policies.
	operatorRule string
}

// policyAuthorizerRadixLeaf is used as the main
// structure for storing in the radix.Tree's within the
// PolicyAuthorizer
type policyAuthorizerRadixLeaf struct {
	exact  interface{}
	prefix interface{}
}

// getPolicy first attempts to get an exact match for the segment from the "exact" tree and then falls
// back to getting the policy for the longest prefix from the "prefix" tree
func getPolicy(segment string, tree *radix.Tree) (policy interface{}, found bool) {
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

func insertPolicyIntoRadix(segment string, tree *radix.Tree, exactPolicy interface{}, prefixPolicy interface{}) {
	leaf, found := tree.Get(segment)
	if found {
		policy := leaf.(*policyAuthorizerRadixLeaf)
		if exactPolicy != nil {
			policy.exact = exactPolicy
		}
		if prefixPolicy != nil {
			policy.prefix = prefixPolicy
		}
	} else {
		policy := &policyAuthorizerRadixLeaf{exact: exactPolicy, prefix: prefixPolicy}
		tree.Insert(segment, policy)
	}
}

func enforce(rule string, requiredPermission string) (allow, recurse bool) {
	switch rule {
	case PolicyWrite:
		// grants read, list and write permissions
		return true, false
	case PolicyList:
		// grants read and list permissions
		if requiredPermission == PolicyList || requiredPermission == PolicyRead {
			return true, false
		} else {
			return false, false
		}
	case PolicyRead:
		// grants just read permissions
		if requiredPermission == PolicyRead {
			return true, false
		} else {
			return false, false
		}
	case PolicyDeny:
		// explicit denial - do not recurse
		return false, false
	default:
		// need to recurse as there was no specific policy set
		return false, true
	}
}

// NewPolicyAuthorizer is used to construct a policy based ACL from a set of policies
// and a parent policy to resolve missing cases.
func NewPolicyAuthorizer(parent Authorizer, policies []*Policy, sentinel sentinel.Evaluator) (*PolicyAuthorizer, error) {
	p := &PolicyAuthorizer{
		parent:             parent,
		agentRules:         radix.New(),
		intentionRules:     radix.New(),
		keyRules:           radix.New(),
		nodeRules:          radix.New(),
		serviceRules:       radix.New(),
		sessionRules:       radix.New(),
		eventRules:         radix.New(),
		preparedQueryRules: radix.New(),
		sentinel:           sentinel,
	}

	policy := MergePolicies(policies)

	// Load the agent policy (exact matches)
	for _, ap := range policy.Agents {
		insertPolicyIntoRadix(ap.Node, p.agentRules, ap.Policy, nil)
	}

	// Load the agent policy (prefix matches)
	for _, ap := range policy.AgentPrefixes {
		insertPolicyIntoRadix(ap.Node, p.agentRules, nil, ap.Policy)
	}

	// Load the key policy (exact matches)
	for _, kp := range policy.Keys {
		policyRule := RulePolicy{
			aclPolicy:      kp.Policy,
			sentinelPolicy: kp.Sentinel,
		}
		insertPolicyIntoRadix(kp.Prefix, p.keyRules, policyRule, nil)
	}

	// Load the key policy (prefix matches)
	for _, kp := range policy.KeyPrefixes {
		policyRule := RulePolicy{
			aclPolicy:      kp.Policy,
			sentinelPolicy: kp.Sentinel,
		}
		insertPolicyIntoRadix(kp.Prefix, p.keyRules, nil, policyRule)
	}

	// Load the node policy (exact matches)
	for _, np := range policy.Nodes {
		policyRule := RulePolicy{
			aclPolicy:      np.Policy,
			sentinelPolicy: np.Sentinel,
		}
		insertPolicyIntoRadix(np.Name, p.nodeRules, policyRule, nil)
	}

	// Load the node policy (prefix matches)
	for _, np := range policy.NodePrefixes {
		policyRule := RulePolicy{
			aclPolicy:      np.Policy,
			sentinelPolicy: np.Sentinel,
		}
		insertPolicyIntoRadix(np.Name, p.nodeRules, nil, policyRule)
	}

	// Load the service policy (exact matches)
	for _, sp := range policy.Services {
		policyRule := RulePolicy{
			aclPolicy:      sp.Policy,
			sentinelPolicy: sp.Sentinel,
		}
		insertPolicyIntoRadix(sp.Name, p.serviceRules, policyRule, nil)

		intention := sp.Intentions
		if intention == "" {
			switch sp.Policy {
			case PolicyRead, PolicyWrite:
				intention = PolicyRead
			default:
				intention = PolicyDeny
			}
		}

		policyRule = RulePolicy{
			aclPolicy:      intention,
			sentinelPolicy: sp.Sentinel,
		}
		insertPolicyIntoRadix(sp.Name, p.intentionRules, policyRule, nil)
	}

	// Load the service policy (prefix matches)
	for _, sp := range policy.ServicePrefixes {
		policyRule := RulePolicy{
			aclPolicy:      sp.Policy,
			sentinelPolicy: sp.Sentinel,
		}
		insertPolicyIntoRadix(sp.Name, p.serviceRules, nil, policyRule)

		intention := sp.Intentions
		if intention == "" {
			switch sp.Policy {
			case PolicyRead, PolicyWrite:
				intention = PolicyRead
			default:
				intention = PolicyDeny
			}
		}

		policyRule = RulePolicy{
			aclPolicy:      intention,
			sentinelPolicy: sp.Sentinel,
		}
		insertPolicyIntoRadix(sp.Name, p.intentionRules, nil, policyRule)
	}

	// Load the session policy (exact matches)
	for _, sp := range policy.Sessions {
		insertPolicyIntoRadix(sp.Node, p.sessionRules, sp.Policy, nil)
	}

	// Load the session policy (prefix matches)
	for _, sp := range policy.SessionPrefixes {
		insertPolicyIntoRadix(sp.Node, p.sessionRules, nil, sp.Policy)
	}

	// Load the event policy (exact matches)
	for _, ep := range policy.Events {
		insertPolicyIntoRadix(ep.Event, p.eventRules, ep.Policy, nil)
	}

	// Load the event policy (prefix matches)
	for _, ep := range policy.EventPrefixes {
		insertPolicyIntoRadix(ep.Event, p.eventRules, nil, ep.Policy)
	}

	// Load the prepared query policy (exact matches)
	for _, qp := range policy.PreparedQueries {
		insertPolicyIntoRadix(qp.Prefix, p.preparedQueryRules, qp.Policy, nil)
	}

	// Load the prepared query policy (prefix matches)
	for _, qp := range policy.PreparedQueryPrefixes {
		insertPolicyIntoRadix(qp.Prefix, p.preparedQueryRules, nil, qp.Policy)
	}

	// Load the acl policy
	p.aclRule = policy.ACL

	// Load the keyring policy
	p.keyringRule = policy.Keyring

	// Load the operator policy
	p.operatorRule = policy.Operator

	return p, nil
}

// ACLRead checks if listing of ACLs is allowed
func (p *PolicyAuthorizer) ACLRead() bool {
	if allow, recurse := enforce(p.aclRule, PolicyRead); !recurse {
		return allow
	}

	return p.parent.ACLRead()
}

// ACLWrite checks if modification of ACLs is allowed
func (p *PolicyAuthorizer) ACLWrite() bool {
	if allow, recurse := enforce(p.aclRule, PolicyWrite); !recurse {
		return allow
	}

	return p.parent.ACLWrite()
}

// AgentRead checks for permission to read from agent endpoints for a given
// node.
func (p *PolicyAuthorizer) AgentRead(node string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(node, p.agentRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.AgentRead(node)
}

// AgentWrite checks for permission to make changes via agent endpoints for a
// given node.
func (p *PolicyAuthorizer) AgentWrite(node string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(node, p.agentRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyWrite); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.AgentWrite(node)
}

// Snapshot checks if taking and restoring snapshots is allowed.
func (p *PolicyAuthorizer) Snapshot() bool {
	if allow, recurse := enforce(p.aclRule, PolicyWrite); !recurse {
		return allow
	}
	return p.parent.Snapshot()
}

// EventRead is used to determine if the policy allows for a
// specific user event to be read.
func (p *PolicyAuthorizer) EventRead(name string) bool {
	// Longest-prefix match on event names
	if rule, ok := getPolicy(name, p.eventRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.EventRead(name)
}

// EventWrite is used to determine if new events can be created
// (fired) by the policy.
func (p *PolicyAuthorizer) EventWrite(name string) bool {
	// Longest-prefix match event names
	if rule, ok := getPolicy(name, p.eventRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyWrite); !recurse {
			return allow
		}
	}

	// No match, use parent
	return p.parent.EventWrite(name)
}

// IntentionDefaultAllow returns whether the default behavior when there are
// no matching intentions is to allow or deny.
func (p *PolicyAuthorizer) IntentionDefaultAllow() bool {
	// We always go up, this can't be determined by a policy.
	return p.parent.IntentionDefaultAllow()
}

// IntentionRead checks if writing (creating, updating, or deleting) of an
// intention is allowed.
func (p *PolicyAuthorizer) IntentionRead(prefix string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(prefix, p.intentionRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.IntentionRead(prefix)
}

// IntentionWrite checks if writing (creating, updating, or deleting) of an
// intention is allowed.
func (p *PolicyAuthorizer) IntentionWrite(prefix string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(prefix, p.intentionRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyWrite); !recurse {
			// TODO (ACL-V2) - should we do sentinel enforcement here
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.IntentionWrite(prefix)
}

// KeyRead returns if a key is allowed to be read
func (p *PolicyAuthorizer) KeyRead(key string) bool {
	// Look for a matching rule
	if rule, ok := getPolicy(key, p.keyRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.KeyRead(key)
}

// KeyList returns if a key is allowed to be listed
func (p *PolicyAuthorizer) KeyList(key string) bool {
	// Look for a matching rule
	if rule, ok := getPolicy(key, p.keyRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyList); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.KeyList(key)
}

// KeyWrite returns if a key is allowed to be written
func (p *PolicyAuthorizer) KeyWrite(key string, scope sentinel.ScopeFn) bool {
	// Look for a matching rule
	if rule, ok := getPolicy(key, p.keyRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyWrite); !recurse {
			if allow {
				return p.executeCodePolicy(&pr.sentinelPolicy, scope)
			}
			return false
		}
	}

	// No matching rule, use the parent.
	return p.parent.KeyWrite(key, scope)
}

// KeyWritePrefix returns if a prefix is allowed to be written
//
// This is mainly used to detect whether a whole tree within
// the KV can be removed. For that reason we must be able to
// delete everything under the prefix. First we must have "write"
// on the prefix itself
func (p *PolicyAuthorizer) KeyWritePrefix(prefix string) bool {
	parentAllows := p.parent.KeyWritePrefix(prefix)

	// Look for a matching rule that denies
	prefixAllowed := parentAllows
	found := false

	// Look for a prefix rule that would apply to the prefix we are checking
	// WalkPath starts at the root and walks down to the given prefix.
	// Therefore the last prefix rule we see is the one that matters
	p.keyRules.WalkPath(prefix, func(path string, leaf interface{}) bool {
		rule := leaf.(*policyAuthorizerRadixLeaf)

		if rule.prefix != nil {
			found = true
			if rule.prefix.(RulePolicy).aclPolicy != PolicyWrite {
				prefixAllowed = false
			} else {
				prefixAllowed = true
			}
		}
		return false
	})

	// This will be false if we had a prefix that didn't allow write or if
	// there was no prefix rule and the parent policy would deny access.
	if !prefixAllowed {
		return false
	}

	// Look if any of our children do not allow write access. This loop takes
	// into account both prefix and exact match rules.
	deny := false
	p.keyRules.WalkPrefix(prefix, func(path string, leaf interface{}) bool {
		rule := leaf.(*policyAuthorizerRadixLeaf)

		if rule.prefix != nil && rule.prefix.(RulePolicy).aclPolicy != PolicyWrite {
			deny = true
			return true
		}
		if rule.exact != nil && rule.exact.(RulePolicy).aclPolicy != PolicyWrite {
			deny = true
			return true
		}

		return false
	})

	// Deny the write if any sub-rules may be violated
	if deny {
		return false
	}

	// If we had a matching prefix rule and it allowed writes, then we can allow the access
	if found {
		return true
	}

	// No matching rule, use the parent policy.
	return parentAllows
}

// KeyringRead is used to determine if the keyring can be
// read by the current ACL token.
func (p *PolicyAuthorizer) KeyringRead() bool {
	if allow, recurse := enforce(p.keyringRule, PolicyRead); !recurse {
		return allow
	}

	return p.parent.KeyringRead()
}

// KeyringWrite determines if the keyring can be manipulated.
func (p *PolicyAuthorizer) KeyringWrite() bool {
	if allow, recurse := enforce(p.keyringRule, PolicyWrite); !recurse {
		return allow
	}

	return p.parent.KeyringWrite()
}

// OperatorRead determines if the read-only operator functions are allowed.
func (p *PolicyAuthorizer) OperatorRead() bool {
	if allow, recurse := enforce(p.operatorRule, PolicyRead); !recurse {
		return allow
	}

	return p.parent.OperatorRead()
}

// OperatorWrite determines if the state-changing operator functions are
// allowed.
func (p *PolicyAuthorizer) OperatorWrite() bool {
	if allow, recurse := enforce(p.operatorRule, PolicyWrite); !recurse {
		return allow
	}

	return p.parent.OperatorWrite()
}

// NodeRead checks if reading (discovery) of a node is allowed
func (p *PolicyAuthorizer) NodeRead(name string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(name, p.nodeRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyRead); !recurse {
			// TODO (ACL-V2) - Should we do sentinel enforcement here
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.NodeRead(name)
}

// NodeWrite checks if writing (registering) a node is allowed
func (p *PolicyAuthorizer) NodeWrite(name string, scope sentinel.ScopeFn) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(name, p.nodeRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyWrite); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.NodeWrite(name, scope)
}

// PreparedQueryRead checks if reading (listing) of a prepared query is
// allowed - this isn't execution, just listing its contents.
func (p *PolicyAuthorizer) PreparedQueryRead(prefix string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(prefix, p.preparedQueryRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.PreparedQueryRead(prefix)
}

// PreparedQueryWrite checks if writing (creating, updating, or deleting) of a
// prepared query is allowed.
func (p *PolicyAuthorizer) PreparedQueryWrite(prefix string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(prefix, p.preparedQueryRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyWrite); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.PreparedQueryWrite(prefix)
}

// ServiceRead checks if reading (discovery) of a service is allowed
func (p *PolicyAuthorizer) ServiceRead(name string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(name, p.serviceRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.ServiceRead(name)
}

// ServiceWrite checks if writing (registering) a service is allowed
func (p *PolicyAuthorizer) ServiceWrite(name string, scope sentinel.ScopeFn) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(name, p.serviceRules); ok {
		pr := rule.(RulePolicy)
		if allow, recurse := enforce(pr.aclPolicy, PolicyWrite); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.ServiceWrite(name, scope)
}

// SessionRead checks for permission to read sessions for a given node.
func (p *PolicyAuthorizer) SessionRead(node string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(node, p.sessionRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyRead); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.SessionRead(node)
}

// SessionWrite checks for permission to create sessions for a given node.
func (p *PolicyAuthorizer) SessionWrite(node string) bool {
	// Check for an exact rule or catch-all
	if rule, ok := getPolicy(node, p.sessionRules); ok {
		if allow, recurse := enforce(rule.(string), PolicyWrite); !recurse {
			return allow
		}
	}

	// No matching rule, use the parent.
	return p.parent.SessionWrite(node)
}

// executeCodePolicy will run the associated code policy if code policies are
// enabled.
func (p *PolicyAuthorizer) executeCodePolicy(policy *Sentinel, scope sentinel.ScopeFn) bool {
	if p.sentinel == nil {
		return true
	}

	if policy.Code == "" || scope == nil {
		return true
	}

	enforcement := policy.EnforcementLevel
	if enforcement == "" {
		enforcement = DefaultPolicyEnforcementLevel
	}

	return p.sentinel.Execute(policy.Code, enforcement, scope())
}
