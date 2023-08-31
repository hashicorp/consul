// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

type policyRulesMergeContext struct {
	aclRule                  string
	agentRules               map[string]*AgentRule
	agentPrefixRules         map[string]*AgentRule
	eventRules               map[string]*EventRule
	eventPrefixRules         map[string]*EventRule
	keyringRule              string
	keyRules                 map[string]*KeyRule
	keyPrefixRules           map[string]*KeyRule
	meshRule                 string
	peeringRule              string
	nodeRules                map[string]*NodeRule
	nodePrefixRules          map[string]*NodeRule
	operatorRule             string
	preparedQueryRules       map[string]*PreparedQueryRule
	preparedQueryPrefixRules map[string]*PreparedQueryRule
	serviceRules             map[string]*ServiceRule
	servicePrefixRules       map[string]*ServiceRule
	sessionRules             map[string]*SessionRule
	sessionPrefixRules       map[string]*SessionRule
	// namespaceRule is an enterprise-only field
	namespaceRule string
}

func (p *policyRulesMergeContext) init() {
	p.aclRule = ""
	p.agentRules = make(map[string]*AgentRule)
	p.agentPrefixRules = make(map[string]*AgentRule)
	p.eventRules = make(map[string]*EventRule)
	p.eventPrefixRules = make(map[string]*EventRule)
	p.keyringRule = ""
	p.keyRules = make(map[string]*KeyRule)
	p.keyPrefixRules = make(map[string]*KeyRule)
	p.meshRule = ""
	p.peeringRule = ""
	p.nodeRules = make(map[string]*NodeRule)
	p.nodePrefixRules = make(map[string]*NodeRule)
	p.operatorRule = ""
	p.preparedQueryRules = make(map[string]*PreparedQueryRule)
	p.preparedQueryPrefixRules = make(map[string]*PreparedQueryRule)
	p.serviceRules = make(map[string]*ServiceRule)
	p.servicePrefixRules = make(map[string]*ServiceRule)
	p.sessionRules = make(map[string]*SessionRule)
	p.sessionPrefixRules = make(map[string]*SessionRule)
}

func (p *policyRulesMergeContext) merge(policy *PolicyRules) {
	if takesPrecedenceOver(policy.ACL, p.aclRule) {
		p.aclRule = policy.ACL
	}

	for _, ap := range policy.Agents {
		update := true
		if permission, found := p.agentRules[ap.Node]; found {
			update = takesPrecedenceOver(ap.Policy, permission.Policy)
		}

		if update {
			p.agentRules[ap.Node] = ap
		}
	}

	for _, ap := range policy.AgentPrefixes {
		update := true
		if permission, found := p.agentPrefixRules[ap.Node]; found {
			update = takesPrecedenceOver(ap.Policy, permission.Policy)
		}

		if update {
			p.agentPrefixRules[ap.Node] = ap
		}
	}

	for _, ep := range policy.Events {
		update := true
		if permission, found := p.eventRules[ep.Event]; found {
			update = takesPrecedenceOver(ep.Policy, permission.Policy)
		}

		if update {
			p.eventRules[ep.Event] = ep
		}
	}

	for _, ep := range policy.EventPrefixes {
		update := true
		if permission, found := p.eventPrefixRules[ep.Event]; found {
			update = takesPrecedenceOver(ep.Policy, permission.Policy)
		}

		if update {
			p.eventPrefixRules[ep.Event] = ep
		}
	}

	if takesPrecedenceOver(policy.Keyring, p.keyringRule) {
		p.keyringRule = policy.Keyring
	}

	for _, kp := range policy.Keys {
		update := true
		if permission, found := p.keyRules[kp.Prefix]; found {
			update = takesPrecedenceOver(kp.Policy, permission.Policy)
		}

		if update {
			p.keyRules[kp.Prefix] = kp
		}
	}

	for _, kp := range policy.KeyPrefixes {
		update := true
		if permission, found := p.keyPrefixRules[kp.Prefix]; found {
			update = takesPrecedenceOver(kp.Policy, permission.Policy)
		}

		if update {
			p.keyPrefixRules[kp.Prefix] = kp
		}
	}

	for _, np := range policy.Nodes {
		update := true
		if permission, found := p.nodeRules[np.Name]; found {
			update = takesPrecedenceOver(np.Policy, permission.Policy)
		}

		if update {
			p.nodeRules[np.Name] = np
		}
	}

	for _, np := range policy.NodePrefixes {
		update := true
		if permission, found := p.nodePrefixRules[np.Name]; found {
			update = takesPrecedenceOver(np.Policy, permission.Policy)
		}

		if update {
			p.nodePrefixRules[np.Name] = np
		}
	}

	if takesPrecedenceOver(policy.Mesh, p.meshRule) {
		p.meshRule = policy.Mesh
	}

	if takesPrecedenceOver(policy.Peering, p.peeringRule) {
		p.peeringRule = policy.Peering
	}

	if takesPrecedenceOver(policy.Operator, p.operatorRule) {
		p.operatorRule = policy.Operator
	}

	for _, qp := range policy.PreparedQueries {
		update := true
		if permission, found := p.preparedQueryRules[qp.Prefix]; found {
			update = takesPrecedenceOver(qp.Policy, permission.Policy)
		}

		if update {
			p.preparedQueryRules[qp.Prefix] = qp
		}
	}

	for _, qp := range policy.PreparedQueryPrefixes {
		update := true
		if permission, found := p.preparedQueryPrefixRules[qp.Prefix]; found {
			update = takesPrecedenceOver(qp.Policy, permission.Policy)
		}

		if update {
			p.preparedQueryPrefixRules[qp.Prefix] = qp
		}
	}

	for _, sp := range policy.Services {
		existing, found := p.serviceRules[sp.Name]

		if !found {
			p.serviceRules[sp.Name] = sp
			continue
		}

		if takesPrecedenceOver(sp.Policy, existing.Policy) {
			existing.Policy = sp.Policy
			existing.EnterpriseRule = sp.EnterpriseRule
		}

		if takesPrecedenceOver(sp.Intentions, existing.Intentions) {
			existing.Intentions = sp.Intentions
		}
	}

	for _, sp := range policy.ServicePrefixes {
		existing, found := p.servicePrefixRules[sp.Name]

		if !found {
			p.servicePrefixRules[sp.Name] = sp
			continue
		}

		if takesPrecedenceOver(sp.Policy, existing.Policy) {
			existing.Policy = sp.Policy
			existing.EnterpriseRule = sp.EnterpriseRule
		}

		if takesPrecedenceOver(sp.Intentions, existing.Intentions) {
			existing.Intentions = sp.Intentions
		}
	}

	for _, sp := range policy.Sessions {
		update := true
		if permission, found := p.sessionRules[sp.Node]; found {
			update = takesPrecedenceOver(sp.Policy, permission.Policy)
		}

		if update {
			p.sessionRules[sp.Node] = sp
		}
	}

	for _, sp := range policy.SessionPrefixes {
		update := true
		if permission, found := p.sessionPrefixRules[sp.Node]; found {
			update = takesPrecedenceOver(sp.Policy, permission.Policy)
		}

		if update {
			p.sessionPrefixRules[sp.Node] = sp
		}
	}
}

