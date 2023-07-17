package consul

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-version"
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

	// minVirtualIPVersion is the minimum version for all Consul servers for virtual IP
	// assignment to be enabled.
	minVirtualIPVersion = version.Must(version.NewVersion("1.11.0"))

	// minVirtualIPVersion is the minimum version for all Consul servers for virtual IP
	// assignment to be enabled for terminating gateways.
	minVirtualIPTerminatingGatewayVersion = version.Must(version.NewVersion("1.11.2"))

	// virtualIPVersionCheckInterval is the frequency we check whether all servers meet
	// the minimum version to enable virtual IP assignment for services.
	virtualIPVersionCheckInterval = time.Minute
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
	s.leaderRoutineManager.Start(ctx, virtualIPCheckRoutineName, s.runVirtualIPVersionCheck)

	return s.startIntentionConfigEntryMigration(ctx)
}

// stopConnectLeader stops connect specific leader functions.
func (s *Server) stopConnectLeader() {
	s.caManager.Stop()
	s.leaderRoutineManager.Stop(intentionMigrationRoutineName)
	s.leaderRoutineManager.Stop(caRootPruningRoutineName)
	s.leaderRoutineManager.Stop(caRootMetricRoutineName)
	s.leaderRoutineManager.Stop(caSigningMetricRoutineName)
	s.leaderRoutineManager.Stop(virtualIPCheckRoutineName)
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

func (s *Server) runVirtualIPVersionCheck(ctx context.Context) error {
	// Return early if the flag is already set.
	done, err := s.setVirtualIPFlags()
	if err != nil {
		s.loggers.Named(logging.Connect).Warn("error enabling virtual IPs", "error", err)
	}
	if done {
		s.leaderRoutineManager.Stop(virtualIPCheckRoutineName)
		return nil
	}

	ticker := time.NewTicker(virtualIPVersionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			done, err := s.setVirtualIPFlags()
			if err != nil {
				s.loggers.Named(logging.Connect).Warn("error enabling virtual IPs", "error", err)
				continue
			}
			if done {
				return nil
			}
		}
	}
}

func (s *Server) setVirtualIPFlags() (bool, error) {
	virtualIPFlag, err := s.setVirtualIPVersionFlag()
	if err != nil {
		return false, err
	}
	terminatingGatewayVirtualIPFlag, err := s.setVirtualIPTerminatingGatewayVersionFlag()
	if err != nil {
		return false, err
	}

	return virtualIPFlag && terminatingGatewayVirtualIPFlag, nil
}

func (s *Server) setVirtualIPVersionFlag() (bool, error) {
	val, err := s.GetSystemMetadata(structs.SystemMetadataVirtualIPsEnabled)
	if err != nil {
		return false, err
	}
	if val != "" {
		return true, nil
	}

	if ok, _ := ServersInDCMeetMinimumVersion(s, s.config.Datacenter, minVirtualIPVersion); !ok {
		return false, fmt.Errorf("can't allocate Virtual IPs until all servers >= %s",
			minVirtualIPVersion.String())
	}

	if err := s.SetSystemMetadataKey(structs.SystemMetadataVirtualIPsEnabled, "true"); err != nil {
		return false, nil
	}

	return true, nil
}

func (s *Server) setVirtualIPTerminatingGatewayVersionFlag() (bool, error) {
	val, err := s.GetSystemMetadata(structs.SystemMetadataTermGatewayVirtualIPsEnabled)
	if err != nil {
		return false, err
	}
	if val != "" {
		return true, nil
	}

	if ok, _ := ServersInDCMeetMinimumVersion(s, s.config.Datacenter, minVirtualIPTerminatingGatewayVersion); !ok {
		return false, fmt.Errorf("can't allocate Virtual IPs for terminating gateways until all servers >= %s",
			minVirtualIPTerminatingGatewayVersion.String())
	}

	if err := s.SetSystemMetadataKey(structs.SystemMetadataTermGatewayVirtualIPsEnabled, "true"); err != nil {
		return false, nil
	}

	return true, nil
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
