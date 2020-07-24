package consul

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
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
	ErrRateLimited          = errors.New("Rate limit reached, try again later")
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

	if done, err := s.srv.ForwardRPC("ConnectCA.ConfigurationGet", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && rule.OperatorRead(nil) != acl.Allow {
		return acl.ErrPermissionDenied
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

	if done, err := s.srv.ForwardRPC("ConnectCA.ConfigurationSet", args, args, reply); done {
		return err
	}

	// This action requires operator write access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && rule.OperatorWrite(nil) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// Exit early if it's a no-op change
	state := s.srv.fsm.State()
	confIdx, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}

	// Don't allow state changes. Either it needs to be empty or the same to allow
	// read-modify-write loops that don't touch the State field.
	if len(args.Config.State) > 0 &&
		!reflect.DeepEqual(args.Config.State, config.State) {
		return ErrStateReadOnly
	}

	// Don't allow users to change the ClusterID.
	args.Config.ClusterID = config.ClusterID
	if args.Config.Provider == config.Provider && reflect.DeepEqual(args.Config.Config, config.Config) {
		return nil
	}

	// If the provider hasn't changed, we need to load the current Provider state
	// so it can decide if it needs to change resources or not based on the config
	// change.
	if args.Config.Provider == config.Provider {
		// Note this is a shallow copy since the State method doc requires the
		// provider return a map that will not be further modified and should not
		// modify the one we pass to Configure.
		args.Config.State = config.State
	}

	// Create a new instance of the provider described by the config
	// and get the current active root CA. This acts as a good validation
	// of the config and makes sure the provider is functioning correctly
	// before we commit any changes to Raft.
	newProvider, err := s.srv.createCAProvider(args.Config)
	if err != nil {
		return fmt.Errorf("could not initialize provider: %v", err)
	}
	pCfg := ca.ProviderConfig{
		ClusterID:  args.Config.ClusterID,
		Datacenter: s.srv.config.Datacenter,
		// This endpoint can be called in a secondary DC too so set this correctly.
		IsPrimary: s.srv.config.Datacenter == s.srv.config.PrimaryDatacenter,
		RawConfig: args.Config.Config,
		State:     args.Config.State,
	}
	if err := newProvider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}
	if err := newProvider.GenerateRoot(); err != nil {
		return fmt.Errorf("error generating CA root certificate: %v", err)
	}

	newRootPEM, err := newProvider.ActiveRoot()
	if err != nil {
		return err
	}

	newActiveRoot, err := parseCARoot(newRootPEM, args.Config.Provider, args.Config.ClusterID)
	if err != nil {
		return err
	}

	// See if the provider needs to persist any state along with the config
	pState, err := newProvider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}
	args.Config.State = pState

	// Compare the new provider's root CA ID to the current one. If they
	// match, just update the existing provider with the new config.
	// If they don't match, begin the root rotation process.
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}

	// If the root didn't change or if this is a secondary DC, just update the
	// config and return.
	if (s.srv.config.Datacenter != s.srv.config.PrimaryDatacenter) ||
		root != nil && root.ID == newActiveRoot.ID {
		args.Op = structs.CAOpSetConfig
		resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}

		// If the config has been committed, update the local provider instance
		s.srv.setCAProvider(newProvider, newActiveRoot)

		s.logger.Info("CA provider config updated")

		return nil
	}

	// At this point, we know the config change has trigged a root rotation,
	// either by swapping the provider type or changing the provider's config
	// to use a different root certificate.

	// First up, sanity check that the current provider actually supports
	// cross-signing.
	oldProvider, _ := s.srv.getCAProvider()
	if oldProvider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}
	canXSign, err := oldProvider.SupportsCrossSigning()
	if err != nil {
		return fmt.Errorf("CA provider error: %s", err)
	}
	if !canXSign && !args.Config.ForceWithoutCrossSigning {
		return errors.New("The current CA Provider does not support cross-signing. " +
			"You can try again with ForceWithoutCrossSigningSet but this may cause " +
			"disruption - see documentation for more.")
	}
	if !canXSign && args.Config.ForceWithoutCrossSigning {
		s.logger.Warn("current CA doesn't support cross signing but " +
			"CA reconfiguration forced anyway with ForceWithoutCrossSigning")
	}

	// If it's a config change that would trigger a rotation (different provider/root):
	// 1. Get the root from the new provider.
	// 2. Call CrossSignCA on the old provider to sign the new root with the old one to
	// get a cross-signed certificate.
	// 3. Take the active root for the new provider and append the intermediate from step 2
	// to its list of intermediates.
	newRoot, err := connect.ParseCert(newRootPEM)
	if err != nil {
		return err
	}

	if canXSign {
		// Have the old provider cross-sign the new root
		xcCert, err := oldProvider.CrossSignCA(newRoot)
		if err != nil {
			return err
		}

		// Add the cross signed cert to the new CA's intermediates (to be attached
		// to leaf certs).
		newActiveRoot.IntermediateCerts = []string{xcCert}
	}

	intermediate, err := newProvider.GenerateIntermediate()
	if err != nil {
		return err
	}
	if intermediate != newRootPEM {
		newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediate)
	}

	// Update the roots and CA config in the state store at the same time
	idx, roots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	var newRoots structs.CARoots
	for _, r := range roots {
		newRoot := *r
		if newRoot.Active {
			newRoot.Active = false
			newRoot.RotatedOutAt = time.Now()
		}
		newRoots = append(newRoots, &newRoot)
	}
	newRoots = append(newRoots, newActiveRoot)

	args.Op = structs.CAOpSetRootsAndConfig
	args.Index = idx
	args.Config.ModifyIndex = confIdx
	args.Roots = newRoots
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	if respOk, ok := resp.(bool); ok && !respOk {
		return fmt.Errorf("could not atomically update roots and config")
	}

	// If the config has been committed, update the local provider instance
	// and call teardown on the old provider
	s.srv.setCAProvider(newProvider, newActiveRoot)

	if err := oldProvider.Cleanup(); err != nil {
		s.logger.Warn("failed to clean up old provider", "provider", config.Provider)
	}

	s.logger.Info("CA rotated to new root under provider", "provider", args.Config.Provider)

	return nil
}

