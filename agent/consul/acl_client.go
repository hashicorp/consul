package consul

import (
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/serf/serf"
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
		canUpgrade := false
		for _, member := range c.LANMembers() {
			if valid, parts := metadata.IsConsulServer(member); valid && parts.Status == serf.StatusAlive {
				if parts.ACLs != structs.ACLModeEnabled {
					canUpgrade = false
					break
				} else {
					canUpgrade = true
				}
			}
		}

		if canUpgrade {
			c.logger.Printf("[DEBUG] acl: transition out of legacy ACL mode")
			atomic.StoreInt32(&c.useNewACLs, 1)
			lib.UpdateSerfTag(c.serf, "acls", string(structs.ACLModeEnabled))
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
