package consul

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

const (
	// maxIntentionTxnSize is the maximum size (in bytes) of a transaction used during
	// Intention replication.
	maxIntentionTxnSize = raftWarnSize / 4
)

func (s *Server) startIntentionConfigEntryMigration(ctx context.Context) error {
	if !s.config.ConnectEnabled {
		return nil
	}

	// Check for the system metadata first, as that's the most trustworthy in
	// both the primary and secondaries.
	intentionFormat, err := s.GetSystemMetadata(structs.SystemMetadataIntentionFormatKey)
	if err != nil {
		return err
	}
	if intentionFormat == structs.SystemMetadataIntentionFormatConfigValue {
		// Bypass the serf component and jump right to the final state.
		s.setDatacenterSupportsIntentionsAsConfigEntries()
		return nil // nothing to migrate
	}

	if s.config.PrimaryDatacenter == s.config.Datacenter {
		// Do a quick legacy intentions check to see if it's even worth
		// spinning up the routine at all. This only applies if the primary
		// datacenter is composed entirely of compatible servers and there are
		// no more legacy intentions.
		if s.DatacenterSupportsIntentionsAsConfigEntries() {
			// NOTE: we only have to migrate legacy intentions from the default
			// partition because partitions didn't exist when legacy intentions
			// were canonical
			_, ixns, err := s.fsm.State().LegacyIntentions(nil, structs.WildcardEnterpriseMetaInDefaultPartition())
			if err != nil {
				return err
			}
			if len(ixns) == 0 {
				// Though there's nothing to migrate, still trigger the special
				// delete-all operation which should update various indexes and
				// drop some system metadata so we can skip all of this next
				// time.
				//
				// This is done inline with leader election so that new
				// clusters on 1.9.0 with no legacy intentions will immediately
				// transition to intentions-as-config-entries mode.
				return s.legacyIntentionsMigrationCleanupPhase(true)
			}
		}

		// When running in the primary we do all of the real work.
		s.leaderRoutineManager.Start(ctx, intentionMigrationRoutineName, s.legacyIntentionMigration)
	} else {
		// When running in the secondary we mostly just wait until the
		// primary finishes, and then wait until we're pretty sure the main
		// config entry replication thread has seen all of the
		// migration-related config entry edits before zeroing OUR copy of
		// the old intentions table.
		s.leaderRoutineManager.Start(ctx, intentionMigrationRoutineName, s.legacyIntentionMigrationInSecondaryDC)
	}

	return nil
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) legacyIntentionMigration(ctx context.Context) error {
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		return nil
	}

	connectLogger := s.loggers.Named(logging.Connect)

	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	retryLoopBackoff(loopCtx, func() error {
		// We have to wait until all of our sibling servers are upgraded.
		if !s.DatacenterSupportsIntentionsAsConfigEntries() {
			return nil
		}

		state := s.fsm.State()
		// NOTE: we only have to migrate legacy intentions from the default
		// partition because partitions didn't exist when legacy intentions
		// were canonical
		_, ixns, err := state.LegacyIntentions(nil, structs.WildcardEnterpriseMetaInDefaultPartition())
		if err != nil {
			return err
		}

		// NOTE: do not early abort here if the list is empty, let it run to completion.

		entries, err := convertLegacyIntentionsToConfigEntries(ixns)
		if err != nil {
			return err
		}

		entries, err = s.filterMigratedLegacyIntentions(entries)
		if err != nil {
			return err
		}

		// Totally cheat and repurpose one part of config entry replication
		// here so we automatically get our writes rate limited.
		_, err = s.reconcileLocalConfig(ctx, entries, structs.ConfigEntryUpsert)
		if err != nil {
			return err
		}

		// Wrap up
		if err := s.legacyIntentionsMigrationCleanupPhase(false); err != nil {
			return err
		}

		loopCancel()
		connectLogger.Info("intention migration complete")
		return nil

	}, func(err error) {
		connectLogger.Error(
			"error migrating intentions to config entries, will retry",
			"routine", intentionMigrationRoutineName,
			"error", err,
		)
	})

	return nil
}

