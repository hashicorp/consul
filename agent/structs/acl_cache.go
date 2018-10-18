package structs

import (
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/golang-lru"
)

type ACLCachesConfig struct {
	Identities     int
	Policies       int
	ParsedPolicies int
	Authorizers    int
}

type ACLCaches struct {
	identities     *lru.TwoQueueCache // identity id -> structs.ACLIdentity
	parsedPolicies *lru.TwoQueueCache // policy content hash -> acl.Policy
	policies       *lru.TwoQueueCache // policy ID -> ACLPolicy
	authorizers    *lru.TwoQueueCache // token secret -> acl.Authorizer
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

// PutIdentity adds a new identity to the cache
func (c *ACLCaches) PutIdentity(id string, ident ACLIdentity) {
	if c == nil || c.identities == nil {
		return
	}

	c.identities.Add(id, &IdentityCacheEntry{Identity: ident, CacheTime: time.Now()})
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

func (c *ACLCaches) PutAuthorizerWithTTL(id string, authorizer acl.Authorizer, ttl time.Duration) {
	if c == nil || c.authorizers == nil {
		return
	}

	c.authorizers.Add(id, &AuthorizerCacheEntry{Authorizer: authorizer, CacheTime: time.Now(), TTL: ttl})
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
	}
}