// Roots returns the currently trusted root certificates.
func (s *ConnectCA) Roots(
	args *structs.DCSpecificRequest,
	reply *structs.IndexedCARoots) error {
	// Forward if necessary
	if done, err := s.srv.ForwardRPC("ConnectCA.Roots", args, args, reply); done {
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

	if done, err := s.srv.ForwardRPC("ConnectCA.Sign", args, args, reply); done {
		return err
	}

	// Parse the CSR
	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	// Parse the SPIFFE ID
	spiffeID, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return err
	}

	// Verify that the ACL token provided has permission to act as this service
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	var authzContext acl.AuthorizerContext
	var entMeta structs.EnterpriseMeta

	serviceID, isService := spiffeID.(*connect.SpiffeIDService)
	agentID, isAgent := spiffeID.(*connect.SpiffeIDAgent)
	if !isService && !isAgent {
		return fmt.Errorf("SPIFFE ID in CSR must be a service or agent ID")
	}

	if isService {
		entMeta.Merge(serviceID.GetEnterpriseMeta())
		entMeta.FillAuthzContext(&authzContext)
		if rule != nil && rule.ServiceWrite(serviceID.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}

		// Verify that the DC in the service URI matches us. We might relax this
		// requirement later but being restrictive for now is safer.
		if serviceID.Datacenter != s.srv.config.Datacenter {
			return fmt.Errorf("SPIFFE ID in CSR from a different datacenter: %s, "+
				"we are %s", serviceID.Datacenter, s.srv.config.Datacenter)
		}
	} else if isAgent {
		structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
		if rule != nil && rule.NodeWrite(agentID.Agent, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	cert, err := s.srv.SignCertificate(csr, spiffeID)
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

	if done, err := s.srv.ForwardRPC("ConnectCA.SignIntermediate", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if s.srv.config.PrimaryDatacenter != s.srv.config.Datacenter {
		return ErrNotPrimaryDatacenter
	}

	// This action requires operator write access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && rule.OperatorWrite(nil) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	provider, _ := s.srv.getCAProvider()
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
