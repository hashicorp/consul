package acl

import (
	"crypto/md5"
	"fmt"

	"github.com/hashicorp/golang-lru"
)

// FaultFunc is a function used to fault in the rules for an
// ACL given it's ID
type FaultFunc func(id string) (string, error)

// aclEntry allows us to store the ACL with it's policy ID
type aclEntry struct {
	ACL      ACL
	PolicyID string
}

// Cache is used to implement policy and ACL caching
type Cache struct {
	aclCache    *lru.Cache
	faultfn     FaultFunc
	parent      ACL
	policyCache *lru.Cache
}

// NewCache contructs a new policy and ACL cache of a given size
func NewCache(size int, parent ACL, faultfn FaultFunc) (*Cache, error) {
	if size <= 0 {
		return nil, fmt.Errorf("Must provide positive cache size")
	}
	pc, _ := lru.New(size)
	ac, _ := lru.New(size)
	c := &Cache{
		aclCache:    ac,
		faultfn:     faultfn,
		parent:      parent,
		policyCache: pc,
	}
	return c, nil
}

// GetPolicy is used to get a potentially cached policy set.
// If not cached, it will be parsed, and then cached.
func (c *Cache) GetPolicy(rules string) (*Policy, error) {
	return c.getPolicy(c.ruleID(rules), rules)
}

// getPolicy is an internal method to get a cached policy,
// but it assumes a pre-computed ID
func (c *Cache) getPolicy(id, rules string) (*Policy, error) {
	raw, ok := c.policyCache.Get(id)
	if ok {
		return raw.(*Policy), nil
	}
	policy, err := Parse(rules)
	if err != nil {
		return nil, err
	}
	policy.ID = id
	c.policyCache.Add(id, policy)
	return policy, nil

}

// ruleID is used to generate an ID for a rule
func (c *Cache) ruleID(rules string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(rules)))
}

// GetACLPolicy is used to get the potentially cached ACL
// policy. If not cached, it will be generated and then cached.
func (c *Cache) GetACLPolicy(id string) (*Policy, error) {
	// Check for a cached acl
	if raw, ok := c.aclCache.Get(id); ok {
		cached := raw.(aclEntry)
		if raw, ok := c.policyCache.Get(cached.PolicyID); ok {
			return raw.(*Policy), nil
		}
	}

	// Fault in the rules
	rules, err := c.faultfn(id)
	if err != nil {
		return nil, err
	}

	// Get cached
	return c.GetPolicy(rules)
}

// GetACL is used to get a potentially cached ACL policy.
// If not cached, it will be generated and then cached.
func (c *Cache) GetACL(id string) (ACL, error) {
	// Look for the ACL directly
	raw, ok := c.aclCache.Get(id)
	if ok {
		return raw.(aclEntry).ACL, nil
	}

	// Get the rules
	rules, err := c.faultfn(id)
	if err != nil {
		return nil, err
	}
	ruleID := c.ruleID(rules)

	// Get the policy
	policy, err := c.getPolicy(ruleID, rules)
	if err != nil {
		return nil, err
	}

	// Get the ACL
	acl, err := New(c.parent, policy)
	if err != nil {
		return nil, err
	}

	// Cache and return the ACL
	c.aclCache.Add(id, aclEntry{acl, ruleID})
	return acl, nil
}

// ClearACL is used to clear the ACL cache if any
func (c *Cache) ClearACL(id string) {
	c.aclCache.Remove(id)
}
