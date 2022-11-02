//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/structs"
)

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

func (*Server) enterpriseAuthMethodTypeValidation(authMethodType string) error {
	return nil
}
