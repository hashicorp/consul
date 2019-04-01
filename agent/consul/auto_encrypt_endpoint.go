package consul

import (
	"github.com/hashicorp/consul/agent/structs"
)

type AutoEncrypt struct {
	srv *Server
}

// Sign signs a certificate for an agent.
func (a *AutoEncrypt) Sign(
	args *structs.CASignRequest,
	reply *structs.IssuedCert) error {
	c := &ConnectCA{srv: a.srv}
	return c.Sign(args, reply)
}
