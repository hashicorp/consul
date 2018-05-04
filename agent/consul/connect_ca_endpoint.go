package consul

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

var ErrConnectNotEnabled = errors.New("Connect must be enabled in order to use this endpoint")

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server
}

// ConfigurationGet returns the configuration for the CA.
func (s *ConnectCA) ConfigurationGet(
	args *structs.DCSpecificRequest,
	reply *structs.CAConfiguration) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.forward("ConnectCA.ConfigurationGet", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorRead() {
		return acl.ErrPermissionDenied
	}

	state := s.srv.fsm.State()
	_, config, err := state.CAConfig()
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

	if done, err := s.srv.forward("ConnectCA.ConfigurationSet", args, args, reply); done {
		return err
	}

	// This action requires operator write access.
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorWrite() {
		return acl.ErrPermissionDenied
	}

	// Exit early if it's a no-op change
	state := s.srv.fsm.State()
	_, config, err := state.CAConfig()
	if err != nil {
		return err
	}
	if args.Config.Provider == config.Provider && reflect.DeepEqual(args.Config.Config, config.Config) {
		return nil
	}

	// Create a new instance of the provider described by the config
	// and get the current active root CA. This acts as a good validation
	// of the config and makes sure the provider is functioning correctly
	// before we commit any changes to Raft.
	newProvider, err := s.srv.createCAProvider(args.Config)
	if err != nil {
		return fmt.Errorf("could not initialize provider: %v", err)
	}

	newRootPEM, err := newProvider.ActiveRoot()
	if err != nil {
		return err
	}

	newActiveRoot, err := parseCARoot(newRootPEM, args.Config.Provider)
	if err != nil {
		return err
	}

	// Compare the new provider's root CA ID to the current one. If they
	// match, just update the existing provider with the new config.
	// If they don't match, begin the root rotation process.
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}

	if root != nil && root.ID == newActiveRoot.ID {
		args.Op = structs.CAOpSetConfig
		resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}

		// If the config has been committed, update the local provider instance
		s.srv.setCAProvider(newProvider)

		s.srv.logger.Printf("[INFO] connect: CA provider config updated")

		return nil
	}

	// At this point, we know the config change has trigged a root rotation,
	// either by swapping the provider type or changing the provider's config
	// to use a different root certificate.

	// If it's a config change that would trigger a rotation (different provider/root):
	// 1. Get the intermediate from the new provider
	// 2. Generate a CSR for the new intermediate, call SignCA on the old/current provider
	// to get the cross-signed intermediate
	// 3. Get the active root for the new provider, append the intermediate from step 3
	// to its list of intermediates
	intermediatePEM, err := newProvider.GenerateIntermediate()
	if err != nil {
		return err
	}

	intermediateCA, err := connect.ParseCert(intermediatePEM)
	if err != nil {
		return err
	}

	// Have the old provider cross-sign the new intermediate
	oldProvider := s.srv.getCAProvider()
	if oldProvider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}
	xcCert, err := oldProvider.CrossSignCA(intermediateCA)
	if err != nil {
		return err
	}

	// Add the cross signed cert to the new root's intermediates
	newActiveRoot.IntermediateCerts = []string{xcCert}

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
		}
		newRoots = append(newRoots, &newRoot)
	}
	newRoots = append(newRoots, newActiveRoot)

	args.Op = structs.CAOpSetRootsAndConfig
	args.Index = idx
	args.Roots = newRoots
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// If the config has been committed, update the local provider instance
	// and call teardown on the old provider
	s.srv.setCAProvider(newProvider)

	if err := oldProvider.Cleanup(); err != nil {
		s.srv.logger.Printf("[WARN] connect: failed to clean up old provider %q", config.Provider)
	}

	s.srv.logger.Printf("[INFO] connect: CA rotated to new root under provider %q", args.Config.Provider)

	return nil
}

// Roots returns the currently trusted root certificates.
func (s *ConnectCA) Roots(
	args *structs.DCSpecificRequest,
	reply *structs.IndexedCARoots) error {
	// Forward if necessary
	if done, err := s.srv.forward("ConnectCA.Roots", args, args, reply); done {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, roots, err := state.CARoots(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Roots = index, roots
			if reply.Roots == nil {
				reply.Roots = make(structs.CARoots, 0)
			}

			// The API response must NEVER contain the secret information
			// such as keys and so on. We use a whitelist below to copy the
			// specific fields we want to expose.
			for i, r := range reply.Roots {
				// IMPORTANT: r must NEVER be modified, since it is a pointer
				// directly to the structure in the memdb store.

				reply.Roots[i] = &structs.CARoot{
					ID:                r.ID,
					Name:              r.Name,
					SerialNumber:      r.SerialNumber,
					SigningKeyID:      r.SigningKeyID,
					NotBefore:         r.NotBefore,
					NotAfter:          r.NotAfter,
					RootCert:          r.RootCert,
					IntermediateCerts: r.IntermediateCerts,
					RaftIndex:         r.RaftIndex,
					Active:            r.Active,
				}

				if r.Active {
					reply.ActiveRootID = r.ID
				}
			}

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

	if done, err := s.srv.forward("ConnectCA.Sign", args, args, reply); done {
		return err
	}

	// Parse the CSR
	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	// Parse the SPIFFE ID
	spiffeId, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return err
	}
	serviceId, ok := spiffeId.(*connect.SpiffeIDService)
	if !ok {
		return fmt.Errorf("SPIFFE ID in CSR must be a service ID")
	}

	provider := s.srv.getCAProvider()
	if provider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}

	// todo(kyhavlov): more validation on the CSR before signing
	pem, err := provider.Sign(csr)
	if err != nil {
		return err
	}

	// TODO(banks): when we implement IssuedCerts table we can use the insert to
	// that as the raft index to return in response. Right now we can rely on only
	// the built-in provider being supported and the implementation detail that we
	// have to write a SerialIndex update to the provider config table for every
	// cert issued so in all cases this index will be higher than any previous
	// sign response. This has to happen after the provider.Sign call to observe
	// the index update.
	modIdx, _, err := s.srv.fsm.State().CAConfig()
	if err != nil {
		return err
	}

	cert, err := connect.ParseCert(pem)
	if err != nil {
		return err
	}

	// Set the response
	*reply = structs.IssuedCert{
		SerialNumber: connect.HexString(cert.SerialNumber.Bytes()),
		CertPEM:      pem,
		Service:      serviceId.Service,
		ServiceURI:   cert.URIs[0].String(),
		ValidAfter:   cert.NotBefore,
		ValidBefore:  cert.NotAfter,
		RaftIndex: structs.RaftIndex{
			ModifyIndex: modIdx,
			CreateIndex: modIdx,
		},
	}

	return nil
}
