package structs

import (
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/hashicorp/consul/acl"
)

type ACLCachesConfig struct {
	Identities     int
	Policies       int
	ParsedPolicies int
	Authorizers    int
	Roles          int
}

type ACLCaches struct {
	identities     *lru.TwoQueueCache // identity id -> structs.ACLIdentity
	parsedPolicies *lru.TwoQueueCache // policy content hash -> acl.Policy
	policies       *lru.TwoQueueCache // policy ID -> ACLPolicy
	authorizers    *lru.TwoQueueCache // token secret -> acl.Authorizer
	roles          *lru.TwoQueueCache // role ID -> ACLRole
}

type IdentityCacheEntry struct {
	Identity  ACLIdentity
	CacheTime time.Time
}

func (e *IdentityCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type ParsedPolicyCacheEntry struct {
	Policy    *acl.Policy
	CacheTime time.Time
}

func (e *ParsedPolicyCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type PolicyCacheEntry struct {
	Policy    *ACLPolicy
	CacheTime time.Time
}

func (e *PolicyCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type AuthorizerCacheEntry struct {
	Authorizer acl.Authorizer
	CacheTime  time.Time
	TTL        time.Duration
}

func (e *AuthorizerCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type RoleCacheEntry struct {
	Role      *ACLRole
	CacheTime time.Time
}

func (e *RoleCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

func NewACLCaches(config *ACLCachesConfig) (*ACLCaches, error) {
	cache := &ACLCaches{}

	if config != nil && config.Identities > 0 {
		identCache, err := lru.New2Q(config.Identities)
		if err != nil {
			return nil, err
		}

		cache.identities = identCache
	}

	if config != nil && config.Policies > 0 {
		policyCache, err := lru.New2Q(config.Policies)
		if err != nil {
			return nil, err
		}

		cache.policies = policyCache
	}

	if config != nil && config.ParsedPolicies > 0 {
		parsedCache, err := lru.New2Q(config.ParsedPolicies)
		if err != nil {
			return nil, err
		}

		cache.parsedPolicies = parsedCache
	}

	if config != nil && config.Authorizers > 0 {
		authCache, err := lru.New2Q(config.Authorizers)
		if err != nil {
			return nil, err
		}

		cache.authorizers = authCache
	}

	if config != nil && config.Roles > 0 {
		roleCache, err := lru.New2Q(config.Roles)
		if err != nil {
			return nil, err
		}

		cache.roles = roleCache
	}

	return cache, nil
}

// GetIdentity fetches an identity from the cache and returns it
func (c *ACLCaches) GetIdentity(id string) *IdentityCacheEntry {
	if c == nil || c.identities == nil {
		return nil
	}

	if raw, ok := c.identities.Get(id); ok {
		return raw.(*IdentityCacheEntry)
	}

	return nil
}

// GetIdentityWithSecretToken fetches the identity with the given secret token
// from the cache.
func (c *ACLCaches) GetIdentityWithSecretToken(secretToken string) *IdentityCacheEntry {
	return c.GetIdentity(cacheIDSecretToken(secretToken))
}

// GetPolicy fetches a policy from the cache and returns it
func (c *ACLCaches) GetPolicy(policyID string) *PolicyCacheEntry {
	if c == nil || c.policies == nil {
		return nil
	}

	if raw, ok := c.policies.Get(policyID); ok {
		return raw.(*PolicyCacheEntry)
	}

	return nil
}

// GetPolicy fetches a policy from the cache and returns it
func (c *ACLCaches) GetParsedPolicy(id string) *ParsedPolicyCacheEntry {
	if c == nil || c.parsedPolicies == nil {
		return nil
	}

	if raw, ok := c.parsedPolicies.Get(id); ok {
		return raw.(*ParsedPolicyCacheEntry)
	}

	return nil
}

// GetAuthorizer fetches a acl from the cache and returns it
func (c *ACLCaches) GetAuthorizer(id string) *AuthorizerCacheEntry {
	if c == nil || c.authorizers == nil {
		return nil
	}

	if raw, ok := c.authorizers.Get(id); ok {
		return raw.(*AuthorizerCacheEntry)
	}

	return nil
}

// GetRole fetches a role from the cache by id and returns it
func (c *ACLCaches) GetRole(roleID string) *RoleCacheEntry {
	if c == nil || c.roles == nil {
		return nil
	}

	if raw, ok := c.roles.Get(roleID); ok {
		return raw.(*RoleCacheEntry)
	}

	return nil
}

// PutIdentity adds a new identity to the cache
func (c *ACLCaches) PutIdentity(id string, ident ACLIdentity) {
	if c == nil || c.identities == nil {
		return
	}

	c.identities.Add(id, &IdentityCacheEntry{Identity: ident, CacheTime: time.Now()})
}

// PutIdentityWithSecretToken adds a new identity to the cache, keyed by the
// given secret token (with a prefix to prevent collisions).
func (c *ACLCaches) PutIdentityWithSecretToken(secretToken string, identity ACLIdentity) {
	c.PutIdentity(cacheIDSecretToken(secretToken), identity)
}

// RemoveIdentityWithSecretToken removes the identity from the cache with the
// given secret token.
func (c *ACLCaches) RemoveIdentityWithSecretToken(secretToken string) {
	if c == nil || c.identities == nil {
		return
	}

	c.identities.Remove(cacheIDSecretToken(secretToken))
}

func (c *ACLCaches) PutPolicy(policyId string, policy *ACLPolicy) {
	if c == nil || c.policies == nil {
		return
	}

	c.policies.Add(policyId, &PolicyCacheEntry{Policy: policy, CacheTime: time.Now()})
}

func (c *ACLCaches) PutParsedPolicy(id string, policy *acl.Policy) {
	if c == nil || c.parsedPolicies == nil {
		return
	}

	c.parsedPolicies.Add(id, &ParsedPolicyCacheEntry{Policy: policy, CacheTime: time.Now()})
}

func (c *ACLCaches) PutAuthorizer(id string, authorizer acl.Authorizer) {
	if c == nil || c.authorizers == nil {
		return
	}

	c.authorizers.Add(id, &AuthorizerCacheEntry{Authorizer: authorizer, CacheTime: time.Now()})
}

func (c *ACLCaches) PutRole(roleID string, role *ACLRole) {
	if c == nil || c.roles == nil {
		return
	}

	c.roles.Add(roleID, &RoleCacheEntry{Role: role, CacheTime: time.Now()})
}

func (c *ACLCaches) RemoveIdentity(id string) {
	if c != nil && c.identities != nil {
		c.identities.Remove(id)
	}
}

func (c *ACLCaches) RemovePolicy(policyID string) {
	if c != nil && c.policies != nil {
		c.policies.Remove(policyID)
	}
}

func (c *ACLCaches) RemoveRole(roleID string) {
	if c != nil && c.roles != nil {
		c.roles.Remove(roleID)
	}
}

func (c *ACLCaches) Purge() {
	if c != nil {
		if c.identities != nil {
			c.identities.Purge()
		}
		if c.policies != nil {
			c.policies.Purge()
		}
		if c.parsedPolicies != nil {
			c.parsedPolicies.Purge()
		}
		if c.authorizers != nil {
			c.authorizers.Purge()
		}
		if c.roles != nil {
			c.roles.Purge()
		}
	}
}

func cacheIDSecretToken(token string) string {
	return "token-secret:" + token
}
