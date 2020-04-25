package consul

import (
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

var serverACLCacheConfig *structs.ACLCachesConfig = &structs.ACLCachesConfig{
	// The server's ACL caching has a few underlying assumptions:
	//
	// 1 - All policies can be resolved locally. Hence we do not cache any
	//     unparsed policies/roles as we have memdb for that.
	// 2 - While there could be many identities being used within a DC the
	//     number of distinct policies and combined multi-policy authorizers
	//     will be much less.
	// 3 - If you need more than 10k tokens cached then you should probably
	//     enable token replication or be using DC local tokens. In both
	//     cases resolving the tokens from memdb will avoid the cache
	//     entirely
	//
	Identities:     10 * 1024,
	Policies:       0,
	ParsedPolicies: 512,
	Authorizers:    1024,
	Roles:          0,
}

func (s *Server) checkTokenUUID(id string) (bool, error) {
	state := s.fsm.State()

	// We won't check expiration times here. If we generate a UUID that matches
	// a token that hasn't been reaped yet, then we won't be able to insert the
	// new token due to a collision.

	if _, token, err := state.ACLTokenGetByAccessor(nil, id, nil); err != nil {
		return false, err
	} else if token != nil {
		return false, nil
	}

	if _, token, err := state.ACLTokenGetBySecret(nil, id, nil); err != nil {
		return false, err
	} else if token != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) checkPolicyUUID(id string) (bool, error) {
	state := s.fsm.State()
	if _, policy, err := state.ACLPolicyGetByID(nil, id, nil); err != nil {
		return false, err
	} else if policy != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) checkRoleUUID(id string) (bool, error) {
	state := s.fsm.State()
	if _, role, err := state.ACLRoleGetByID(nil, id, nil); err != nil {
		return false, err
	} else if role != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) checkBindingRuleUUID(id string) (bool, error) {
	state := s.fsm.State()
	if _, rule, err := state.ACLBindingRuleGetByID(nil, id, nil); err != nil {
		return false, err
	} else if rule != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) updateSerfTags(key, value string) {
	// Update the LAN serf
	lib.UpdateSerfTag(s.serfLAN, key, value)

	if s.serfWAN != nil {
		lib.UpdateSerfTag(s.serfWAN, key, value)
	}

	s.updateEnterpriseSerfTags(key, value)
}

func (s *Server) updateACLAdvertisement() {
	// One thing to note is that once in new ACL mode the server will
	// never transition to legacy ACL mode. This is not currently a
	// supported use case.
	s.updateSerfTags("acls", string(structs.ACLModeEnabled))
}

func (s *Server) canUpgradeToNewACLs(isLeader bool) bool {
	if atomic.LoadInt32(&s.useNewACLs) != 0 {
		// can't upgrade because we are already upgraded
		return false
	}

	if !s.InACLDatacenter() {
		foundServers, mode, _ := ServersGetACLMode(s, "", s.config.ACLDatacenter)
		if mode != structs.ACLModeEnabled || !foundServers {
			s.logger.Debug("Cannot upgrade to new ACLs, servers in acl datacenter are not yet upgraded", "ACLDatacenter", s.config.ACLDatacenter, "mode", mode, "found", foundServers)
			return false
		}
	}

	leaderAddr := string(s.raft.Leader())
	foundServers, mode, leaderMode := ServersGetACLMode(s, leaderAddr, s.config.Datacenter)
	if isLeader {
		if mode == structs.ACLModeLegacy {
			return true
		}
	} else {
		if leaderMode == structs.ACLModeEnabled {
			return true
		}
	}

	s.logger.Debug("Cannot upgrade to new ACLs", "leaderMode", leaderMode, "mode", mode, "found", foundServers, "leader", leaderAddr)
	return false
}

func (s *Server) InACLDatacenter() bool {
	return s.config.ACLDatacenter == "" || s.config.Datacenter == s.config.ACLDatacenter
}

func (s *Server) UseLegacyACLs() bool {
	return atomic.LoadInt32(&s.useNewACLs) == 0
}

func (s *Server) LocalTokensEnabled() bool {
	// in ACL datacenter so local tokens are always enabled
	if s.InACLDatacenter() {
		return true
	}

	if !s.config.ACLTokenReplication || s.tokens.ReplicationToken() == "" {
		return false
	}

	// token replication is off so local tokens are disabled
	return true
}

func (s *Server) ACLDatacenter(legacy bool) string {
	// For resolution running on servers the only option
	// is to contact the configured ACL Datacenter
	if s.config.ACLDatacenter != "" {
		return s.config.ACLDatacenter
	}

	// This function only gets called if ACLs are enabled.
	// When no ACL DC is set then it is assumed that this DC
	// is the primary DC
	return s.config.Datacenter
}

func (s *Server) ACLsEnabled() bool {
	return s.config.ACLsEnabled
}

// ResolveIdentityFromToken retrieves a token's full identity given its secretID.
func (s *Server) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// only allow remote RPC resolution when token replication is off and
	// when not in the ACL datacenter
	if !s.InACLDatacenter() && !s.config.ACLTokenReplication {
		return false, nil, nil
	}

	index, aclToken, err := s.fsm.State().ACLTokenGetBySecret(nil, token, nil)
	if err != nil {
		return true, nil, err
	} else if aclToken != nil && !aclToken.IsExpired(time.Now()) {
		return true, aclToken, nil
	}

	return s.InACLDatacenter() || index > 0, nil, acl.ErrNotFound
}

