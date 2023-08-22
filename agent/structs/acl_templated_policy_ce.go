// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

const (
	ACLTemplatedPolicyService = `
service "{{.Name}}" {
	policy = "write"
}
service "{{.Name}}-sidecar-proxy" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}
node_prefix "" {
	policy = "read"
}`

	ACLTemplatedPolicyNode = `
node "{{.Name}}" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}`

	ACLTemplatedPolicyDNS = `
node_prefix "" {
	policy = "read"
}
service_prefix "" {
	policy = "read"
}
query_prefix "" {
	policy = "read"
}`
)

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
