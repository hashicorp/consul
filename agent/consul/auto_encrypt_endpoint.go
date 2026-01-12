// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	b, err1 := json.Marshal(args)

	fmt.Println(time.Now().String()+" ===================>  AutoEncrypt Sign function called 0 ", string(b), err1)
	
	if !a.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}
	if !a.srv.config.AutoEncryptAllowTLS {
		return ErrAutoEncryptAllowTLSNotEnabled
	}
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 1 ")

	// There's no reason to forward the AutoEncrypt.Sign RPC to a remote datacenter because its certificates
	// won't be valid in this datacenter. If the client is requesting a different datacenter, then this is a
	// misconfiguration, and we can give them a useful error.
	if args.Datacenter != a.srv.config.Datacenter {
		return fmt.Errorf("mismatched datacenter (client_dc='%s' server_dc='%s');"+
			" check client has same datacenter set as servers", args.Datacenter, a.srv.config.Datacenter)
	}
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 2 ")

	if done, err := a.srv.ForwardRPC("AutoEncrypt.Sign", args, reply); done {
		return err
	}
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 3 ")

	// This is the ConnectCA endpoint which is reused here because it is
	// exactly what is needed.
	c := ConnectCA{srv: a.srv}

	rootsArgs := structs.DCSpecificRequest{Datacenter: args.Datacenter}
	roots := structs.IndexedCARoots{}
	err := c.Roots(&rootsArgs, &roots)
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 4 ")

	if err != nil {
		return err
	}
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 5 ")

	cert := structs.IssuedCert{}
	err = c.Sign(args, &cert)
	if err != nil {
		return err
	}
	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 6 ")

	reply.IssuedCert = cert
	reply.ConnectCARoots = roots
	reply.ManualCARoots = a.srv.tlsConfigurator.ManualCAPems()
	reply.VerifyServerHostname = a.srv.tlsConfigurator.VerifyServerHostname()

	fmt.Println(time.Now().String() + " ===================>  AutoEncrypt Sign function called 7 ")

	return nil
}