func (s *Server) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	index, policy, err := s.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
	if err != nil {
		return true, nil, err
	} else if policy != nil {
		return true, policy, nil
	}

	// If the max index of the policies table is non-zero then we have acls, until then
	// we may need to allow remote resolution. This is particularly useful to allow updating
	// the replication token via the API in a non-primary dc.
	return s.InACLDatacenter() || index > 0, policy, acl.ErrNotFound
}

func (s *Server) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	index, role, err := s.fsm.State().ACLRoleGetByID(nil, roleID, nil)
	if err != nil {
		return true, nil, err
	} else if role != nil {
		return true, role, nil
	}

	// If the max index of the roles table is non-zero then we have acls, until then
	// we may need to allow remote resolution. This is particularly useful to allow updating
	// the replication token via the API in a non-primary dc.
	return s.InACLDatacenter() || index > 0, role, acl.ErrNotFound
}

func (s *Server) ResolveToken(token string) (acl.Authorizer, error) {
	_, authz, err := s.ResolveTokenToIdentityAndAuthorizer(token)
	return authz, err
}

func (s *Server) ResolveTokenToIdentityAndAuthorizer(token string) (structs.ACLIdentity, acl.Authorizer, error) {
	if id, authz := s.ResolveEntTokenToIdentityAndAuthorizer(token); id != nil && authz != nil {
		return id, authz, nil
	}
	return s.acls.ResolveTokenToIdentityAndAuthorizer(token)
}

// ResolveTokenIdentityAndDefaultMeta retrieves an identity and authorizer for the caller,
// and populates the EnterpriseMeta based on the AuthorizerContext.
func (s *Server) ResolveTokenIdentityAndDefaultMeta(token string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (structs.ACLIdentity, acl.Authorizer, error) {
	identity, authz, err := s.ResolveTokenToIdentityAndAuthorizer(token)
	if err != nil {
		return nil, nil, err
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

	return identity, authz, err
}

// ResolveTokenAndDefaultMeta passes through to ResolveTokenIdentityAndDefaultMeta, eliding the identity from its response.
func (s *Server) ResolveTokenAndDefaultMeta(token string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
	_, authz, err := s.ResolveTokenIdentityAndDefaultMeta(token, entMeta, authzContext)
	return authz, err
}

func (s *Server) filterACL(token string, subj interface{}) error {
	if id, authz := s.ResolveEntTokenToIdentityAndAuthorizer(token); id != nil && authz != nil {
		return s.acls.filterACLWithAuthorizer(authz, subj)
	}
	return s.acls.filterACL(token, subj)
}

func (s *Server) filterACLWithAuthorizer(authorizer acl.Authorizer, subj interface{}) error {
	return s.acls.filterACLWithAuthorizer(authorizer, subj)
}
