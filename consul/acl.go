package consul

import (
	"errors"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
)

const (
	// aclNotFound indicates there is no matching ACL
	aclNotFound = "ACL not found"

	// rootDenied is returned when attempting to resolve a root ACL
	rootDenied = "Cannot resolve root ACL"

	// permissionDenied is returned when an ACL based rejection happens
	permissionDenied = "Permission denied"

	// aclDisabled is returned when ACL changes are not permitted
	// since they are disabled.
	aclDisabled = "ACL support disabled"

	// anonymousToken is the token ID we re-write to if there
	// is no token ID provided
	anonymousToken = "anonymous"
)

var (
	permissionDeniedErr = errors.New(permissionDenied)
)

// aclCacheEntry is used to cache non-authoritative ACL's
// If non-authoritative, then we must respect a TTL
type aclCacheEntry struct {
	ACL     acl.ACL
	Expires time.Time
	ETag    string
}

// aclFault is used to fault in the rules for an ACL if we take a miss
func (s *Server) aclFault(id string) (string, string, error) {
	defer metrics.MeasureSince([]string{"consul", "acl", "fault"}, time.Now())
	state := s.fsm.State()
	_, acl, err := state.ACLGet(id)
	if err != nil {
		return "", "", err
	}
	if acl == nil {
		return "", "", errors.New(aclNotFound)
	}

	// Management tokens have no policy and inherit from the
	// 'manage' root policy
	if acl.Type == structs.ACLTypeManagement {
		return "manage", "", nil
	}

	// Otherwise use the base policy
	return s.config.ACLDefaultPolicy, acl.Rules, nil
}

// resolveToken is used to resolve an ACL is any is appropriate
func (s *Server) resolveToken(id string) (acl.ACL, error) {
	// Check if there is no ACL datacenter (ACL's disabled)
	authDC := s.config.ACLDatacenter
	if len(authDC) == 0 {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"consul", "acl", "resolveToken"}, time.Now())

	// Handle the anonymous token
	if len(id) == 0 {
		id = anonymousToken
	} else if acl.RootACL(id) != nil {
		return nil, errors.New(rootDenied)
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
		metrics.IncrCounter([]string{"consul", "acl", "cache_hit"}, 1)
		return cached.ACL, nil
	} else {
		metrics.IncrCounter([]string{"consul", "acl", "cache_miss"}, 1)
	}

	// Attempt to refresh the policy
	args := structs.ACLPolicyRequest{
		Datacenter: authDC,
		ACL:        id,
	}
	if cached != nil {
		args.ETag = cached.ETag
	}
	var out structs.ACLPolicy
	err := s.RPC("ACL.GetPolicy", &args, &out)

	// Handle the happy path
	if err == nil {
		return s.useACLPolicy(id, authDC, cached, &out)
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
func (s *Server) useACLPolicy(id, authDC string, cached *aclCacheEntry, p *structs.ACLPolicy) (acl.ACL, error) {
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
		// Resolve the parent policy
		parent := acl.RootACL(p.Parent)
		if parent == nil {
			var err error
			parent, err = s.lookupACL(p.Parent, authDC)
			if err != nil {
				return nil, err
			}
		}

		// Compile the ACL
		acl, err := acl.New(parent, p.Policy)
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

// applyDiscoveryACLs is used to filter results from our service catalog based
// on the configured rules for the request ACL. Nodes or services which do
// not match the ACL rules will be dropped from the result.
func (s *Server) applyDiscoveryACLs(token string, subj interface{}) error {
	// Get the ACL from the token
	acl, err := s.resolveToken(token)
	if err != nil {
		return err
	}

	// Fast path if ACLs are not enabled
	if acl == nil {
		return nil
	}

	filt := func(service string) bool {
		// Don't filter the "consul" service or empty service names
		if service == "" || service == ConsulServiceID {
			return true
		}

		// Check the ACL
		if !acl.ServiceRead(service) {
			s.logger.Printf("[DEBUG] consul: reading service '%s' denied due to ACLs", service)
			return false
		}
		return true
	}

	switch v := subj.(type) {
	// Filter health checks
	case *structs.IndexedHealthChecks:
		for i := 0; i < len(v.HealthChecks); i++ {
			hc := v.HealthChecks[i]
			if filt(hc.ServiceName) {
				continue
			}
			v.HealthChecks = append(v.HealthChecks[:i], v.HealthChecks[i+1:]...)
			i--
		}

	// Filter services
	case *structs.IndexedServices:
		for svc, _ := range v.Services {
			if filt(svc) {
				continue
			}
			delete(v.Services, svc)
		}

	// Filter service nodes
	case *structs.IndexedServiceNodes:
		for i := 0; i < len(v.ServiceNodes); i++ {
			node := v.ServiceNodes[i]
			if filt(node.ServiceName) {
				continue
			}
			v.ServiceNodes = append(v.ServiceNodes[:i], v.ServiceNodes[i+1:]...)
			i--
		}

	// Filter node services
	case *structs.IndexedNodeServices:
		for svc, _ := range v.NodeServices.Services {
			if filt(svc) {
				continue
			}
			delete(v.NodeServices.Services, svc)
		}

	// Filter check service nodes
	case *structs.IndexedCheckServiceNodes:
		for i := 0; i < len(v.Nodes); i++ {
			cs := v.Nodes[i]
			if filt(cs.Service.Service) {
				continue
			}
			v.Nodes = append(v.Nodes[:i], v.Nodes[i+1:]...)
			i--
		}

	// Filter node dumps
	case *structs.IndexedNodeDump:
		for i := 0; i < len(v.Dump); i++ {
			dump := v.Dump[i]

			// Filter the services
			for i := 0; i < len(dump.Services); i++ {
				svc := dump.Services[i]
				if filt(svc.Service) {
					continue
				}
				dump.Services = append(dump.Services[:i], dump.Services[i+1:]...)
				i--
			}

			// Filter the checks
			for i := 0; i < len(dump.Checks); i++ {
				chk := dump.Checks[i]
				if filt(chk.ServiceName) {
					continue
				}
				dump.Checks = append(dump.Checks[:i], dump.Checks[i+1:]...)
				i--
			}
		}
	}

	return nil
}
