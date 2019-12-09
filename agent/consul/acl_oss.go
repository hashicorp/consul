// +build !consulent

package consul

import (
	"log"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// EnterpriseACLResolverDelegate stub
type EnterpriseACLResolverDelegate interface{}

func (s *Server) replicationEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.ReplicationEnterpriseMeta()
}

func newEnterpriseACLConfig(*log.Logger) *acl.EnterpriseACLConfig {
	return nil
}

func (r *ACLResolver) resolveEnterpriseDefaultsForIdentity(identity structs.ACLIdentity) (acl.Authorizer, error) {
	return nil, nil
}
