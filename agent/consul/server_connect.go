package consul

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) getCARoots(ws memdb.WatchSet, state *state.Store) (*structs.IndexedCARoots, error) {
	index, roots, config, err := state.CARootsAndConfig(ws)
	if err != nil {
		return nil, err
	}
	if config == nil || config.ClusterID == "" {
		return nil, fmt.Errorf("CA has not finished initializing")
	}

	indexedRoots := &structs.IndexedCARoots{}

	// Build TrustDomain based on the ClusterID stored.
	signingID := connect.SpiffeIDSigningForCluster(config.ClusterID)
	if signingID == nil {
		// If CA is bootstrapped at all then this should never happen but be
		// defensive.
		return nil, fmt.Errorf("no cluster trust domain setup")
	}

	indexedRoots.TrustDomain = signingID.Host()

	indexedRoots.Index, indexedRoots.Roots = index, roots
	if indexedRoots.Roots == nil {
		indexedRoots.Roots = make(structs.CARoots, 0)
	}

	// The response should not contain all fields as there are sensitive
	// data such as key material stored within the struct. So here we
	// pull out some of the fields and copy them into
	for i, r := range indexedRoots.Roots {
		var intermediates []string
		if r.IntermediateCerts != nil {
			intermediates = make([]string, len(r.IntermediateCerts))
			for i, intermediate := range r.IntermediateCerts {
				intermediates[i] = intermediate
			}
		}
		// IMPORTANT: r must NEVER be modified, since it is a pointer
		// directly to the structure in the memdb store.

		indexedRoots.Roots[i] = &structs.CARoot{
			ID:                  r.ID,
			Name:                r.Name,
			SerialNumber:        r.SerialNumber,
			SigningKeyID:        r.SigningKeyID,
			ExternalTrustDomain: r.ExternalTrustDomain,
			NotBefore:           r.NotBefore,
			NotAfter:            r.NotAfter,
			RootCert:            ca.EnsureTrailingNewline(r.RootCert),
			IntermediateCerts:   intermediates,
			RaftIndex:           r.RaftIndex,
			Active:              r.Active,
			PrivateKeyType:      r.PrivateKeyType,
			PrivateKeyBits:      r.PrivateKeyBits,
		}

		if r.Active {
			indexedRoots.ActiveRootID = r.ID
		}
	}

	return indexedRoots, nil
}
