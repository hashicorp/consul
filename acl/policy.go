package acl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/sentinel"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	hclprinter "github.com/hashicorp/hcl/hcl/printer"
	"github.com/hashicorp/hcl/hcl/token"
	"golang.org/x/crypto/blake2b"
)

type SyntaxVersion int

const (
	SyntaxCurrent SyntaxVersion = iota
	SyntaxLegacy
)

const (
	PolicyDeny  = "deny"
	PolicyRead  = "read"
	PolicyWrite = "write"
	PolicyList  = "list"
)

// Policy is used to represent the policy specified by
// an ACL configuration.
type Policy struct {
	ID                    string                 `hcl:"id"`
	Revision              uint64                 `hcl:"revision"`
	ACL                   string                 `hcl:"acl,expand"`
	Agents                []*AgentPolicy         `hcl:"agent,expand"`
	AgentPrefixes         []*AgentPolicy         `hcl:"agent_prefix,expand"`
	Keys                  []*KeyPolicy           `hcl:"key,expand"`
	KeyPrefixes           []*KeyPolicy           `hcl:"key_prefix,expand"`
	Nodes                 []*NodePolicy          `hcl:"node,expand"`
	NodePrefixes          []*NodePolicy          `hcl:"node_prefix,expand"`
	Services              []*ServicePolicy       `hcl:"service,expand"`
	ServicePrefixes       []*ServicePolicy       `hcl:"service_prefix,expand"`
	Sessions              []*SessionPolicy       `hcl:"session,expand"`
	SessionPrefixes       []*SessionPolicy       `hcl:"session_prefix,expand"`
	Events                []*EventPolicy         `hcl:"event,expand"`
	EventPrefixes         []*EventPolicy         `hcl:"event_prefix,expand"`
	PreparedQueries       []*PreparedQueryPolicy `hcl:"query,expand"`
	PreparedQueryPrefixes []*PreparedQueryPolicy `hcl:"query_prefix,expand"`
	Keyring               string                 `hcl:"keyring"`
	Operator              string                 `hcl:"operator"`
}

// Sentinel defines a snippet of Sentinel code that can be attached to a policy.
type Sentinel struct {
	Code             string
	EnforcementLevel string
}

// AgentPolicy represents a policy for working with agent endpoints on nodes
// with specific name prefixes.
type AgentPolicy struct {
	Node   string `hcl:",key"`
	Policy string
}

func (a *AgentPolicy) GoString() string {
	return fmt.Sprintf("%#v", *a)
}

// KeyPolicy represents a policy for a key
type KeyPolicy struct {
	Prefix   string `hcl:",key"`
	Policy   string
	Sentinel Sentinel
}

func (k *KeyPolicy) GoString() string {
	return fmt.Sprintf("%#v", *k)
}

// NodePolicy represents a policy for a node
type NodePolicy struct {
	Name     string `hcl:",key"`
	Policy   string
	Sentinel Sentinel
}

func (n *NodePolicy) GoString() string {
	return fmt.Sprintf("%#v", *n)
}

// ServicePolicy represents a policy for a service
type ServicePolicy struct {
	Name     string `hcl:",key"`
	Policy   string
	Sentinel Sentinel

	// Intentions is the policy for intentions where this service is the
	// destination. This may be empty, in which case the Policy determines
	// the intentions policy.
	Intentions string
}

func (s *ServicePolicy) GoString() string {
	return fmt.Sprintf("%#v", *s)
}

// SessionPolicy represents a policy for making sessions tied to specific node
// name prefixes.
type SessionPolicy struct {
	Node   string `hcl:",key"`
	Policy string
}

func (s *SessionPolicy) GoString() string {
	return fmt.Sprintf("%#v", *s)
}

// EventPolicy represents a user event policy.
type EventPolicy struct {
	Event  string `hcl:",key"`
	Policy string
}

func (e *EventPolicy) GoString() string {
	return fmt.Sprintf("%#v", *e)
}

