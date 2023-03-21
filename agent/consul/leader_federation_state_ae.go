// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"time"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// federationStatePruneInterval is how often we check for stale federation
	// states to remove should a datacenter be removed from the WAN.
	federationStatePruneInterval = time.Hour
)

func (s *Server) startFederationStateAntiEntropy(ctx context.Context) {
	// Check to see if we can skip waiting for serf feature detection below.
	if !s.DatacenterSupportsFederationStates() {
		_, fedStates, err := s.fsm.State().FederationStateList(nil)
		if err != nil {
			s.logger.Warn("Failed to check for existing federation states and activate the feature flag quicker; skipping this optimization", "error", err)
		} else if len(fedStates) > 0 {
			s.setDatacenterSupportsFederationStates()
		}
	}

	if s.config.DisableFederationStateAntiEntropy {
		return
	}
	s.leaderRoutineManager.Start(ctx, federationStateAntiEntropyRoutineName, s.federationStateAntiEntropySync)

	// If this is the primary, then also prune any stale datacenters from the
	// list of federation states.
	if s.config.PrimaryDatacenter == s.config.Datacenter {
		s.leaderRoutineManager.Start(ctx, federationStatePruningRoutineName, s.federationStatePruning)
	}
}

func (s *Server) stopFederationStateAntiEntropy() {
	if s.config.DisableFederationStateAntiEntropy {
		return
	}
	s.leaderRoutineManager.Stop(federationStateAntiEntropyRoutineName)
	if s.config.PrimaryDatacenter == s.config.Datacenter {
		s.leaderRoutineManager.Stop(federationStatePruningRoutineName)
	}
}

func (s *Server) federationStateAntiEntropySync(ctx context.Context) error {
	var lastFetchIndex uint64

	retryLoopBackoff(ctx, func() error {
		if !s.DatacenterSupportsFederationStates() {
			return nil
		}

		idx, err := s.federationStateAntiEntropyMaybeSync(ctx, lastFetchIndex)
		if err != nil {
			return err
		}

		lastFetchIndex = idx
		return nil
	}, func(err error) {
		s.logger.Error("error performing anti-entropy sync of federation state", "error", err)
	})

	return nil
}

func (s *Server) federationStateAntiEntropyMaybeSync(ctx context.Context, lastFetchIndex uint64) (uint64, error) {
	queryOpts := &structs.QueryOptions{
		MinQueryIndex:     lastFetchIndex,
		RequireConsistent: true,
		// This is just for a local blocking query so no token is needed.
	}
	idx, prev, curr, err := s.fetchFederationStateAntiEntropyDetails(queryOpts)
	if err != nil {
		return 0, err
	}

	// We should check to see if our context was cancelled while we were blocked.
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	if prev != nil && prev.IsSame(curr) {
		s.logger.Trace("federation state anti-entropy sync skipped; already up to date")
		return idx, nil
	}

	if err := s.updateOurFederationState(curr); err != nil {
		return 0, fmt.Errorf("error performing federation state anti-entropy sync: %v", err)
	}

	s.logger.Info("federation state anti-entropy synced")

	return idx, nil
}

func (s *Server) updateOurFederationState(curr *structs.FederationState) error {
	if curr.Datacenter != s.config.Datacenter { // sanity check
		return fmt.Errorf("cannot use this mechanism to update federation states for other datacenters")
	}

	curr.UpdatedAt = time.Now().UTC()

	args := structs.FederationStateRequest{
		Op:    structs.FederationStateUpsert,
		State: curr,
	}

	if s.config.Datacenter == s.config.PrimaryDatacenter {
		// We are the primary, so we can't do an RPC as we don't have a replication token.
		_, err := s.leaderRaftApply("FederationState.Apply", structs.FederationStateRequestType, args)
		if err != nil {
			return err
		}
	} else {
		args.WriteRequest = structs.WriteRequest{
			Token: s.tokens.ReplicationToken(),
		}
		ignored := false
		if err := s.forwardDC("FederationState.Apply", s.config.PrimaryDatacenter, &args, &ignored); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) fetchFederationStateAntiEntropyDetails(
	queryOpts *structs.QueryOptions,
) (uint64, *structs.FederationState, *structs.FederationState, error) {
	var (
		prevFedState, currFedState *structs.FederationState
		queryMeta                  structs.QueryMeta
	)
	err := s.blockingQuery(
		queryOpts,
		&queryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			// Get the existing stored version of this FedState that has replicated down.
			// We could phone home to get this but that would incur extra WAN traffic
			// when we already have enough information locally to figure it out
			// (assuming that our replicator is still functioning).
			idx1, prev, err := state.FederationStateGet(ws, s.config.Datacenter)
			if err != nil {
				return err
			}

			// Fetch our current list of all mesh gateways.
			entMeta := structs.WildcardEnterpriseMetaInDefaultPartition()
			idx2, raw, err := state.ServiceDump(ws, structs.ServiceKindMeshGateway, true, entMeta, structs.DefaultPeerKeyword)
			if err != nil {
				return err
			}

			curr := &structs.FederationState{
				Datacenter:   s.config.Datacenter,
				MeshGateways: raw,
			}

			// Compute the maximum index seen.
			if idx2 > idx1 {
				queryMeta.Index = idx2
			} else {
				queryMeta.Index = idx1
			}

			prevFedState = prev
			currFedState = curr

			return nil
		})
	if err != nil {
		return 0, nil, nil, err
	}

	return queryMeta.Index, prevFedState, currFedState, nil
}

func (s *Server) federationStatePruning(ctx context.Context) error {
	ticker := time.NewTicker(federationStatePruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.pruneStaleFederationStates(); err != nil {
				s.logger.Error("error pruning stale federation states", "error", err)
			}
		}
	}
}

func (s *Server) pruneStaleFederationStates() error {
	state := s.fsm.State()
	_, fedStates, err := state.FederationStateList(nil)
	if err != nil {
		return err
	}

	for _, fedState := range fedStates {
		dc := fedState.Datacenter
		if s.router.HasDatacenter(dc) {
			continue
		}

		s.logger.Info("pruning stale federation state", "datacenter", dc)

		req := structs.FederationStateRequest{
			Op: structs.FederationStateDelete,
			State: &structs.FederationState{
				Datacenter: dc,
			},
		}
		_, err := s.leaderRaftApply("FederationState.Delete", structs.FederationStateRequestType, &req)

		if err != nil {
			return fmt.Errorf("Failed to delete federation state %s: %v", dc, err)
		}
	}

	return nil
}
