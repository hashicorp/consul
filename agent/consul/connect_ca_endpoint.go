package consul

import (
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server
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
				}
			}

			return nil
		},
	)
}