// PreparedQueryPolicy represents a prepared query policy.
type PreparedQueryPolicy struct {
	Prefix string `hcl:",key"`
	Policy string
}

func (p *PreparedQueryPolicy) GoString() string {
	return fmt.Sprintf("%#v", *p)
}

// isPolicyValid makes sure the given string matches one of the valid policies.
func isPolicyValid(policy string) bool {
	switch policy {
	case PolicyDeny:
		return true
	case PolicyRead:
		return true
	case PolicyWrite:
		return true
	default:
		return false
	}
}

// isSentinelValid makes sure the given sentinel block is valid, and will skip
// out if the evaluator is nil.
func isSentinelValid(sentinel sentinel.Evaluator, basicPolicy string, sp Sentinel) error {
	// Sentinel not enabled at all, or for this policy.
	if sentinel == nil {
		return nil
	}
	if sp.Code == "" {
		return nil
	}

	// We only allow sentinel code on write policies at this time.
	if basicPolicy != PolicyWrite {
		return fmt.Errorf("code is only allowed for write policies")
	}

	// Validate the sentinel parts.
	switch sp.EnforcementLevel {
	case "", "soft-mandatory", "hard-mandatory":
		// OK
	default:
		return fmt.Errorf("unsupported enforcement level %q", sp.EnforcementLevel)
	}
	return sentinel.Compile(sp.Code)
}

func parseCurrent(rules string, sentinel sentinel.Evaluator) (*Policy, error) {
	p := &Policy{}

	if err := hcl.Decode(p, rules); err != nil {
		return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
	}

	// Validate the acl policy
	if p.ACL != "" && !isPolicyValid(p.ACL) {
		return nil, fmt.Errorf("Invalid acl policy: %#v", p.ACL)
	}

	// Validate the agent policy
	for _, ap := range p.Agents {
		if !isPolicyValid(ap.Policy) {
			return nil, fmt.Errorf("Invalid agent policy: %#v", ap)
		}
	}
	for _, ap := range p.AgentPrefixes {
		if !isPolicyValid(ap.Policy) {
			return nil, fmt.Errorf("Invalid agent_prefix policy: %#v", ap)
		}
	}

	// Validate the key policy
	for _, kp := range p.Keys {
		if kp.Policy != PolicyList && !isPolicyValid(kp.Policy) {
			return nil, fmt.Errorf("Invalid key policy: %#v", kp)
		}
		if err := isSentinelValid(sentinel, kp.Policy, kp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid key Sentinel policy: %#v, got error:%v", kp, err)
		}
	}
	for _, kp := range p.KeyPrefixes {
		if kp.Policy != PolicyList && !isPolicyValid(kp.Policy) {
			return nil, fmt.Errorf("Invalid key_prefix policy: %#v", kp)
		}
		if err := isSentinelValid(sentinel, kp.Policy, kp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid key_prefix Sentinel policy: %#v, got error:%v", kp, err)
		}
	}

	// Validate the node policies
	for _, np := range p.Nodes {
		if !isPolicyValid(np.Policy) {
			return nil, fmt.Errorf("Invalid node policy: %#v", np)
		}
		if err := isSentinelValid(sentinel, np.Policy, np.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid node Sentinel policy: %#v, got error:%v", np, err)
		}
	}
	for _, np := range p.NodePrefixes {
		if !isPolicyValid(np.Policy) {
			return nil, fmt.Errorf("Invalid node_prefix policy: %#v", np)
		}
		if err := isSentinelValid(sentinel, np.Policy, np.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid node_prefix Sentinel policy: %#v, got error:%v", np, err)
		}
	}

	// Validate the service policies
	for _, sp := range p.Services {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid service policy: %#v", sp)
		}
		if sp.Intentions != "" && !isPolicyValid(sp.Intentions) {
			return nil, fmt.Errorf("Invalid service intentions policy: %#v", sp)
		}
		if err := isSentinelValid(sentinel, sp.Policy, sp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid service Sentinel policy: %#v, got error:%v", sp, err)
		}
	}
	for _, sp := range p.ServicePrefixes {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid service_prefix policy: %#v", sp)
		}
		if sp.Intentions != "" && !isPolicyValid(sp.Intentions) {
			return nil, fmt.Errorf("Invalid service_prefix intentions policy: %#v", sp)
		}
		if err := isSentinelValid(sentinel, sp.Policy, sp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid service_prefix Sentinel policy: %#v, got error:%v", sp, err)
		}
	}

	// Validate the session policies
	for _, sp := range p.Sessions {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid session policy: %#v", sp)
		}
	}
	for _, sp := range p.SessionPrefixes {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid session_prefix policy: %#v", sp)
		}
	}

	// Validate the user event policies
	for _, ep := range p.Events {
		if !isPolicyValid(ep.Policy) {
			return nil, fmt.Errorf("Invalid event policy: %#v", ep)
		}
	}
	for _, ep := range p.EventPrefixes {
		if !isPolicyValid(ep.Policy) {
			return nil, fmt.Errorf("Invalid event_prefix policy: %#v", ep)
		}
	}

	// Validate the prepared query policies
	for _, pq := range p.PreparedQueries {
		if !isPolicyValid(pq.Policy) {
			return nil, fmt.Errorf("Invalid query policy: %#v", pq)
		}
	}
	for _, pq := range p.PreparedQueryPrefixes {
		if !isPolicyValid(pq.Policy) {
			return nil, fmt.Errorf("Invalid query_prefix policy: %#v", pq)
		}
	}

	// Validate the keyring policy - this one is allowed to be empty
	if p.Keyring != "" && !isPolicyValid(p.Keyring) {
		return nil, fmt.Errorf("Invalid keyring policy: %#v", p.Keyring)
	}

	// Validate the operator policy - this one is allowed to be empty
	if p.Operator != "" && !isPolicyValid(p.Operator) {
		return nil, fmt.Errorf("Invalid operator policy: %#v", p.Operator)
	}

	return p, nil
}

