// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var (
	// Err strings. net/rpc doesn't have a way to transport typed/rich errors so
	// we currently rely on sniffing the error string in a few cases where we need
	// to change client behavior. These are the canonical error strings to use.
	// Note though that client code can't use `err == consul.Err*` directly since
	// the error returned by RPC will be a plain error.errorString created by
	// net/rpc client so will not be the same _instance_ that this package
	// variable points to. Clients need to compare using `err.Error() ==
	// consul.ErrRateLimited.Error()` which is very sad. Short of replacing our
	// RPC mechanism it's hard to know how to make that much better though.
	ErrConnectNotEnabled    = errors.New("Connect must be enabled in order to use this endpoint")
	ErrRateLimited          = errors.New("Rate limit reached, try again later") // Note: we depend on this error message in the gRPC ConnectCA.Sign endpoint (see: isRateLimitError).
	ErrNotPrimaryDatacenter = errors.New("not the primary datacenter")
	ErrStateReadOnly        = errors.New("CA Provider State is read-only")
)

const (
	// csrLimitWait is the maximum time we'll wait for a slot when CSR concurrency
	// limiting or rate limiting is occurring. It's intentionally short so small
	// batches of requests can be accommodated when server has capacity (assuming
	// signing one cert takes much less than this) but failing requests fast when
	// a thundering herd comes along.
	csrLimitWait = 500 * time.Millisecond
)

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server

	logger hclog.Logger
}

// ConfigurationGet returns the configuration for the CA.
func (s *ConnectCA) ConfigurationGet(
	args *structs.DCSpecificRequest,
	reply *structs.CAConfiguration) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.ForwardRPC("ConnectCA.ConfigurationGet", args, reply); done {
		return err
	}

	// This action requires operator read access.
	authz, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	state := s.srv.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}
	*reply = *config

	return nil
}

// ConfigurationSet updates the configuration for the CA.
func (s *ConnectCA) ConfigurationSet(
	args *structs.CARequest,
	reply *interface{}) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.ForwardRPC("ConnectCA.ConfigurationSet", args, reply); done {
		return err
	}

	// This action requires operator write access.
	authz, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	return s.srv.caManager.UpdateConfiguration(args)
}

// Roots returns the currently trusted root certificates.
func (s *ConnectCA) Roots(
	args *structs.DCSpecificRequest,
	reply *structs.IndexedCARoots) error {
	// Forward if necessary
	if done, err := s.srv.ForwardRPC("ConnectCA.Roots", args, reply); done {
		return err
	}

	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			roots, err := s.srv.getCARoots(ws, state)
			if err != nil {
				return err
			}

			*reply = *roots
			return nil
		},
	)
}

// Sign signs a certificate for a service.
func (s *ConnectCA) Sign(
	args *structs.CASignRequest,
	reply *structs.IssuedCert) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.ForwardRPC("ConnectCA.Sign", args, reply); done {
		return err
	}

	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	authz, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	cert, err := s.srv.caManager.AuthorizeAndSignCertificate(csr, authz)
	if err != nil {
		return err
	}
	*reply = *cert
	return nil
}

// SignIntermediate signs an intermediate certificate for a remote datacenter.
func (s *ConnectCA) SignIntermediate(
	args *structs.CASignRequest,
	reply *string) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.ForwardRPC("ConnectCA.SignIntermediate", args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if s.srv.config.PrimaryDatacenter != s.srv.config.Datacenter {
		return ErrNotPrimaryDatacenter
	}

	// This action requires operator write access.
	authz, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	provider, _ := s.srv.caManager.getCAProvider()
	if provider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}

	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	cert, err := provider.SignIntermediate(csr)
	if err != nil {
		return err
	}

	*reply = cert

	return nil
}
