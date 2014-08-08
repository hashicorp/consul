package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
)

// aclCacheEntry is used to cache non-authoritative ACL's
// If non-authoritative, then we must respect a TTL
type aclCacheEntry struct {
	ACL     acl.ACL
	TTL     time.Duration
	Expires time.Time
}

// aclFault is used to fault in the rules for an ACL if we take a miss
func (s *Server) aclFault(id string) (string, error) {
	state := s.fsm.State()
	_, acl, err := state.ACLGet(id)
	if err != nil {
		return "", err
	}
	if acl == nil {
		return "", fmt.Errorf("ACL not found: %s", id)
	}
	return acl.Rules, nil
}

// resolveToken is used to resolve an ACL is any is appropriate
func (s *Server) resolveToken(id string) (acl.ACL, error) {
	// Check if there is no ACL datacenter (ACL's disabled)
	authDC := s.config.ACLDatacenter
	if authDC == "" {
		return nil, nil
	}

	// Check if we are the ACL datacenter and the leader, use the
	// authoritative cache
	if s.config.Datacenter == authDC && s.IsLeader() {
		return s.aclAuthCache.GetACL(id)
	}

	// Use our non-authoritative cache
	return s.lookupACL(id)
}

// lookupACL is used when we are non-authoritative, and need
// to resolve an ACL
func (s *Server) lookupACL(id string) (acl.ACL, error) {
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
	// TODO: GetPolicy...

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
