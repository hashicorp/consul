package consul

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

var errFederationStatesNotSupported = errors.New("Not all servers in the datacenter support federation states - preventing replication")

func isErrFederationStatesNotSupported(err error) bool {
	return errors.Is(err, errFederationStatesNotSupported)
}

type FederationStateReplicator struct {
	srv            *Server
	gatewayLocator *GatewayLocator
}

var _ IndexReplicatorDelegate = (*FederationStateReplicator)(nil)

// SingularNoun implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) SingularNoun() string { return "federation state" }

// PluralNoun implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) PluralNoun() string { return "federation states" }

// MetricName implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) MetricName() string { return "federation-state" }

// FetchRemote implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) FetchRemote(lastRemoteIndex uint64) (int, interface{}, uint64, error) {
	if !r.srv.DatacenterSupportsFederationStates() {
		return 0, nil, 0, errFederationStatesNotSupported
	}
	lenRemote, remote, remoteIndex, err := r.fetchRemote(lastRemoteIndex)
	if r.gatewayLocator != nil {
		r.gatewayLocator.SetLastFederationStateReplicationError(err, true)
	}
	return lenRemote, remote, remoteIndex, err
}

func (r *FederationStateReplicator) fetchRemote(lastRemoteIndex uint64) (int, interface{}, uint64, error) {
	req := structs.DCSpecificRequest{
		Datacenter: r.srv.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         r.srv.tokens.ReplicationToken(),
		},
	}

	var response structs.IndexedFederationStates
	if err := r.srv.RPC("FederationState.List", &req, &response); err != nil {
		return 0, nil, 0, err
	}

	states := []*structs.FederationState(response.States)

	return len(response.States), states, response.QueryMeta.Index, nil
}

// FetchLocal implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) FetchLocal() (int, interface{}, error) {
	_, local, err := r.srv.fsm.State().FederationStateList(nil)
	if err != nil {
		return 0, nil, err
	}

	return len(local), local, nil
}

// DiffRemoteAndLocalState implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) DiffRemoteAndLocalState(localRaw interface{}, remoteRaw interface{}, lastRemoteIndex uint64) (*IndexReplicatorDiff, error) {
	local, ok := localRaw.([]*structs.FederationState)
	if !ok {
		return nil, fmt.Errorf("invalid type for local federation states: %T", localRaw)
	}
	remote, ok := remoteRaw.([]*structs.FederationState)
	if !ok {
		return nil, fmt.Errorf("invalid type for remote federation states: %T", remoteRaw)
	}
	federationStateSort(local)
	federationStateSort(remote)

	var deletions []*structs.FederationState
	var updates []*structs.FederationState
	var localIdx int
	var remoteIdx int
	for localIdx, remoteIdx = 0, 0; localIdx < len(local) && remoteIdx < len(remote); {
		if local[localIdx].Datacenter == remote[remoteIdx].Datacenter {
			// fedState is in both the local and remote state - need to check raft indices
			if remote[remoteIdx].ModifyIndex > lastRemoteIndex {
				updates = append(updates, remote[remoteIdx])
			}
			// increment both indices when equal
			localIdx += 1
			remoteIdx += 1
		} else if local[localIdx].Datacenter < remote[remoteIdx].Datacenter {
			// fedState no longer in remoted state - needs deleting
			deletions = append(deletions, local[localIdx])

			// increment just the local index
			localIdx += 1
		} else {
			// local state doesn't have this fedState - needs updating
			updates = append(updates, remote[remoteIdx])

			// increment just the remote index
			remoteIdx += 1
		}
	}

	for ; localIdx < len(local); localIdx += 1 {
		deletions = append(deletions, local[localIdx])
	}

	for ; remoteIdx < len(remote); remoteIdx += 1 {
		updates = append(updates, remote[remoteIdx])
	}

	return &IndexReplicatorDiff{
		NumDeletions: len(deletions),
		Deletions:    deletions,
		NumUpdates:   len(updates),
		Updates:      updates,
	}, nil
}

func federationStateSort(states []*structs.FederationState) {
	sort.Slice(states, func(i, j int) bool {
		return states[i].Datacenter < states[j].Datacenter
	})
}

// PerformDeletions implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) PerformDeletions(ctx context.Context, deletionsRaw interface{}) (exit bool, err error) {
	deletions, ok := deletionsRaw.([]*structs.FederationState)
	if !ok {
		return false, fmt.Errorf("invalid type for federation states deletions list: %T", deletionsRaw)
	}

	ticker := time.NewTicker(time.Second / time.Duration(r.srv.config.FederationStateReplicationApplyLimit))
	defer ticker.Stop()

	for i, state := range deletions {
		req := structs.FederationStateRequest{
			Op:         structs.FederationStateDelete,
			Datacenter: r.srv.config.Datacenter,
			State:      state,
		}

		_, err := r.srv.leaderRaftApply("FederationState.Delete", structs.FederationStateRequestType, &req)
		if err != nil {
			return false, err
		}

		if i < len(deletions)-1 {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}

// PerformUpdates implements IndexReplicatorDelegate.
func (r *FederationStateReplicator) PerformUpdates(ctx context.Context, updatesRaw interface{}) (exit bool, err error) {
	updates, ok := updatesRaw.([]*structs.FederationState)
	if !ok {
		return false, fmt.Errorf("invalid type for federation states update list: %T", updatesRaw)
	}

	ticker := time.NewTicker(time.Second / time.Duration(r.srv.config.FederationStateReplicationApplyLimit))
	defer ticker.Stop()

	for i, state := range updates {
		dup := *state // lightweight copy
		state2 := &dup

		// Keep track of the raft modify index at the primary
		state2.PrimaryModifyIndex = state.ModifyIndex

		req := structs.FederationStateRequest{
			Op:         structs.FederationStateUpsert,
			Datacenter: r.srv.config.Datacenter,
			State:      state2,
		}

		_, err := r.srv.leaderRaftApply("FederationState.Apply", structs.FederationStateRequestType, &req)
		if err != nil {
			return false, err
		}

		if i < len(updates)-1 {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}
