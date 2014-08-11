package consul

import (
	"errors"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
)

const (
	// aclNotFound indicates there is no matching ACL
	aclNotFound = "ACL not found"

	// anonymousToken is the token ID we re-write to if there
	// is no token ID provided
	anonymousToken = "anonymous"
)

// aclCacheEntry is used to cache non-authoritative ACL's
// If non-authoritative, then we must respect a TTL
type aclCacheEntry struct {
	ACL     acl.ACL
	Expires time.Time
	ETag    string
}

// aclFault is used to fault in the rules for an ACL if we take a miss
func (s *Server) aclFault(id string) (string, error) {
	state := s.fsm.State()
	_, acl, err := state.ACLGet(id)
	if err != nil {
		return "", err
	}
	if acl == nil {
		return "", errors.New(aclNotFound)
	}
	return acl.Rules, nil
}

// resolveToken is used to resolve an ACL is any is appropriate
func (s *Server) resolveToken(id string) (acl.ACL, error) {
	// Check if there is no ACL datacenter (ACL's disabled)
	authDC := s.config.ACLDatacenter
	if len(authDC) == 0 {
		return nil, nil
	}

	// Handle the anonymous token
	if len(id) == 0 {
		id = anonymousToken
	}

	// Check if we are the ACL datacenter and the leader, use the
	// authoritative cache
	if s.config.Datacenter == authDC && s.IsLeader() {
		return s.aclAuthCache.GetACL(id)
	}

	// Use our non-authoritative cache
	return s.lookupACL(id, authDC)
}

// lookupACL is used when we are non-authoritative, and need
// to resolve an ACL
func (s *Server) lookupACL(id, authDC string) (acl.ACL, error) {
	// Check the cache for the ACL
	var cached *aclCacheEntry
	raw, ok := s.aclCache.Get(id)
	if ok {
		cached = raw.(*aclCacheEntry)
	}

	// Check for live cache
	if cached != nil && time.Now().Before(cached.Expires) {
		return cached.ACL, nil
	}

	// Attempt to refresh the policy
	args := structs.ACLPolicyRequest{
		Datacenter: authDC,
		ACL:        id,
	}
	var out structs.ACLPolicy
	err := s.RPC("ACL.GetPolicy", &args, &out)

	// Handle the happy path
	if err == nil {
		return s.useACLPolicy(id, cached, &out)
	}

	// Check for not-found
	if strings.Contains(err.Error(), aclNotFound) {
		return nil, errors.New(aclNotFound)
	} else {
		s.logger.Printf("[ERR] consul.acl: Failed to get policy for '%s': %v", id, err)
	}

	// Unable to refresh, apply the down policy
	switch s.config.ACLDownPolicy {
	case "allow":
		return acl.AllowAll(), nil
	case "extend-cache":
		if cached != nil {
			return cached.ACL, nil
		}
		fallthrough
	default:
		return acl.DenyAll(), nil
	}
}

// useACLPolicy handles an ACLPolicy response
func (s *Server) useACLPolicy(id string, cached *aclCacheEntry, p *structs.ACLPolicy) (acl.ACL, error) {
	// Check if we can used the cached policy
	if cached != nil && cached.ETag == p.ETag {
		if p.TTL > 0 {
			cached.Expires = time.Now().Add(p.TTL)
		}
		return cached.ACL, nil
	}

	// Check for a cached compiled policy
	var compiled acl.ACL
	raw, ok := s.aclPolicyCache.Get(p.ETag)
	if ok {
		compiled = raw.(acl.ACL)
	} else {
		// Determine the root policy
		var root acl.ACL
		switch p.Root {
		case "allow":
			root = acl.AllowAll()
		default:
			root = acl.DenyAll()
		}

		// Compile the ACL
		acl, err := acl.New(root, p.Policy)
		if err != nil {
			return nil, err
		}

		// Cache the policy
		s.aclPolicyCache.Add(p.ETag, acl)
		compiled = acl
	}

	// Cache the ACL
	cached = &aclCacheEntry{
		ACL:  compiled,
		ETag: p.ETag,
	}
	if p.TTL > 0 {
		cached.Expires = time.Now().Add(p.TTL)
	}
	s.aclCache.Add(id, cached)
	return compiled, nil
}
