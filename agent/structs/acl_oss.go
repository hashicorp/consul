// +build !consulent

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

const (
	EnterpriseACLPolicyGlobalManagement = ""

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
)

func aclServiceIdentityRules(svc string, _ *EnterpriseMeta) string {
	return fmt.Sprintf(aclPolicyTemplateServiceIdentity, svc)
}

func (p *ACLPolicy) EnterprisePolicyMeta() *acl.EnterprisePolicyMeta {
	return nil
}
