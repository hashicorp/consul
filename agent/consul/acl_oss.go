//go:build !consulent
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

func serverPartitionInfo(s *Server) acl.ExportFetcher {
	return &partitionInfoNoop{}
}

func newACLConfig(_ acl.ExportFetcher, _ hclog.Logger) *acl.Config {
	return &acl.Config{
		WildcardName: structs.WildcardSpecifier,
	}
}

func (r *ACLResolver) resolveEnterpriseDefaultsForIdentity(identity structs.ACLIdentity) (acl.Authorizer, error) {
	return nil, nil
}

// resolveEnterpriseIdentityAndRoles will resolve an enterprise identity to an additional set of roles
func (_ *ACLResolver) resolveEnterpriseIdentityAndRoles(_ structs.ACLIdentity) (structs.ACLIdentity, structs.ACLRoles, error) {
	// this function does nothing in OSS
	return nil, nil, nil
}

// resolveEnterpriseIdentityAndPolicies will resolve an enterprise identity to an additional set of policies
func (_ *ACLResolver) resolveEnterpriseIdentityAndPolicies(_ structs.ACLIdentity) (structs.ACLIdentity, structs.ACLPolicies, error) {
	// this function does nothing in OSS
	return nil, nil, nil
}

// resolveLocallyManagedEnterpriseToken will resolve a managed service provider token to an identity and authorizer
func (_ *ACLResolver) resolveLocallyManagedEnterpriseToken(_ string) (structs.ACLIdentity, acl.Authorizer, bool) {
	return nil, nil, false
}

func setEnterpriseConf(entMeta *structs.EnterpriseMeta, conf *acl.Config) {}