func convertLegacyIntentionsToConfigEntries(ixns structs.Intentions) ([]structs.ConfigEntry, error) {
	entries := migrateIntentionsToConfigEntries(ixns)
	genericEntries := make([]structs.ConfigEntry, 0, len(entries))
	for _, entry := range entries {
		if err := entry.LegacyNormalize(); err != nil {
			return nil, err
		}
		if err := entry.LegacyValidate(); err != nil {
			return nil, err
		}
		genericEntries = append(genericEntries, entry)
	}
	return genericEntries, nil
}

// legacyIntentionsMigrationCleanupPhase will delete all legacy intentions and
// also record a piece of system metadata indicating that the migration has
// been completed.
func (s *Server) legacyIntentionsMigrationCleanupPhase(quiet bool) error {
	if !quiet {
		s.loggers.Named(logging.Connect).
			Info("finishing up intention migration by clearing the legacy store")
	}

	// This is a special intention op that ensures we bind the raft indexes
	// associated with both the legacy table and the config entry table.
	//
	// We also update a piece of system metadata to reflect that we are
	// definitely in a post-migration world.
	req := structs.IntentionRequest{
		Op: structs.IntentionOpDeleteAll,
	}

	if _, err := s.leaderRaftApply("Intentions.DeleteAll", structs.IntentionRequestType, req); err != nil {
		return err
	}

	// Bypass the serf component and jump right to the final state.
	s.setDatacenterSupportsIntentionsAsConfigEntries()

	return nil
}

