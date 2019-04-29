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
	reply *structs.SignResponse) error {
	if !a.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}
	if !a.srv.config.AutoEncryptAllowTLS {
		return ErrAutoEncryptAllowTLSNotEnabled
	}
	if done, err := a.srv.forward("AutoEncrypt.Sign", args, args, reply); done {
		return err
	}

	// This is the ConnectCA endpoint which is reused here because it is
	// exactly what is needed.
	c := &ConnectCA{srv: a.srv}
	cert := &structs.IssuedCert{}
	err := c.Sign(args, cert)
	if err != nil {
		return err
	}

	reply.Agent = cert.Agent
	reply.AgentURI = cert.AgentURI
	reply.CertPEM = cert.CertPEM
	reply.RootCAs = a.srv.tlsConfigurator.CAPems()
	reply.VerifyServerHostname = a.srv.tlsConfigurator.VerifyServerHostname()

	return nil
}