func parseLegacy(rules string, sentinel sentinel.Evaluator) (*Policy, error) {
	p := &Policy{}

	type LegacyPolicy struct {
		Agents          []*AgentPolicy         `hcl:"agent,expand"`
		Keys            []*KeyPolicy           `hcl:"key,expand"`
		Nodes           []*NodePolicy          `hcl:"node,expand"`
		Services        []*ServicePolicy       `hcl:"service,expand"`
		Sessions        []*SessionPolicy       `hcl:"session,expand"`
		Events          []*EventPolicy         `hcl:"event,expand"`
		PreparedQueries []*PreparedQueryPolicy `hcl:"query,expand"`
		Keyring         string                 `hcl:"keyring"`
		Operator        string                 `hcl:"operator"`
	}

	lp := &LegacyPolicy{}

	if err := hcl.Decode(lp, rules); err != nil {
		return nil, fmt.Errorf("Failed to parse ACL rules: %v", err)
	}

	// Validate the agent policy
	for _, ap := range lp.Agents {
		if !isPolicyValid(ap.Policy) {
			return nil, fmt.Errorf("Invalid agent policy: %#v", ap)
		}

		p.AgentPrefixes = append(p.AgentPrefixes, ap)
	}

	// Validate the key policy
	for _, kp := range lp.Keys {
		if kp.Policy != PolicyList && !isPolicyValid(kp.Policy) {
			return nil, fmt.Errorf("Invalid key policy: %#v", kp)
		}
		if err := isSentinelValid(sentinel, kp.Policy, kp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid key Sentinel policy: %#v, got error:%v", kp, err)
		}

		p.KeyPrefixes = append(p.KeyPrefixes, kp)
	}

	// Validate the node policies
	for _, np := range lp.Nodes {
		if !isPolicyValid(np.Policy) {
			return nil, fmt.Errorf("Invalid node policy: %#v", np)
		}
		if err := isSentinelValid(sentinel, np.Policy, np.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid node Sentinel policy: %#v, got error:%v", np, err)
		}

		p.NodePrefixes = append(p.NodePrefixes, np)
	}

	// Validate the service policies
	for _, sp := range lp.Services {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid service policy: %#v", sp)
		}
		if sp.Intentions != "" && !isPolicyValid(sp.Intentions) {
			return nil, fmt.Errorf("Invalid service intentions policy: %#v", sp)
		}
		if err := isSentinelValid(sentinel, sp.Policy, sp.Sentinel); err != nil {
			return nil, fmt.Errorf("Invalid service Sentinel policy: %#v, got error:%v", sp, err)
		}

		p.ServicePrefixes = append(p.ServicePrefixes, sp)
	}

	// Validate the session policies
	for _, sp := range lp.Sessions {
		if !isPolicyValid(sp.Policy) {
			return nil, fmt.Errorf("Invalid session policy: %#v", sp)
		}

		p.SessionPrefixes = append(p.SessionPrefixes, sp)
	}

	// Validate the user event policies
	for _, ep := range lp.Events {
		if !isPolicyValid(ep.Policy) {
			return nil, fmt.Errorf("Invalid event policy: %#v", ep)
		}

		p.EventPrefixes = append(p.EventPrefixes, ep)
	}

	// Validate the prepared query policies
	for _, pq := range lp.PreparedQueries {
		if !isPolicyValid(pq.Policy) {
			return nil, fmt.Errorf("Invalid query policy: %#v", pq)
		}

		p.PreparedQueryPrefixes = append(p.PreparedQueryPrefixes, pq)
	}

	// Validate the keyring policy - this one is allowed to be empty
	if lp.Keyring != "" && !isPolicyValid(lp.Keyring) {
		return nil, fmt.Errorf("Invalid keyring policy: %#v", lp.Keyring)
	} else {
		p.Keyring = lp.Keyring
	}

	// Validate the operator policy - this one is allowed to be empty
	if lp.Operator != "" && !isPolicyValid(lp.Operator) {
		return nil, fmt.Errorf("Invalid operator policy: %#v", lp.Operator)
	} else {
		p.Operator = lp.Operator
	}

	return p, nil
}