func (s *Server) legacyIntentionMigrationInSecondaryDC(ctx context.Context) error {
	if s.config.PrimaryDatacenter == s.config.Datacenter {
		return nil
	}

	const (
		stateReplicateLegacy = iota
		stateWaitForPrimary
		stateWaitForConfigReplication
		stateDoCleanup
	)

	var (
		connectLogger = s.loggers.Named(logging.Connect)

		currentState                    = stateReplicateLegacy
		lastLegacyReplicationFetchIndex uint64
		legacyReplicationDisabled       bool
		lastLegacyOnlyFetchIndex        uint64
	)

	// This loop does several things:
	//
	// (1) Until we know for certain that the all of the servers in the primary
	// DC and all of the servers in our DC are running a Consul version that
	// can support intentions as config entries we have to continue to do
	// legacy intention replication.
	//
	// (2) Once we know all versions of Consul are compatible, we cease to
	// replicate legacy intentions as that table is frozen in the primary DC.
	// We do a special blocking query back to exclusively the legacy intentions
	// table in the primary to detect when it is zeroed out. We capture the max
	// raft index of this zeroing.
	//
	// (3) We wait until our own config entry replication crosses the primary
	// index from (2) so we know that we have replicated all of the new forms
	// of the existing intentions.

	// (1) Legacy intention replication. A blocking query back to the primary
	// asking for intentions to replicate is both needed if the primary is OLD
	// since we still need to replicate new writes, but also if the primary is
	// NEW to know when the migration code in the primary has completed and
	// zeroed the legacy memdb table.
	//
	// (2) If the primary has finished migration, we have to wait until our own
	// config entry replication catches up.
	//
	// (3) After config entry replication catches up we should zero out own own
	// legacy intentions memdb table.

	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	retryLoopBackoff(loopCtx, func() error {
		// This for loop only exists to avoid backoff every state transition.
		// Only trigger the loop if the state changes, otherwise return a nil
		// error.
		for {
			// Check for the system metadata first, as that's the most trustworthy.
			intentionFormat, err := s.GetSystemMetadata(structs.SystemMetadataIntentionFormatKey)
			if err != nil {
				return err
			}
			if intentionFormat == structs.SystemMetadataIntentionFormatConfigValue {
				// Bypass the serf component and jump right to the final state.
				s.setDatacenterSupportsIntentionsAsConfigEntries()
				loopCancel()
				return nil // nothing to migrate
			}

			switch currentState {
			case stateReplicateLegacy:
				if s.DatacenterSupportsIntentionsAsConfigEntries() {
					// Now all nodes in this datacenter and the primary are totally
					// ready for intentions as config entries, so disable legacy
					// replication and transition to the next phase.
					currentState = stateWaitForPrimary

					// Explicitly zero these out as they are now unused but could
					// be at worst misleading.
					lastLegacyReplicationFetchIndex = 0
					legacyReplicationDisabled = false

				} else if !legacyReplicationDisabled {
					// This is the embedded legacy intention replication.
					index, outOfLegacyMode, err := s.replicateLegacyIntentionsOnce(ctx, lastLegacyReplicationFetchIndex)
					if err != nil {
						return err
					} else if outOfLegacyMode {
						// We chill out and wait until all of the nodes in this
						// datacenter are ready for intentions as config entries.
						//
						// It's odd that we get this to happen before serf gives us
						// the feature flag, but gossip isn't immediate so it's
						// technically possible.
						legacyReplicationDisabled = true
					} else {
						lastLegacyReplicationFetchIndex = nextIndexVal(lastLegacyReplicationFetchIndex, index)
						return nil
					}
				}

			case stateWaitForPrimary:
				// Loop until we see the primary has finished migrating to config entries.
				index, numIxns, err := s.fetchLegacyIntentionsSummary(ctx, lastLegacyOnlyFetchIndex)
				if err != nil {
					return err
				}

				lastLegacyOnlyFetchIndex = nextIndexVal(lastLegacyOnlyFetchIndex, index)
				if numIxns == 0 {
					connectLogger.Debug("intention migration in secondary status", "last_primary_index", lastLegacyOnlyFetchIndex)
					currentState = stateWaitForConfigReplication
					// do not clear lastLegacyOnlyFetchIndex!
				} else {
					return nil
				}

			case stateWaitForConfigReplication:

				// manually list replicated config entries by kind

				// lastLegacyOnlyFetchIndex is now the raft commit index that
				// zeroed out the intentions memdb table.
				//
				// We compare that with the last raft commit index we have replicated
				// config entries for and use that to determine if we have caught up.
				lastReplicatedConfigIndex := s.configReplicator.Index()
				connectLogger.Debug(
					"intention migration in secondary status",
					"last_primary_intention_index", lastLegacyOnlyFetchIndex,
					"last_primary_replicated_config_index", lastReplicatedConfigIndex,
				)
				if lastReplicatedConfigIndex >= lastLegacyOnlyFetchIndex {
					currentState = stateDoCleanup
				} else {
					return nil
				}

			case stateDoCleanup:
				if err := s.legacyIntentionsMigrationCleanupPhase(false); err != nil {
					return err
				}

				loopCancel()
				return nil

			default:
				return fmt.Errorf("impossible state: %v", currentState)
			}
		}
	}, func(err error) {
		connectLogger.Error(
			"error performing intention migration in secondary datacenter, will retry",
			"routine", intentionMigrationRoutineName,
			"error", err,
		)
	})

	return nil
}

func (s *Server) fetchLegacyIntentionsSummary(_ context.Context, lastFetchIndex uint64) (uint64, int, error) {
	args := structs.IntentionListRequest{
		Datacenter: s.config.PrimaryDatacenter,
		Legacy:     true,
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: lastFetchIndex,
			Token:         s.tokens.ReplicationToken(),
		},
	}

	var remote structs.IndexedIntentions
	if err := s.forwardDC("Intention.List", s.config.PrimaryDatacenter, &args, &remote); err != nil {
		return 0, 0, err
	}

	return remote.Index, len(remote.Intentions), nil
}

