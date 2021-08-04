package consul

import (
	"context"
	"time"

	"golang.org/x/time/rate"

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
)

var (
	// maxRetryBackoff is the maximum number of seconds to wait between failed blocking
	// queries when backing off.
	maxRetryBackoff = 256
)

// startConnectLeader starts multi-dc connect leader routines.
func (s *Server) startConnectLeader(ctx context.Context) error {
	if !s.config.ConnectEnabled {
		return nil
	}

	s.caManager.Start(ctx)
	s.leaderRoutineManager.Start(ctx, caRootPruningRoutineName, s.runCARootPruning)
	s.leaderRoutineManager.Start(ctx, caRootMetricRoutineName, rootCAExpiryMonitor(s).Monitor)
	s.leaderRoutineManager.Start(ctx, caSigningMetricRoutineName, signingCAExpiryMonitor(s).Monitor)

	return s.startIntentionConfigEntryMigration(ctx)
}

// stopConnectLeader stops connect specific leader functions.
func (s *Server) stopConnectLeader() {
	s.caManager.Stop()
	s.leaderRoutineManager.Stop(intentionMigrationRoutineName)
	s.leaderRoutineManager.Stop(caRootPruningRoutineName)
	s.leaderRoutineManager.Stop(caRootMetricRoutineName)
	s.leaderRoutineManager.Stop(caSigningMetricRoutineName)
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
	_, err = s.raftApply(structs.ConnectCARequestType, args)
	return err
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
