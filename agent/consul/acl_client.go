package consul

import (
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

var clientACLCacheConfig *structs.ACLCachesConfig = &structs.ACLCachesConfig{
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

func (c *Client) UseLegacyACLs() bool {
	return atomic.LoadInt32(&c.useNewACLs) == 0
}

func (c *Client) monitorACLMode() {
	waitTime := aclModeCheckMinInterval
	for {
		foundServers, mode, _ := ServersGetACLMode(c, "", c.config.Datacenter)
		if foundServers && mode == structs.ACLModeEnabled {
			c.logger.Debug("transitioned out of legacy ACL mode")
			c.updateSerfTags("acls", string(structs.ACLModeEnabled))
			atomic.StoreInt32(&c.useNewACLs, 1)
			return
		}

		select {
		case <-c.shutdownCh:
			return
		case <-time.After(waitTime):
			// do nothing
		}

		// calculate the amount of time to wait for the next round
		waitTime = waitTime * 2
		if waitTime > aclModeCheckMaxInterval {
			waitTime = aclModeCheckMaxInterval
		}
	}
}

func (c *Client) ACLDatacenter(legacy bool) string {
	// For resolution running on clients, when not in
	// legacy mode the servers within the current datacenter
	// must be queried first to pick up local tokens. When
	// in legacy mode the clients should directly query the
	// ACL Datacenter. When no ACL datacenter has been set
	// then we assume that the local DC is the ACL DC
	if legacy && c.config.ACLDatacenter != "" {
		return c.config.ACLDatacenter
	}

	return c.config.Datacenter
}

func (c *Client) ACLsEnabled() bool {
	return c.config.ACLsEnabled
}

func (c *Client) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// clients do no local identity resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	// clients do no local policy resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	// clients do no local role resolution at the moment
	return false, nil, nil
}

func (c *Client) ResolveToken(token string) (acl.Authorizer, error) {
	return c.acls.ResolveToken(token)
}

func (c *Client) ResolveTokenToIdentityAndAuthorizer(token string) (structs.ACLIdentity, acl.Authorizer, error) {
	return c.acls.ResolveTokenToIdentityAndAuthorizer(token)
}

func (c *Client) ResolveTokenAndDefaultMeta(token string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
	identity, authz, err := c.acls.ResolveTokenToIdentityAndAuthorizer(token)
	if err != nil {
		return nil, err
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	if identity != nil {
		entMeta.Merge(identity.EnterpriseMetadata())
	} else {
		entMeta.Merge(structs.DefaultEnterpriseMeta())
	}

	// Use the meta to fill in the ACL authorization context
	entMeta.FillAuthzContext(authzContext)

	return authz, err
}

func (c *Client) updateSerfTags(key, value string) {
	// Update the LAN serf
	lib.UpdateSerfTag(c.serf, key, value)
}