// replicateLegacyIntentionsOnce executes a blocking query to the primary
// datacenter to replicate the intentions there to the local state one time.
func (s *Server) replicateLegacyIntentionsOnce(ctx context.Context, lastFetchIndex uint64) (uint64, bool, error) {
	args := structs.DCSpecificRequest{
		Datacenter:     s.config.PrimaryDatacenter,
		EnterpriseMeta: *s.replicationEnterpriseMeta(),
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: lastFetchIndex,
			Token:         s.tokens.ReplicationToken(),
		},
	}

	var remote structs.IndexedIntentions
	if err := s.forwardDC("Intention.List", s.config.PrimaryDatacenter, &args, &remote); err != nil {
		return 0, false, err
	}

	select {
	case <-ctx.Done():
		return 0, false, ctx.Err()
	default:
	}

	if remote.DataOrigin == structs.IntentionDataOriginConfigEntries {
		return 0, true, nil
	}

	_, local, err := s.fsm.State().LegacyIntentions(nil, s.replicationEnterpriseMeta())
	if err != nil {
		return 0, false, err
	}

	// Do a quick sanity check that somehow Permissions didn't slip through.
	// This shouldn't be necessary, but one extra check isn't going to hurt
	// anything.
	for _, ixn := range local {
		if len(ixn.Permissions) > 0 {
			// Assume that the data origin has switched to config entries.
			return 0, true, nil
		}
	}

	// Compute the diff between the remote and local intentions.
	deletes, updates := diffIntentions(local, remote.Intentions)
	txnOpSets := batchLegacyIntentionUpdates(deletes, updates)

	// Apply batched updates to the state store.
	for _, ops := range txnOpSets {
		txnReq := structs.TxnRequest{Ops: ops}

		// TODO(rpc-metrics-improv) -- verify labels
		resp, err := s.leaderRaftApply("Txn.Apply", structs.TxnRequestType, &txnReq)

		if err != nil {
			return 0, false, err
		}

		if txnResp, ok := resp.(structs.TxnResponse); ok {
			if len(txnResp.Errors) > 0 {
				return 0, false, txnResp.Error()
			}
		} else {
			return 0, false, fmt.Errorf("unexpected return type %T", resp)
		}
	}

	return remote.QueryMeta.Index, false, nil
}

// diffIntentions computes the difference between the local and remote intentions
// and returns lists of deletes and updates.
func diffIntentions(local, remote structs.Intentions) (structs.Intentions, structs.Intentions) {
	localIdx := make(map[string][]byte, len(local))
	remoteIdx := make(map[string]struct{}, len(remote))

	var deletes structs.Intentions
	var updates structs.Intentions

	for _, intention := range local {
		localIdx[intention.ID] = intention.Hash
	}
	for _, intention := range remote {
		remoteIdx[intention.ID] = struct{}{}
	}

	for _, intention := range local {
		if _, ok := remoteIdx[intention.ID]; !ok {
			deletes = append(deletes, intention)
		}
	}

	for _, intention := range remote {
		existingHash, ok := localIdx[intention.ID]
		if !ok {
			updates = append(updates, intention)
		} else if bytes.Compare(existingHash, intention.Hash) != 0 {
			updates = append(updates, intention)
		}
	}

	return deletes, updates
}

// batchLegacyIntentionUpdates breaks up the given updates into sets of TxnOps based
// on the estimated size of the operations.
//
//nolint:staticcheck
func batchLegacyIntentionUpdates(deletes, updates structs.Intentions) []structs.TxnOps {
	var txnOps structs.TxnOps
	for _, delete := range deletes {
		deleteOp := &structs.TxnIntentionOp{
			Op:        structs.IntentionOpDelete,
			Intention: delete,
		}
		txnOps = append(txnOps, &structs.TxnOp{Intention: deleteOp})
	}

	for _, update := range updates {
		updateOp := &structs.TxnIntentionOp{
			Op:        structs.IntentionOpUpdate,
			Intention: update,
		}
		txnOps = append(txnOps, &structs.TxnOp{Intention: updateOp})
	}

	// Divide the operations into chunks according to maxIntentionTxnSize.
	var batchedOps []structs.TxnOps
	for batchStart := 0; batchStart < len(txnOps); {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < len(txnOps) && batchSize < maxIntentionTxnSize; batchEnd += 1 {
			batchSize += txnOps[batchEnd].Intention.Intention.LegacyEstimateSize()
		}

		batchedOps = append(batchedOps, txnOps[batchStart:batchEnd])

		// txnOps[batchEnd] wasn't included as the slicing doesn't include the element at the stop index
		batchStart = batchEnd
	}

	return batchedOps
}