// NewPolicyFromSource is used to parse the specified ACL rules into an
// intermediary set of policies, before being compiled into
// the ACL
func NewPolicyFromSource(id string, revision uint64, rules string, syntax SyntaxVersion, sentinel sentinel.Evaluator) (*Policy, error) {
	if rules == "" {
		// Hot path for empty source
		return &Policy{ID: id, Revision: revision}, nil
	}

	var policy *Policy
	var err error
	switch syntax {
	case SyntaxLegacy:
		policy, err = parseLegacy(rules, sentinel)
	case SyntaxCurrent:
		policy, err = parseCurrent(rules, sentinel)
	default:
		return nil, fmt.Errorf("Invalid rules version: %d", syntax)
	}

	if err == nil {
		policy.ID = id
		policy.Revision = revision
	}
	return policy, err
}

func (policy *Policy) ConvertToLegacy() *Policy {
	converted := &Policy{
		ID:       policy.ID,
		Revision: policy.Revision,
		ACL:      policy.ACL,
		Keyring:  policy.Keyring,
		Operator: policy.Operator,
	}

	converted.Agents = append(converted.Agents, policy.Agents...)
	converted.Agents = append(converted.Agents, policy.AgentPrefixes...)
	converted.Keys = append(converted.Keys, policy.Keys...)
	converted.Keys = append(converted.Keys, policy.KeyPrefixes...)
	converted.Nodes = append(converted.Nodes, policy.Nodes...)
	converted.Nodes = append(converted.Nodes, policy.NodePrefixes...)
	converted.Services = append(converted.Services, policy.Services...)
	converted.Services = append(converted.Services, policy.ServicePrefixes...)
	converted.Sessions = append(converted.Sessions, policy.Sessions...)
	converted.Sessions = append(converted.Sessions, policy.SessionPrefixes...)
	converted.Events = append(converted.Events, policy.Events...)
	converted.Events = append(converted.Events, policy.EventPrefixes...)
	converted.PreparedQueries = append(converted.PreparedQueries, policy.PreparedQueries...)
	converted.PreparedQueries = append(converted.PreparedQueries, policy.PreparedQueryPrefixes...)
	return converted
}

