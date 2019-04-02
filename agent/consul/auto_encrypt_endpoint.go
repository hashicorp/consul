package consul

import (
	"errors"

	"github.com/hashicorp/consul/agent/structs"
)

var (
	ErrAutoEncryptTLSNotEnabled = errors.New("AutoEncrypt.TLS must be enabled in order to use this endpoint")
)

type AutoEncrypt struct {
	srv *Server
}

// Sign signs a certificate for an agent.
func (a *AutoEncrypt) Sign(
	args *structs.CASignRequest,
	reply *structs.IssuedCert) error {
	if !a.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}
	if !a.srv.config.AutoEncryptTLS {
		return ErrAutoEncryptTLSNotEnabled
	}

	if done, err := a.srv.forward("AutoEncrypt.Sign", args, args, reply); done {
		return err
	}

	// TODO (hans): add root CA to reply
	c := &ConnectCA{srv: a.srv}
	return c.Sign(args, reply)
}
