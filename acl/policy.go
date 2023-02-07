package acl

import (
	"fmt"
	"strings"
)

const (
	PolicyDeny  = "deny"
	PolicyRead  = "read"
	PolicyList  = "list"
	PolicyWrite = "write"
)

type AccessLevel int

const (
	AccessUnknown AccessLevel = iota
	AccessDeny
	AccessRead
	AccessList
	AccessWrite
)

func (l AccessLevel) String() string {
	switch l {
	case AccessDeny:
		return PolicyDeny
	case AccessRead:
		return PolicyRead
	case AccessList:
		return PolicyList
	case AccessWrite:
		return PolicyWrite
	default:
		return "unknown"
	}
}

func AccessLevelFromString(level string) (AccessLevel, error) {
	switch strings.ToLower(level) {
	case PolicyDeny:
		return AccessDeny, nil
	case PolicyRead:
		return AccessRead, nil
	case PolicyList:
		return AccessList, nil
	case PolicyWrite:
		return AccessWrite, nil
	default:
		return AccessUnknown, fmt.Errorf("%q is not a valid access level", level)
	}
}

type PolicyRules struct {
	ACL                   string               `hcl:"acl,expand"`
	Agents                []*AgentRule         `hcl:"agent,expand"`
	AgentPrefixes         []*AgentRule         `hcl:"agent_prefix,expand"`
	Keys                  []*KeyRule           `hcl:"key,expand"`
	KeyPrefixes           []*KeyRule           `hcl:"key_prefix,expand"`
	Nodes                 []*NodeRule          `hcl:"node,expand"`
	NodePrefixes          []*NodeRule          `hcl:"node_prefix,expand"`
	Services              []*ServiceRule       `hcl:"service,expand"`
	ServicePrefixes       []*ServiceRule       `hcl:"service_prefix,expand"`
	Sessions              []*SessionRule       `hcl:"session,expand"`
	SessionPrefixes       []*SessionRule       `hcl:"session_prefix,expand"`
	Events                []*EventRule         `hcl:"event,expand"`
	EventPrefixes         []*EventRule         `hcl:"event_prefix,expand"`
	PreparedQueries       []*PreparedQueryRule `hcl:"query,expand"`
	PreparedQueryPrefixes []*PreparedQueryRule `hcl:"query_prefix,expand"`
	Keyring               string               `hcl:"keyring"`
	Operator              string               `hcl:"operator"`
	Mesh                  string               `hcl:"mesh"`
	Peering               string               `hcl:"peering"`
}

// Policy is used to represent the policy specified by an ACL configuration.
type Policy struct {
	PolicyRules           `hcl:",squash"`
	EnterprisePolicyRules `hcl:",squash"`
}

// AgentRule represents a rule for working with agent endpoints on nodes
// with specific name prefixes.
type AgentRule struct {
	Node   string `hcl:",key"`
	Policy string
}

// KeyRule represents a rule for a key
type KeyRule struct {
	Prefix string `hcl:",key"`
	Policy string

	EnterpriseRule `hcl:",squash"`
}

// NodeRule represents a rule for a node
type NodeRule struct {
	Name   string `hcl:",key"`
	Policy string

	EnterpriseRule `hcl:",squash"`
}

// ServiceRule represents a policy for a service
type ServiceRule struct {
	Name   string `hcl:",key"`
	Policy string

	// Intentions is the policy for intentions where this service is the
	// destination. This may be empty, in which case the Policy determines
	// the intentions policy.
	Intentions string

	EnterpriseRule `hcl:",squash"`
}

// SessionRule represents a rule for making sessions tied to specific node
// name prefixes.
type SessionRule struct {
	Node   string `hcl:",key"`
	Policy string
}

// EventRule represents a user event rule.
type EventRule struct {
	Event  string `hcl:",key"`
	Policy string
}

// PreparedQueryRule represents a prepared query rule.
type PreparedQueryRule struct {
	Prefix string `hcl:",key"`
	Policy string
}

// isPolicyValid makes sure the given string matches one of the valid policies.
func isPolicyValid(policy string, allowList bool) bool {
	access, err := AccessLevelFromString(policy)
	if err != nil {
		return false
	}
	if access == AccessList && !allowList {
		return false
	}
	return true
}