func (policy *Policy) ConvertFromLegacy() *Policy {
	return &Policy{
		ID:                    policy.ID,
		Revision:              policy.Revision,
		AgentPrefixes:         policy.Agents,
		KeyPrefixes:           policy.Keys,
		NodePrefixes:          policy.Nodes,
		ServicePrefixes:       policy.Services,
		SessionPrefixes:       policy.Sessions,
		EventPrefixes:         policy.Events,
		PreparedQueryPrefixes: policy.PreparedQueries,
		Keyring:               policy.Keyring,
		Operator:              policy.Operator,
	}
}

// takesPrecedenceOver returns true when permission a
// should take precedence over permission b
func takesPrecedenceOver(a, b string) bool {
	if a == PolicyDeny {
		return true
	} else if b == PolicyDeny {
		return false
	}

	if a == PolicyWrite {
		return true
	} else if b == PolicyWrite {
		return false
	}

	if a == PolicyList {
		return true
	} else if b == PolicyList {
		return false
	}

	if a == PolicyRead {
		return true
	} else if b == PolicyRead {
		return false
	}

	return false
}

func multiPolicyID(policies []*Policy) []byte {
	cacheKeyHash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, policy := range policies {
		cacheKeyHash.Write([]byte(policy.ID))
		binary.Write(cacheKeyHash, binary.BigEndian, policy.Revision)
	}
	return cacheKeyHash.Sum(nil)
}

