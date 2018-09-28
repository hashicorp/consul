package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

var clientACLCacheConfig *structs.ACLCachesConfig = &structs.ACLCachesConfig{
	// TODO (ACL-V2) - Is 1024 enough? If more are needed that means there
	//   are more than 1024 identities/tokens being used on a single client agent
	Identities: 1024,
	// TODO (ACL-V2) - 128 should be enough right? Will more than 128 unique
	//   policies be in use on a single client agent?
	Policies: 128,
	// TODO (ACL-V2) - 128 should be enough right. Will any users have more
	//   than 128 policies in-use for a single client agent?
	ParsedPolicies: 128,
	// TODO (ACL-V2) 256 should be enough right? Will any users have more
	//   than 256 policy combinations in-use on a single client agent.
	Authorizers: 256,
}

func (c *Client) UseLegacyACLs() bool {
	// TODO (ACL-V2) - implement the real check here
	return false
}

func (c *Client) ACLDatacenter(legacy bool) string {
	// For resolution running on clients, when not in
	// legacy mode the servers within the current datacenter
	// must be queried first to pick up local tokens. When
	// in legacy mode the clients can directly query the ACL Datacenter
	if legacy {
		return c.config.ACLDatacenter
	}

	return c.config.Datacenter
}

func (c *Client) ACLsEnabled() bool {
	// TODO (ACL-V2) implement full check
	if len(c.config.ACLDatacenter) > 0 {
		return true
	}

	return false
}

func (c *Client) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// clients do no local identity resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	// clients do no local policy resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolveToken(token string) (acl.Authorizer, error) {
	return c.acls.ResolveToken(token)
}