func (p *policyRulesMergeContext) fill(merged *PolicyRules) {
	merged.ACL = p.aclRule
	merged.Keyring = p.keyringRule
	merged.Operator = p.operatorRule
	merged.Mesh = p.meshRule
	merged.Peering = p.peeringRule

	// All the for loop appends are ugly but Go doesn't have a way to get
	// a slice of all values within a map so this is necessary

	merged.Agents = []*AgentRule{}
	for _, policy := range p.agentRules {
		merged.Agents = append(merged.Agents, policy)
	}

	merged.AgentPrefixes = []*AgentRule{}
	for _, policy := range p.agentPrefixRules {
		merged.AgentPrefixes = append(merged.AgentPrefixes, policy)
	}

	merged.Events = []*EventRule{}
	for _, policy := range p.eventRules {
		merged.Events = append(merged.Events, policy)
	}

	merged.EventPrefixes = []*EventRule{}
	for _, policy := range p.eventPrefixRules {
		merged.EventPrefixes = append(merged.EventPrefixes, policy)
	}

	merged.Keys = []*KeyRule{}
	for _, policy := range p.keyRules {
		merged.Keys = append(merged.Keys, policy)
	}

	merged.KeyPrefixes = []*KeyRule{}
	for _, policy := range p.keyPrefixRules {
		merged.KeyPrefixes = append(merged.KeyPrefixes, policy)
	}

	merged.Nodes = []*NodeRule{}
	for _, policy := range p.nodeRules {
		merged.Nodes = append(merged.Nodes, policy)
	}

	merged.NodePrefixes = []*NodeRule{}
	for _, policy := range p.nodePrefixRules {
		merged.NodePrefixes = append(merged.NodePrefixes, policy)
	}

	merged.PreparedQueries = []*PreparedQueryRule{}
	for _, policy := range p.preparedQueryRules {
		merged.PreparedQueries = append(merged.PreparedQueries, policy)
	}

	merged.PreparedQueryPrefixes = []*PreparedQueryRule{}
	for _, policy := range p.preparedQueryPrefixRules {
		merged.PreparedQueryPrefixes = append(merged.PreparedQueryPrefixes, policy)
	}

	merged.Services = []*ServiceRule{}
	for _, policy := range p.serviceRules {
		merged.Services = append(merged.Services, policy)
	}

	merged.ServicePrefixes = []*ServiceRule{}
	for _, policy := range p.servicePrefixRules {
		merged.ServicePrefixes = append(merged.ServicePrefixes, policy)
	}

	merged.Sessions = []*SessionRule{}
	for _, policy := range p.sessionRules {
		merged.Sessions = append(merged.Sessions, policy)
	}

	merged.SessionPrefixes = []*SessionRule{}
	for _, policy := range p.sessionPrefixRules {
		merged.SessionPrefixes = append(merged.SessionPrefixes, policy)
	}
}

type PolicyMerger struct {
	policyRulesMergeContext
	enterprisePolicyRulesMergeContext
}

func (m *PolicyMerger) init() {
	m.policyRulesMergeContext.init()
	m.enterprisePolicyRulesMergeContext.init()
}

func (m *PolicyMerger) Merge(policy *Policy) {
	m.policyRulesMergeContext.merge(&policy.PolicyRules)
	m.enterprisePolicyRulesMergeContext.merge(&policy.EnterprisePolicyRules)
}

// Policy outputs the merged policy
func (m *PolicyMerger) Policy() *Policy {
	merged := &Policy{}
	m.policyRulesMergeContext.fill(&merged.PolicyRules)
	m.enterprisePolicyRulesMergeContext.fill(&merged.EnterprisePolicyRules)

	return merged
}

func MergePolicies(policies []*Policy) *Policy {
	var merger PolicyMerger
	merger.init()
	for _, p := range policies {
		merger.Merge(p)
	}

	return merger.Policy()
}
