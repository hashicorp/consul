//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

const (
	EnterpriseACLPolicyGlobalManagement = ""
	EnterpriseACLPolicyGlobalReadOnly   = ""

	// aclPolicyTemplateServiceIdentity is the template used for synthesizing
	// policies for service identities.
	aclPolicyTemplateServiceIdentity = `
service "%[1]s" {
	policy = "write"
}
service "%[1]s-sidecar-proxy" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}
node_prefix "" {
	policy = "read"
}`

	// A typical Consul node requires two permissions for itself.
	// node:write
	//    - register itself in the catalog
	//    - update its network coordinates
	//    - potentially used to delete services during anti-entropy
	// service:read
	//    - used during anti-entropy to discover all services that
	//      are registered to the node. That way the node can diff
	//      its local state against an accurate depiction of the
	//      remote state.
	aclPolicyTemplateNodeIdentity = `
node "%[1]s" {
	policy = "write"
}
service_prefix "" {
	policy = "read"
}`
)

type ACLAuthMethodEnterpriseFields struct{}

type ACLAuthMethodEnterpriseMeta struct{}

func (_ *ACLAuthMethodEnterpriseMeta) FillWithEnterpriseMeta(_ *acl.EnterpriseMeta) {
	// do nothing
}

func (_ *ACLAuthMethodEnterpriseMeta) ToEnterpriseMeta() *acl.EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func aclServiceIdentityRules(svc string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf(aclPolicyTemplateServiceIdentity, svc)
}

func aclNodeIdentityRules(node string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf(aclPolicyTemplateNodeIdentity, node)
}

func (p *ACLPolicy) EnterprisePolicyMeta() *acl.EnterprisePolicyMeta {
	return nil
}

func (t *ACLToken) NodeIdentityList() []*ACLNodeIdentity {
	if len(t.NodeIdentities) == 0 {
		return nil
	}

	out := make([]*ACLNodeIdentity, 0, len(t.NodeIdentities))
	for _, n := range t.NodeIdentities {
		out = append(out, n.Clone())
	}
	return out
}

func (r *ACLRole) NodeIdentityList() []*ACLNodeIdentity {
	if len(r.NodeIdentities) == 0 {
		return nil
	}

	out := make([]*ACLNodeIdentity, 0, len(r.NodeIdentities))
	for _, n := range r.NodeIdentities {
		out = append(out, n.Clone())
	}
	return out
}

func IsValidPartitionAndDatacenter(meta acl.EnterpriseMeta, datacenters []string, primaryDatacenter string) bool {
	return true
}
