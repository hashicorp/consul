package consul

import (
	"errors"

	"github.com/hashicorp/consul/agent/structs"
)

var (
	ErrAutoEncryptNotEnabled = errors.New("Either AutoEncrypt.TLS or AutoEncrypt.Gossip must be enabled in order to use this endpoint")
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
	if !a.srv.config.AutoEncryptTLS && !a.srv.config.AutoEncryptGossip {
		return ErrAutoEncryptNotEnabled
	}
	if done, err := a.srv.forward("AutoEncrypt.Sign", args, args, reply); done {
		return err
	}

	if a.srv.config.AutoEncryptTLS {
		cert := &structs.IssuedCert{}
		c := &ConnectCA{srv: a.srv}
		err := c.Sign(args, cert)
		if err != nil {
			return err
		}

		reply.Agent = cert.Agent
		reply.AgentURI = cert.AgentURI
		reply.CertPEM = cert.CertPEM
		reply.RootCAs = a.srv.tlsConfigurator.CAPems()
		reply.VerifyServerHostname = a.srv.tlsConfigurator.VerifyServerHostname()
	}
	if a.srv.config.AutoEncryptGossip {
	}

	return nil
}
