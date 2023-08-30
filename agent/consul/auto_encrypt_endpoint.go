package consul

import (
	"errors"

	"github.com/hashicorp/consul/agent/structs"
)

var (
	ErrAutoEncryptAllowTLSNotEnabled = errors.New("AutoEncrypt.AllowTLS must be enabled in order to use this endpoint")
)

type AutoEncrypt struct {
	srv *Server
}

// Sign signs a certificate for an agent.
func (a *AutoEncrypt) Sign(
	args *structs.CASignRequest,
	reply *structs.SignedResponse) error {
	if !a.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}
	if !a.srv.config.AutoEncryptAllowTLS {
		return ErrAutoEncryptAllowTLSNotEnabled
	}
	if done, err := a.srv.ForwardRPC("AutoEncrypt.Sign", args, reply); done {
		return err
	}

	// This is the ConnectCA endpoint which is reused here because it is
	// exactly what is needed.
	c := ConnectCA{srv: a.srv}

	rootsArgs := structs.DCSpecificRequest{Datacenter: args.Datacenter}
	roots := structs.IndexedCARoots{}
	err := c.Roots(&rootsArgs, &roots)
	if err != nil {
		return err
	}

	cert := structs.IssuedCert{}
	err = c.Sign(args, &cert)
	if err != nil {
		return err
	}

	reply.IssuedCert = cert
	reply.ConnectCARoots = roots
	reply.ManualCARoots = a.srv.tlsConfigurator.ManualCAPems()
	reply.VerifyServerHostname = a.srv.tlsConfigurator.VerifyServerHostname()

	return nil
}
