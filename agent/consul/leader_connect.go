package consul

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

const (
	// loopRateLimit is the maximum rate per second at which we can rerun CA and intention
	// replication watches.
	loopRateLimit rate.Limit = 0.2

	// retryBucketSize is the maximum number of stored rate limit attempts for looped
	// blocking query operations.
	retryBucketSize = 5

	// maxIntentionTxnSize is the maximum size (in bytes) of a transaction used during
	// Intention replication.
	maxIntentionTxnSize = raftWarnSize / 4
)

var (
	// maxRetryBackoff is the maximum number of seconds to wait between failed blocking
	// queries when backing off.
	maxRetryBackoff = 256
)

// startConnectLeader starts multi-dc connect leader routines.
func (s *Server) startConnectLeader() {
	if !s.config.ConnectEnabled {
		return
	}

	// Start the Connect secondary DC actions if enabled.
	if s.config.Datacenter != s.config.PrimaryDatacenter {
		s.leaderRoutineManager.Start(intentionReplicationRoutineName, s.replicateIntentions)
	}

	s.caManager.Start()
	s.leaderRoutineManager.Start(caRootPruningRoutineName, s.runCARootPruning)
}

// stopConnectLeader stops connect specific leader functions.
func (s *Server) stopConnectLeader() {
	s.caManager.Stop()
	s.leaderRoutineManager.Stop(intentionReplicationRoutineName)
	s.leaderRoutineManager.Stop(caRootPruningRoutineName)

	// If the provider implements NeedsStop, we call Stop to perform any shutdown actions.
	provider, _ := s.caManager.getCAProvider()
	if provider != nil {
		if needsStop, ok := provider.(ca.NeedsStop); ok {
			needsStop.Stop()
		}
	}
}

// createProvider returns a connect CA provider from the given config.
func (s *Server) createCAProvider(conf *structs.CAConfiguration) (ca.Provider, error) {
	var p ca.Provider
	switch conf.Provider {
	case structs.ConsulCAProvider:
		p = &ca.ConsulProvider{Delegate: &consulCADelegate{s}}
	case structs.VaultCAProvider:
		p = ca.NewVaultProvider()
	case structs.AWSCAProvider:
		p = &ca.AWSProvider{}
	default:
		return nil, fmt.Errorf("unknown CA provider %q", conf.Provider)
	}

	// If the provider implements NeedsLogger, we give it our logger.
	if needsLogger, ok := p.(ca.NeedsLogger); ok {
		needsLogger.SetLogger(s.logger)
	}

	return p, nil
}

func (s *Server) runCARootPruning(ctx context.Context) error {
	ticker := time.NewTicker(caRootPruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.pruneCARoots(); err != nil {
				s.loggers.Named(logging.Connect).Error("error pruning CA roots", "error", err)
			}
		}
	}
}

