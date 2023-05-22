package consul

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/auth"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
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

func (s *Server) InPrimaryDatacenter() bool {
	return s.config.InPrimaryDatacenter()
}

func (s *Server) LocalTokensEnabled() bool {
	// in ACL datacenter so local tokens are always enabled
	if s.InPrimaryDatacenter() {
		return true
	}

	if !s.config.ACLTokenReplication || s.tokens.ReplicationToken() == "" {
		// token replication is off so local tokens are disabled
		return false
	}

	return true
}

type serverACLResolverBackend struct {
	// TODO: un-embed
	*Server
}

func (s *serverACLResolverBackend) IsServerManagementToken(token string) bool {
	mgmt, err := s.GetSystemMetadata(structs.ServerManagementTokenAccessorID)
	if err != nil {
		s.logger.Debug("failed to fetch server management token: %w", err)
		return false
	}
	if mgmt == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(mgmt), []byte(token)) == 1
}

func (s *serverACLResolverBackend) ACLDatacenter() string {
	// For resolution running on servers the only option is to contact the
	// configured ACL Datacenter
	if s.config.PrimaryDatacenter != "" {
		return s.config.PrimaryDatacenter
	}

	// This function only gets called if ACLs are enabled.
	// When no ACL DC is set then it is assumed that this DC
	// is the primary DC
	return s.config.Datacenter
}

// ResolveIdentityFromToken retrieves a token's full identity given its secretID.
// TODO: why does some code call this directly instead of using ACLResolver.ResolveTokenToIdentity ?
func (s *Server) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	// only allow remote RPC resolution when token replication is off and
	// when not in the ACL datacenter
	if !s.InPrimaryDatacenter() && !s.config.ACLTokenReplication {
		return false, nil, nil
	}
	index, aclToken, err := s.fsm.State().ACLTokenGetBySecret(nil, token, nil)
	if err != nil {
		return true, nil, err
	} else if aclToken != nil && !aclToken.IsExpired(time.Now()) {
		return true, aclToken, nil
	}
	if aclToken == nil && token == acl.AnonymousTokenSecret {
		// synthesize the anonymous token for early use, bootstrapping has not completed
		s.insertAnonymousToken()
		fallbackId := structs.ACLToken{
			AccessorID:  acl.AnonymousTokenID,
			SecretID:    acl.AnonymousTokenSecret,
			Description: "synthesized anonymous token",
		}
		return true, &fallbackId, nil
	}

	defaultErr := acl.ErrNotFound
	canBootstrap, _, _ := s.fsm.State().CanBootstrapACLToken()
	if canBootstrap {
		defaultErr = fmt.Errorf("ACL system must be bootstrapped before making any requests that require authorization: %w", defaultErr)
	}
	return s.InPrimaryDatacenter() || index > 0, nil, defaultErr
}

func (s *serverACLResolverBackend) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	index, policy, err := s.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
	if err != nil {
		return true, nil, err
	} else if policy != nil {
		return true, policy, nil
	}

	// If the max index of the policies table is non-zero then we have acls, until then
	// we may need to allow remote resolution. This is particularly useful to allow updating
	// the replication token via the API in a non-primary dc.
	return s.InPrimaryDatacenter() || index > 0, policy, acl.ErrNotFound
}

func (s *serverACLResolverBackend) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	index, role, err := s.fsm.State().ACLRoleGetByID(nil, roleID, nil)
	if err != nil {
		return true, nil, err
	} else if role != nil {
		return true, role, nil
	}

	// If the max index of the roles table is non-zero then we have acls, until then
	// we may need to allow remote resolution. This is particularly useful to allow updating
	// the replication token via the API in a non-primary dc.
	return s.InPrimaryDatacenter() || index > 0, role, acl.ErrNotFound
}

func (s *Server) filterACL(token string, subj interface{}) error {
	return filterACL(s.ACLResolver, token, subj)
}

func (s *Server) filterACLWithAuthorizer(authorizer acl.Authorizer, subj interface{}) {
	filterACLWithAuthorizer(s.ACLResolver.logger, authorizer, subj)
}

func (s *Server) aclLogin() *auth.Login {
	return auth.NewLogin(s.aclBinder(), s.aclTokenWriter())
}

func (s *Server) aclBinder() *auth.Binder {
	return auth.NewBinder(s.fsm.State(), s.config.Datacenter)
}

func (s *Server) aclTokenWriter() *auth.TokenWriter {
	return auth.NewTokenWriter(auth.TokenWriterConfig{
		RaftApply:           s.raftApply,
		ACLCache:            s.ACLResolver.cache,
		Store:               s.fsm.State(),
		CheckUUID:           s.checkTokenUUID,
		MaxExpirationTTL:    s.config.ACLTokenMaxExpirationTTL,
		MinExpirationTTL:    s.config.ACLTokenMinExpirationTTL,
		PrimaryDatacenter:   s.config.PrimaryDatacenter,
		InPrimaryDatacenter: s.InPrimaryDatacenter(),
		LocalTokensEnabled:  s.LocalTokensEnabled(),
	})
}

func (s *Server) loadAuthMethod(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, authmethod.Validator, error) {
	idx, method, err := s.fsm.State().ACLAuthMethodGetByName(nil, methodName, entMeta)
	if err != nil {
		return nil, nil, err
	} else if method == nil {
		return nil, nil, fmt.Errorf("%w: auth method %q not found", acl.ErrNotFound, methodName)
	}

	if err := s.enterpriseAuthMethodTypeValidation(method.Type); err != nil {
		return nil, nil, err
	}

	validator, err := s.loadAuthMethodValidator(idx, method)
	if err != nil {
		return nil, nil, err
	}
	return method, validator, nil
}
