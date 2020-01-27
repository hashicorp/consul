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

func newACLConfig(*log.Logger) *acl.Config {
	return &acl.Config{
		WildcardName: structs.WildcardSpecifier,
	}
}

func (r *ACLResolver) resolveEnterpriseDefaultsForIdentity(identity structs.ACLIdentity) (acl.Authorizer, error) {
	return nil, nil
}