// pruneCARoots looks for any CARoots that have been rotated out and expired.
func (s *Server) pruneCARoots() error {
	if !s.config.ConnectEnabled {
		return nil
	}

	state := s.fsm.State()
	idx, roots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	_, caConf, err := state.CAConfig(nil)
	if err != nil {
		return err
	}

	common, err := caConf.GetCommonConfig()
	if err != nil {
		return err
	}

	var newRoots structs.CARoots
	for _, r := range roots {
		if !r.Active && !r.RotatedOutAt.IsZero() && time.Now().Sub(r.RotatedOutAt) > common.LeafCertTTL*2 {
			s.loggers.Named(logging.Connect).Info("pruning old unused root CA", "id", r.ID)
			continue
		}
		newRoot := *r
		newRoots = append(newRoots, &newRoot)
	}

	// Return early if there's nothing to remove.
	if len(newRoots) == len(roots) {
		return nil
	}

	// Commit the new root state.
	var args structs.CARequest
	args.Op = structs.CAOpSetRoots
	args.Index = idx
	args.Roots = newRoots
	resp, err := s.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

// replicateIntentions executes a blocking query to the primary datacenter to replicate
// the intentions there to the local state.
func (s *Server) replicateIntentions(ctx context.Context) error {
	connectLogger := s.loggers.Named(logging.Connect)
	args := structs.DCSpecificRequest{
		Datacenter: s.config.PrimaryDatacenter,
	}

	connectLogger.Debug("starting Connect intention replication from primary datacenter", "primary", s.config.PrimaryDatacenter)

	retryLoopBackoff(ctx, func() error {
		// Always use the latest replication token value in case it changed while looping.
		args.QueryOptions.Token = s.tokens.ReplicationToken()

		var remote structs.IndexedIntentions
		if err := s.forwardDC("Intention.List", s.config.PrimaryDatacenter, &args, &remote); err != nil {
			return err
		}

		_, local, err := s.fsm.State().Intentions(nil)
		if err != nil {
			return err
		}

		// Compute the diff between the remote and local intentions.
		deletes, updates := diffIntentions(local, remote.Intentions)
		txnOpSets := batchIntentionUpdates(deletes, updates)

		// Apply batched updates to the state store.
		for _, ops := range txnOpSets {
			txnReq := structs.TxnRequest{Ops: ops}

			resp, err := s.raftApply(structs.TxnRequestType, &txnReq)
			if err != nil {
				return err
			}
			if respErr, ok := resp.(error); ok {
				return respErr
			}

			if txnResp, ok := resp.(structs.TxnResponse); ok {
				if len(txnResp.Errors) > 0 {
					return txnResp.Error()
				}
			} else {
				return fmt.Errorf("unexpected return type %T", resp)
			}
		}

		args.QueryOptions.MinQueryIndex = nextIndexVal(args.QueryOptions.MinQueryIndex, remote.QueryMeta.Index)
		return nil
	}, func(err error) {
		connectLogger.Error("error replicating intentions",
			"routine", intentionReplicationRoutineName,
			"error", err,
		)
	})
	return nil
}

// retryLoopBackoff loops a given function indefinitely, backing off exponentially
// upon errors up to a maximum of maxRetryBackoff seconds.
func retryLoopBackoff(ctx context.Context, loopFn func() error, errFn func(error)) {
	retryLoopBackoffHandleSuccess(ctx, loopFn, errFn, false)
}

func retryLoopBackoffAbortOnSuccess(ctx context.Context, loopFn func() error, errFn func(error)) {
	retryLoopBackoffHandleSuccess(ctx, loopFn, errFn, true)
}

func retryLoopBackoffHandleSuccess(ctx context.Context, loopFn func() error, errFn func(error), abortOnSuccess bool) {
	var failedAttempts uint
	limiter := rate.NewLimiter(loopRateLimit, retryBucketSize)
	for {
		// Rate limit how often we run the loop
		limiter.Wait(ctx)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if (1 << failedAttempts) < maxRetryBackoff {
			failedAttempts++
		}
		retryTime := (1 << failedAttempts) * time.Second

		if err := loopFn(); err != nil {
			errFn(err)

			timer := time.NewTimer(retryTime)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				continue
			}
		} else if abortOnSuccess {
			return
		}

		// Reset the failed attempts after a successful run.
		failedAttempts = 0
	}
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

// batchIntentionUpdates breaks up the given updates into sets of TxnOps based
// on the estimated size of the operations.
func batchIntentionUpdates(deletes, updates structs.Intentions) []structs.TxnOps {
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
			batchSize += txnOps[batchEnd].Intention.Intention.EstimateSize()
		}

		batchedOps = append(batchedOps, txnOps[batchStart:batchEnd])

		// txnOps[batchEnd] wasn't included as the slicing doesn't include the element at the stop index
		batchStart = batchEnd
	}

	return batchedOps
}

// nextIndexVal computes the next index value to query for, resetting to zero
// if the index went backward.
func nextIndexVal(prevIdx, idx uint64) uint64 {
	if prevIdx > idx {
		return 0
	}
	return idx
}

// halfTime returns a duration that is half the time between notBefore and
// notAfter.
func halfTime(notBefore, notAfter time.Time) time.Duration {
	interval := notAfter.Sub(notBefore)
	return interval / 2
}

// lessThanHalfTimePassed decides if half the time between notBefore and
// notAfter has passed relative to now.
// lessThanHalfTimePassed is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
func lessThanHalfTimePassed(now, notBefore, notAfter time.Time) bool {
	t := notBefore.Add(halfTime(notBefore, notAfter))
	return t.Sub(now) > 0
}

func (s *Server) generateCASignRequest(csr string) *structs.CASignRequest {
	return &structs.CASignRequest{
		Datacenter:   s.config.PrimaryDatacenter,
		CSR:          csr,
		WriteRequest: structs.WriteRequest{Token: s.tokens.ReplicationToken()},
	}
}
