// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// Consul-enterprise only
func (s *Server) ResolveEntTokenToIdentityAndAuthorizer(token string) (structs.ACLIdentity, acl.Authorizer) {
	return nil, nil
}

// Consul-enterprise only
func (s *Server) validateEnterpriseToken(identity structs.ACLIdentity) error {
	return nil
}

// aclBootstrapAllowed returns whether the server's configuration would allow ACL bootstrapping
//
// This endpoint does not take into account whether bootstrapping has been performed previously
// nor the bootstrap reset file.
func (s *Server) aclBootstrapAllowed() error {
	return nil
}
