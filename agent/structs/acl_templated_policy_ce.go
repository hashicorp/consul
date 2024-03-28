// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import _ "embed"

//go:embed acltemplatedpolicy/policies/ce/service.hcl
var ACLTemplatedPolicyService string

//go:embed acltemplatedpolicy/policies/ce/node.hcl
var ACLTemplatedPolicyNode string

//go:embed acltemplatedpolicy/policies/ce/dns.hcl
var ACLTemplatedPolicyDNS string

//go:embed acltemplatedpolicy/policies/ce/nomad-server.hcl
var ACLTemplatedPolicyNomadServer string

//go:embed acltemplatedpolicy/policies/ce/workload-identity.hcl
var ACLTemplatedPolicyWorkloadIdentity string

//go:embed acltemplatedpolicy/policies/ce/api-gateway.hcl
var ACLTemplatedPolicyAPIGateway string

//go:embed acltemplatedpolicy/policies/ce/nomad-client.hcl
var ACLTemplatedPolicyNomadClient string

//go:embed acltemplatedpolicy/policies/ce/agent-read.hcl
var ACLTemplatedPolicyAgentRead string

//go:embed acltemplatedpolicy/policies/ce/agent_prefix-read.hcl
var ACLTemplatedPolicyAgentPrefixRead string

//go:embed acltemplatedpolicy/policies/ce/key-read.hcl
var ACLTemplatedPolicyKeyRead string

//go:embed acltemplatedpolicy/policies/ce/key_prefix-read.hcl
var ACLTemplatedPolicyKeyPrefixRead string

//go:embed acltemplatedpolicy/policies/ce/node-read.hcl
var ACLTemplatedPolicyNodeRead string

//go:embed acltemplatedpolicy/policies/ce/node_prefix-read.hcl
var ACLTemplatedPolicyNodePrefixRead string

//go:embed acltemplatedpolicy/policies/ce/session-read.hcl
var ACLTemplatedPolicySessionRead string

//go:embed acltemplatedpolicy/policies/ce/session_prefix-read.hcl
var ACLTemplatedPolicySessionPrefixRead string

//go:embed acltemplatedpolicy/policies/ce/service-read.hcl
var ACLTemplatedPolicyServiceRead string

//go:embed acltemplatedpolicy/policies/ce/service_prefix-read.hcl
var ACLTemplatedPolicyServicePrefixRead string

//go:embed acltemplatedpolicy/policies/ce/agent-write.hcl
var ACLTemplatedPolicyAgentWrite string

//go:embed acltemplatedpolicy/policies/ce/agent_prefix-write.hcl
var ACLTemplatedPolicyAgentPrefixWrite string

//go:embed acltemplatedpolicy/policies/ce/key-write.hcl
var ACLTemplatedPolicyKeyWrite string

//go:embed acltemplatedpolicy/policies/ce/key_prefix-write.hcl
var ACLTemplatedPolicyKeyPrefixWrite string

//go:embed acltemplatedpolicy/policies/ce/node-write.hcl
var ACLTemplatedPolicyNodeWrite string

//go:embed acltemplatedpolicy/policies/ce/node_prefix-write.hcl
var ACLTemplatedPolicyNodePrefixWrite string

//go:embed acltemplatedpolicy/policies/ce/session-write.hcl
var ACLTemplatedPolicySessionWrite string

//go:embed acltemplatedpolicy/policies/ce/session_prefix-write.hcl
var ACLTemplatedPolicySessionPrefixWrite string

//go:embed acltemplatedpolicy/policies/ce/service-write.hcl
var ACLTemplatedPolicyServiceWrite string

//go:embed acltemplatedpolicy/policies/ce/service_prefix-write.hcl
var ACLTemplatedPolicyServicePrefixWrite string

func (t *ACLToken) TemplatedPolicyList() []*ACLTemplatedPolicy {
	if len(t.TemplatedPolicies) == 0 {
		return nil
	}

	out := make([]*ACLTemplatedPolicy, 0, len(t.TemplatedPolicies))
	for _, n := range t.TemplatedPolicies {
		out = append(out, n.Clone())
	}
	return out
}

func (t *ACLRole) TemplatedPolicyList() []*ACLTemplatedPolicy {
	if len(t.TemplatedPolicies) == 0 {
		return nil
	}

	out := make([]*ACLTemplatedPolicy, 0, len(t.TemplatedPolicies))
	for _, n := range t.TemplatedPolicies {
		out = append(out, n.Clone())
	}
	return out
}
