package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

var clientACLCacheConfig = &structs.ACLCachesConfig{
	// The ACL cache configuration on client agents is more conservative than
	// on the servers. It is assumed that individual client agents will have
	// fewer distinct identities accessing the client than a server would
	// and thus can put smaller limits on the amount of ACL caching done.
	//
	// Identities - number of identities/acl tokens that can be cached
	Identities: 1024,
	// Policies - number of unparsed ACL policies that can be cached
	Policies: 128,
	// ParsedPolicies - number of parsed ACL policies that can be cached
	ParsedPolicies: 128,
	// Authorizers - number of compiled multi-policy effective policies that can be cached
	Authorizers: 256,
	// Roles - number of ACL roles that can be cached
	Roles: 128,
}

type clientACLResolverBackend struct {
	// TODO: un-embed
	*Client
}

func (c *clientACLResolverBackend) ACLDatacenter() string {
	// For resolution running on clients servers within the current datacenter
	// must be queried first to pick up local tokens.
	return c.config.Datacenter
}

func (c *clientACLResolverBackend) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// clients do no local identity resolution at the moment
	return false, nil, nil
}

func (c *clientACLResolverBackend) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	// clients do no local policy resolution at the moment
	return false, nil, nil
}

func (c *clientACLResolverBackend) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	// clients do no local role resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolveTokenToIdentity(token string) (structs.ACLIdentity, error) {
	// not using ResolveTokenToIdentityAndAuthorizer because in this case we don't
	// need to resolve the roles, policies and namespace but just want the identity
	// information such as accessor id.
	return c.acls.ResolveTokenToIdentity(token)
}

// TODO: Server has an identical implementation, remove duplication
func (c *Client) ResolveTokenAndDefaultMeta(token string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
	identity, authz, err := c.acls.ResolveTokenToIdentityAndAuthorizer(token)
	if err != nil {
		return nil, err
	}

	if entMeta == nil {
		entMeta = &structs.EnterpriseMeta{}
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	if identity != nil {
		entMeta.Merge(identity.EnterpriseMetadata())
	} else {
		entMeta.Merge(structs.DefaultEnterpriseMetaInDefaultPartition())
	}

	// Use the meta to fill in the ACL authorization context
	entMeta.FillAuthzContext(authzContext)

	return authz, err
}