func (pr *PolicyRules) Validate(conf *Config) error {
	// Validate the acl policy - this one is allowed to be empty
	if pr.ACL != "" && !isPolicyValid(pr.ACL, false) {
		return fmt.Errorf("Invalid acl policy: %#v", pr.ACL)
	}

	// Validate the agent policy
	for _, ap := range pr.Agents {
		if !isPolicyValid(ap.Policy, false) {
			return fmt.Errorf("Invalid agent policy: %#v", ap)
		}
	}
	for _, ap := range pr.AgentPrefixes {
		if !isPolicyValid(ap.Policy, false) {
			return fmt.Errorf("Invalid agent_prefix policy: %#v", ap)
		}
	}

	// Validate the key policy
	for _, kp := range pr.Keys {
		if !isPolicyValid(kp.Policy, true) {
			return fmt.Errorf("Invalid key policy: %#v", kp)
		}
		if err := kp.EnterpriseRule.Validate(kp.Policy, conf); err != nil {
			return fmt.Errorf("Invalid key enterprise policy: %#v, got error: %v", kp, err)
		}
	}
	for _, kp := range pr.KeyPrefixes {
		if !isPolicyValid(kp.Policy, true) {
			return fmt.Errorf("Invalid key_prefix policy: %#v", kp)
		}
		if err := kp.EnterpriseRule.Validate(kp.Policy, conf); err != nil {
			return fmt.Errorf("Invalid key_prefix enterprise policy: %#v, got error: %v", kp, err)
		}
	}

	// Validate the node policies
	for _, np := range pr.Nodes {
		if !isPolicyValid(np.Policy, false) {
			return fmt.Errorf("Invalid node policy: %#v", np)
		}
		if err := np.EnterpriseRule.Validate(np.Policy, conf); err != nil {
			return fmt.Errorf("Invalid node enterprise policy: %#v, got error: %v", np, err)
		}
	}
	for _, np := range pr.NodePrefixes {
		if !isPolicyValid(np.Policy, false) {
			return fmt.Errorf("Invalid node_prefix policy: %#v", np)
		}
		if err := np.EnterpriseRule.Validate(np.Policy, conf); err != nil {
			return fmt.Errorf("Invalid node_prefix enterprise policy: %#v, got error: %v", np, err)
		}
	}

	// Validate the service policies
	for _, sp := range pr.Services {
		if !isPolicyValid(sp.Policy, false) {
			return fmt.Errorf("Invalid service policy: %#v", sp)
		}
		if sp.Intentions != "" && !isPolicyValid(sp.Intentions, false) {
			return fmt.Errorf("Invalid service intentions policy: %#v", sp)
		}
		if err := sp.EnterpriseRule.Validate(sp.Policy, conf); err != nil {
			return fmt.Errorf("Invalid service enterprise policy: %#v, got error: %v", sp, err)
		}
	}
	for _, sp := range pr.ServicePrefixes {
		if !isPolicyValid(sp.Policy, false) {
			return fmt.Errorf("Invalid service_prefix policy: %#v", sp)
		}
		if sp.Intentions != "" && !isPolicyValid(sp.Intentions, false) {
			return fmt.Errorf("Invalid service_prefix intentions policy: %#v", sp)
		}
		if err := sp.EnterpriseRule.Validate(sp.Policy, conf); err != nil {
			return fmt.Errorf("Invalid service_prefix enterprise policy: %#v, got error: %v", sp, err)
		}
	}

	// Validate the session policies
	for _, sp := range pr.Sessions {
		if !isPolicyValid(sp.Policy, false) {
			return fmt.Errorf("Invalid session policy: %#v", sp)
		}
	}
	for _, sp := range pr.SessionPrefixes {
		if !isPolicyValid(sp.Policy, false) {
			return fmt.Errorf("Invalid session_prefix policy: %#v", sp)
		}
	}

	// Validate the user event policies
	for _, ep := range pr.Events {
		if !isPolicyValid(ep.Policy, false) {
			return fmt.Errorf("Invalid event policy: %#v", ep)
		}
	}
	for _, ep := range pr.EventPrefixes {
		if !isPolicyValid(ep.Policy, false) {
			return fmt.Errorf("Invalid event_prefix policy: %#v", ep)
		}
	}

	// Validate the prepared query policies
	for _, pq := range pr.PreparedQueries {
		if !isPolicyValid(pq.Policy, false) {
			return fmt.Errorf("Invalid query policy: %#v", pq)
		}
	}
	for _, pq := range pr.PreparedQueryPrefixes {
		if !isPolicyValid(pq.Policy, false) {
			return fmt.Errorf("Invalid query_prefix policy: %#v", pq)
		}
	}

	// Validate the keyring policy - this one is allowed to be empty
	if pr.Keyring != "" && !isPolicyValid(pr.Keyring, false) {
		return fmt.Errorf("Invalid keyring policy: %#v", pr.Keyring)
	}

	// Validate the operator policy - this one is allowed to be empty
	if pr.Operator != "" && !isPolicyValid(pr.Operator, false) {
		return fmt.Errorf("Invalid operator policy: %#v", pr.Operator)
	}

	// Validate the mesh policy - this one is allowed to be empty
	if pr.Mesh != "" && !isPolicyValid(pr.Mesh, false) {
		return fmt.Errorf("Invalid mesh policy: %#v", pr.Mesh)
	}

	// Validate the peering policy - this one is allowed to be empty
	if pr.Peering != "" && !isPolicyValid(pr.Peering, false) {
		return fmt.Errorf("Invalid peering policy: %#v", pr.Peering)
	}
	return nil
}

func parse(rules string, conf *Config, meta *EnterprisePolicyMeta) (*Policy, error) {
	p, err := decodeRules(rules, conf, meta)
	if err != nil {
		return nil, err
	}

	if err := p.PolicyRules.Validate(conf); err != nil {
		return nil, err
	}

	if err := p.EnterprisePolicyRules.Validate(conf); err != nil {
		return nil, err
	}

	return p, nil
}

// NewPolicyFromSource is used to parse the specified ACL rules into an
// intermediary set of policies, before being compiled into
// the ACL
func NewPolicyFromSource(rules string, conf *Config, meta *EnterprisePolicyMeta) (*Policy, error) {
	if rules == "" {
		// Hot path for empty source
		return &Policy{}, nil
	}

	var policy *Policy
	var err error
	policy, err = parse(rules, conf, meta)
	return policy, err
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
