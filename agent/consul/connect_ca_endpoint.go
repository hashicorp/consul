package consul

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server
}

// ConfigurationGet returns the configuration for the CA.
func (s *ConnectCA) ConfigurationGet(
	args *structs.DCSpecificRequest,
	reply *structs.CAConfiguration) error {
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
	if done, err := s.srv.forward("ConnectCA.ConfigurationSet", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorWrite() {
		return acl.ErrPermissionDenied
	}

	// Commit
	// todo(kyhavlov): trigger a bootstrap here when the provider changes
	args.Op = structs.CAOpSetConfig
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

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
					ID:        r.ID,
					Name:      r.Name,
					RootCert:  r.RootCert,
					RaftIndex: r.RaftIndex,
					Active:    r.Active,
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

	// todo(kyhavlov): more validation on the CSR before signing

	cert, err := s.srv.signConnectCert(serviceId, csr)
	if err != nil {
		return err
	}

	// Set the response
	*reply = *cert

	return nil
}