// MergePolicies merges multiple ACL policies into one policy
// This function will not set either the ID or the Scope fields
// of the resulting policy as its up to the caller to determine
// what the merged value is.
func MergePolicies(policies []*Policy) *Policy {
	// maps are used here so that we can lookup each policy by
	// the segment that the rule applies to during the policy
	// merge. Otherwise we could do a linear search through a slice
	// and replace it inline
	aclPolicy := ""
	agentPolicies := make(map[string]*AgentPolicy)
	agentPrefixPolicies := make(map[string]*AgentPolicy)
	eventPolicies := make(map[string]*EventPolicy)
	eventPrefixPolicies := make(map[string]*EventPolicy)
	keyringPolicy := ""
	keyPolicies := make(map[string]*KeyPolicy)
	keyPrefixPolicies := make(map[string]*KeyPolicy)
	nodePolicies := make(map[string]*NodePolicy)
	nodePrefixPolicies := make(map[string]*NodePolicy)
	operatorPolicy := ""
	preparedQueryPolicies := make(map[string]*PreparedQueryPolicy)
	preparedQueryPrefixPolicies := make(map[string]*PreparedQueryPolicy)
	servicePolicies := make(map[string]*ServicePolicy)
	servicePrefixPolicies := make(map[string]*ServicePolicy)
	sessionPolicies := make(map[string]*SessionPolicy)
	sessionPrefixPolicies := make(map[string]*SessionPolicy)

	// Parse all the individual rule sets
	for _, policy := range policies {
		if takesPrecedenceOver(policy.ACL, aclPolicy) {
			aclPolicy = policy.ACL
		}

		for _, ap := range policy.Agents {
			update := true
			if permission, found := agentPolicies[ap.Node]; found {
				update = takesPrecedenceOver(ap.Policy, permission.Policy)
			}

			if update {

				agentPolicies[ap.Node] = ap
			}
		}

		for _, ap := range policy.AgentPrefixes {
			update := true
			if permission, found := agentPrefixPolicies[ap.Node]; found {
				update = takesPrecedenceOver(ap.Policy, permission.Policy)
			}

			if update {
				agentPrefixPolicies[ap.Node] = ap
			}
		}

		for _, ep := range policy.Events {
			update := true
			if permission, found := eventPolicies[ep.Event]; found {
				update = takesPrecedenceOver(ep.Policy, permission.Policy)
			}

			if update {
				eventPolicies[ep.Event] = ep
			}
		}

		for _, ep := range policy.EventPrefixes {
			update := true
			if permission, found := eventPrefixPolicies[ep.Event]; found {
				update = takesPrecedenceOver(ep.Policy, permission.Policy)
			}

			if update {
				eventPrefixPolicies[ep.Event] = ep
			}
		}

		if takesPrecedenceOver(policy.Keyring, keyringPolicy) {
			keyringPolicy = policy.Keyring
		}

		for _, kp := range policy.Keys {
			update := true
			if permission, found := keyPolicies[kp.Prefix]; found {
				update = takesPrecedenceOver(kp.Policy, permission.Policy)
			}

			if update {
				keyPolicies[kp.Prefix] = kp
			}
		}

		for _, kp := range policy.KeyPrefixes {
			update := true
			if permission, found := keyPrefixPolicies[kp.Prefix]; found {
				update = takesPrecedenceOver(kp.Policy, permission.Policy)
			}

			if update {
				keyPrefixPolicies[kp.Prefix] = kp
			}
		}

		for _, np := range policy.Nodes {
			update := true
			if permission, found := nodePolicies[np.Name]; found {
				update = takesPrecedenceOver(np.Policy, permission.Policy)
			}

			if update {
				nodePolicies[np.Name] = np
			}
		}

		for _, np := range policy.NodePrefixes {
			update := true
			if permission, found := nodePrefixPolicies[np.Name]; found {
				update = takesPrecedenceOver(np.Policy, permission.Policy)
			}

			if update {
				nodePrefixPolicies[np.Name] = np
			}
		}

		if takesPrecedenceOver(policy.Operator, operatorPolicy) {
			operatorPolicy = policy.Operator
		}

		for _, qp := range policy.PreparedQueries {
			update := true
			if permission, found := preparedQueryPolicies[qp.Prefix]; found {
				update = takesPrecedenceOver(qp.Policy, permission.Policy)
			}

			if update {
				preparedQueryPolicies[qp.Prefix] = qp
			}
		}

		for _, qp := range policy.PreparedQueryPrefixes {
			update := true
			if permission, found := preparedQueryPrefixPolicies[qp.Prefix]; found {
				update = takesPrecedenceOver(qp.Policy, permission.Policy)
			}

			if update {
				preparedQueryPrefixPolicies[qp.Prefix] = qp
			}
		}

		for _, sp := range policy.Services {
			existing, found := servicePolicies[sp.Name]

			if !found {
				servicePolicies[sp.Name] = sp
				continue
			}

			if takesPrecedenceOver(sp.Policy, existing.Policy) {
				existing.Policy = sp.Policy
				existing.Sentinel = sp.Sentinel
			}

			if takesPrecedenceOver(sp.Intentions, existing.Intentions) {
				existing.Intentions = sp.Intentions
			}
		}

		for _, sp := range policy.ServicePrefixes {
			existing, found := servicePrefixPolicies[sp.Name]

			if !found {
				servicePrefixPolicies[sp.Name] = sp
				continue
			}

			if takesPrecedenceOver(sp.Policy, existing.Policy) {
				existing.Policy = sp.Policy
				existing.Sentinel = sp.Sentinel
			}

			if takesPrecedenceOver(sp.Intentions, existing.Intentions) {
				existing.Intentions = sp.Intentions
			}
		}

		for _, sp := range policy.Sessions {
			update := true
			if permission, found := sessionPolicies[sp.Node]; found {
				update = takesPrecedenceOver(sp.Policy, permission.Policy)
			}

			if update {
				sessionPolicies[sp.Node] = sp
			}
		}

		for _, sp := range policy.SessionPrefixes {
			update := true
			if permission, found := sessionPrefixPolicies[sp.Node]; found {
				update = takesPrecedenceOver(sp.Policy, permission.Policy)
			}

			if update {
				sessionPrefixPolicies[sp.Node] = sp
			}
		}
	}

	merged := &Policy{ACL: aclPolicy, Keyring: keyringPolicy, Operator: operatorPolicy}

	// All the for loop appends are ugly but Go doesn't have a way to get
	// a slice of all values within a map so this is necessary

	for _, policy := range agentPolicies {
		merged.Agents = append(merged.Agents, policy)
	}

	for _, policy := range agentPrefixPolicies {
		merged.AgentPrefixes = append(merged.AgentPrefixes, policy)
	}

	for _, policy := range eventPolicies {
		merged.Events = append(merged.Events, policy)
	}

	for _, policy := range eventPrefixPolicies {
		merged.EventPrefixes = append(merged.EventPrefixes, policy)
	}

	for _, policy := range keyPolicies {
		merged.Keys = append(merged.Keys, policy)
	}

	for _, policy := range keyPrefixPolicies {
		merged.KeyPrefixes = append(merged.KeyPrefixes, policy)
	}

	for _, policy := range nodePolicies {
		merged.Nodes = append(merged.Nodes, policy)
	}

	for _, policy := range nodePrefixPolicies {
		merged.NodePrefixes = append(merged.NodePrefixes, policy)
	}

	for _, policy := range preparedQueryPolicies {
		merged.PreparedQueries = append(merged.PreparedQueries, policy)
	}

	for _, policy := range preparedQueryPrefixPolicies {
		merged.PreparedQueryPrefixes = append(merged.PreparedQueryPrefixes, policy)
	}

	for _, policy := range servicePolicies {
		merged.Services = append(merged.Services, policy)
	}

	for _, policy := range servicePrefixPolicies {
		merged.ServicePrefixes = append(merged.ServicePrefixes, policy)
	}

	for _, policy := range sessionPolicies {
		merged.Sessions = append(merged.Sessions, policy)
	}

	for _, policy := range sessionPrefixPolicies {
		merged.SessionPrefixes = append(merged.SessionPrefixes, policy)
	}

	merged.ID = fmt.Sprintf("%x", multiPolicyID(policies))

	return merged
}

