package consul

import (
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/serf/serf"
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
	return c.useNewACLs.IsSet() == false
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
			c.useNewACLs.Set(true)
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
