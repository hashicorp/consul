package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

var serverACLCacheConfig *structs.ACLCachesConfig = &structs.ACLCachesConfig{
	// TODO (ACL-V2) - Is 10240 enough? In a DC with 30k agents we can only
	//   cache 1/3 of the tokens if 1 is given to each agent
	Identities: 10 * 1024,
	// No unparsed policies are cached as they should all be resolvable from
	// the local state store
	Policies: 0,
	// TODO (ACL-V2) - 512 should be enough right. Will any users have more
	//   than 512 policies in-use within a given DC?
	ParsedPolicies: 512,
	// TODO (ACL-V2) 1024 should be enough right? Will any users have more
	//   than 1024 policy combinations in-use within a given DC. If so that
	//   would imply there are over 1024 unique sets of permissions being used
	//   as multiple identities using the same policies will use the same
	//   authorizer.
	Authorizers: 1024,
}

func (s *Server) checkTokenUUID(id string) (bool, error) {
	state := s.fsm.State()
	if _, token, err := state.ACLTokenGetByAccessor(nil, id); err != nil {
		return false, err
	} else if token != nil {
		return false, nil
	}

	if _, token, err := state.ACLTokenGetBySecret(nil, id); err != nil {
		return false, err
	} else if token != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) checkPolicyUUID(id string) (bool, error) {
	state := s.fsm.State()
	if _, policy, err := state.ACLPolicyGetByID(nil, id); err != nil {
		return false, err
	} else if policy != nil {
		return false, nil
	}

	return !structs.ACLIDReserved(id), nil
}

func (s *Server) updateACLAdvertisement() {
	// TODO (ACL-V2) - does this need to support transitioning to old ACLs?

	// always advertise to all the LAN Members
	lib.UpdateSerfTag(s.serfLAN, "acls", string(structs.ACLModeEnabled))

	if s.serfWAN != nil {
		// advertise on the WAN only when we are inside the ACL datacenter
		lib.UpdateSerfTag(s.serfWAN, "acls", string(structs.ACLModeEnabled))
	}
}

func (s *Server) canUpgradeToNewACLs(isLeader bool) bool {
	if !s.InACLDatacenter() {
		mode, _ := ServersGetACLMode(s.WANMembers(), "", s.config.ACLDatacenter)
		if mode != structs.ACLModeEnabled {
			return false
		}
	}

	if isLeader {
		if mode, _ := ServersGetACLMode(s.LANMembers(), "", ""); mode == structs.ACLModeLegacy {
			return true
		}
	} else {
		leader := string(s.raft.Leader())
		if _, leaderMode := ServersGetACLMode(s.LANMembers(), leader, ""); leaderMode == structs.ACLModeEnabled {
			return true
		}
	}

	return false
}

func (s *Server) InACLDatacenter() bool {
	return s.config.Datacenter == s.config.ACLDatacenter
}

func (s *Server) UseLegacyACLs() bool {
	return !s.useNewACLs.IsSet()
}

func (s *Server) LocalTokensEnabled() bool {
	// in ACL datacenter so local tokens are always enabled
	if s.InACLDatacenter() {
		return true
	}

	if !s.config.ACLTokenReplication || s.tokens.ACLReplicationToken() == "" {
		return false
	}

	// token replication is off so local tokens are disabled
	return true
}

func (s *Server) ACLDatacenter(legacy bool) string {
	// For resolution running on servers the only option
	// is to contact the configured ACL Datacenter
	return s.config.ACLDatacenter
}

func (s *Server) ACLsEnabled() bool {
	// TODO (ACL-V2) implement full checking
	if len(s.config.ACLDatacenter) > 0 {
		return true
	}

	return false
}

func (s *Server) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// only allow remote RPC resolution when token replication is off and
	// when not in the ACL datacenter
	if !s.InACLDatacenter() && !s.config.ACLTokenReplication {
		return false, nil, nil
	}

	index, aclToken, err := s.fsm.State().ACLTokenGetBySecret(nil, token)
	if err != nil {
		return true, nil, err
	} else if aclToken != nil {
		return true, aclToken, nil
	}

	return s.InACLDatacenter() || index > 0, nil, nil
}

func (s *Server) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	index, policy, err := s.fsm.State().ACLPolicyGetByID(nil, policyID)
	if err != nil {
		return true, nil, err
	} else if policy != nil {
		return true, policy, nil
	}

	// If the max index of the policies table is non-zero then we have acls, until then
	// we may need to allow remote resolution. This is particularly useful to allow updating
	// the replication token via the API in a non-primary dc.
	return s.InACLDatacenter() || index > 0, policy, err
}

func (s *Server) ResolveToken(token string) (acl.Authorizer, error) {
	return s.acls.ResolveToken(token)
}

func (s *Server) filterACL(token string, subj interface{}) error {
	return s.acls.filterACL(token, subj)
}

func (s *Server) filterACLWithAuthorizer(authorizer acl.Authorizer, subj interface{}) error {
	return s.acls.filterACLWithAuthorizer(authorizer, subj)
}
