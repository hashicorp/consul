package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"

	// register these as a builtin auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/awsauth"
	_ "github.com/hashicorp/consul/agent/consul/authmethod/kubeauth"
	_ "github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
)

type authMethodValidatorEntry struct {
	Validator   authmethod.Validator
	ModifyIndex uint64 // the raft index when this last changed
}

// loadAuthMethodValidator returns an authmethod.Validator for the given auth
// method configuration. If the cache is up to date as-of the provided index
// then the cached version is returned, otherwise a new validator is created
// and cached.
func (s *Server) loadAuthMethodValidator(idx uint64, method *structs.ACLAuthMethod) (authmethod.Validator, error) {
	if prevIdx, v, ok := s.aclAuthMethodValidators.GetValidator(method); ok && idx <= prevIdx {
		return v, nil
	}

	v, err := authmethod.NewValidator(s.logger, method)
	if err != nil {
		return nil, fmt.Errorf("auth method validator for %q could not be initialized: %v", method.Name, err)
	}

	v = s.aclAuthMethodValidators.PutValidatorIfNewer(method, v, idx)

	return v, nil
}