func TranslateLegacyRules(policyBytes []byte) ([]byte, error) {
	parsed, err := hcl.ParseBytes(policyBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse rules: %v", err)
	}

	rewritten := ast.Walk(parsed, func(node ast.Node) (ast.Node, bool) {
		switch n := node.(type) {
		case *ast.ObjectItem:
			if len(n.Keys) < 1 {
				return node, true
			}

			txt := n.Keys[0].Token.Text
			if n.Keys[0].Token.Type == token.STRING {
				txt, err = strconv.Unquote(txt)
				if err != nil {
					return node, true
				}
			}

			switch txt {
			case "policy":
				n.Keys[0].Token.Text = "policy"
			case "agent":
				n.Keys[0].Token.Text = "agent_prefix"
			case "key":
				n.Keys[0].Token.Text = "key_prefix"
			case "node":
				n.Keys[0].Token.Text = "node_prefix"
			case "query":
				n.Keys[0].Token.Text = "query_prefix"
			case "service":
				n.Keys[0].Token.Text = "service_prefix"
			case "session":
				n.Keys[0].Token.Text = "session_prefix"
			case "event":
				n.Keys[0].Token.Text = "event_prefix"
			}
		}

		return node, true
	})

	buffer := new(bytes.Buffer)

	if err := hclprinter.Fprint(buffer, rewritten); err != nil {
		return nil, fmt.Errorf("Failed to output new rules: %v", err)
	}

	return buffer.Bytes(), nil
}
