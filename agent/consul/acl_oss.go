// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

// EnterpriseACLResolverDelegate stub
type EnterpriseACLResolverDelegate interface{}

func (s *Server) replicationEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.ReplicationEnterpriseMeta()
}

func newACLConfig(hclog.Logger) *acl.Config {
	return &acl.Config{
		WildcardName: structs.WildcardSpecifier,
	}
}

func (r *ACLResolver) resolveEnterpriseDefaultsForIdentity(identity structs.ACLIdentity) (acl.Authorizer, error) {
	return nil, nil
}
