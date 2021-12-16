package consul

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/types"
)

var LeaderSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"leader", "barrier"},
		Help: "Measures the time spent waiting for the raft barrier upon gaining leadership.",
	},
	{
		Name: []string{"leader", "reconcileMember"},
		Help: "Measures the time spent updating the raft store for a single serf member's information.",
	},
	{
		Name: []string{"leader", "reapTombstones"},
		Help: "Measures the time spent clearing tombstones.",
	},
}

const (
	newLeaderEvent      = "consul:new-leader"
	barrierWriteTimeout = 2 * time.Minute
)

var (
	// caRootPruneInterval is how often we check for stale CARoots to remove.
	caRootPruneInterval = time.Hour

	// minCentralizedConfigVersion is the minimum Consul version in which centralized
	// config is supported
	minCentralizedConfigVersion = version.Must(version.NewVersion("1.5.0"))
)

// monitorLeadership is used to monitor if we acquire or lose our role
// as the leader in the Raft cluster. There is some work the leader is
// expected to do, so we must react to changes
func (s *Server) monitorLeadership() {
	// We use the notify channel we configured Raft with, NOT Raft's
	// leaderCh, which is only notified best-effort. Doing this ensures
	// that we get all notifications in order, which is required for
	// cleanup and to ensure we never run multiple leader loops.
	raftNotifyCh := s.raftNotifyCh

	var weAreLeaderCh chan struct{}
	var leaderLoop sync.WaitGroup
	for {
		select {
		case isLeader := <-raftNotifyCh:
			switch {
			case isLeader:
				if weAreLeaderCh != nil {
					s.logger.Error("attempted to start the leader loop while running")
					continue
				}

				weAreLeaderCh = make(chan struct{})
				leaderLoop.Add(1)
				go func(ch chan struct{}) {
					defer leaderLoop.Done()
					s.leaderLoop(ch)
				}(weAreLeaderCh)
				s.logger.Info("cluster leadership acquired")

			default:
				if weAreLeaderCh == nil {
					s.logger.Error("attempted to stop the leader loop while not running")
					continue
				}

				s.logger.Debug("shutting down leader loop")
				close(weAreLeaderCh)
				leaderLoop.Wait()
				weAreLeaderCh = nil
				s.logger.Info("cluster leadership lost")
			}
		case <-s.shutdownCh:
			return
		}
	}
}

func (s *Server) leadershipTransfer() error {
	retryCount := 3
	for i := 0; i < retryCount; i++ {
		future := s.raft.LeadershipTransfer()
		if err := future.Error(); err != nil {
			s.logger.Error("failed to transfer leadership attempt, will retry",
				"attempt", i,
				"retry_limit", retryCount,
				"error", err,
			)
		} else {
			s.logger.Info("successfully transferred leadership",
				"attempt", i,
				"retry_limit", retryCount,
			)
			return nil
		}

	}
	return fmt.Errorf("failed to transfer leadership in %d attempts", retryCount)
}

// leaderLoop runs as long as we are the leader to run various
// maintenance activities
func (s *Server) leaderLoop(stopCh chan struct{}) {
	stopCtx := &lib.StopChannelContext{StopCh: stopCh}

	// Fire a user event indicating a new leader
	payload := []byte(s.config.NodeName)
	if err := s.LANSendUserEvent(newLeaderEvent, payload, false); err != nil {
		s.logger.Warn("failed to broadcast new leader event", "error", err)
	}

	// Reconcile channel is only used once initial reconcile
	// has succeeded
	var reconcileCh chan serf.Member
	establishedLeader := false

RECONCILE:
	// Setup a reconciliation timer
	reconcileCh = nil
	interval := time.After(s.config.ReconcileInterval)

	// Apply a raft barrier to ensure our FSM is caught up
	start := time.Now()
	barrier := s.raft.Barrier(barrierWriteTimeout)
	if err := barrier.Error(); err != nil {
		s.logger.Error("failed to wait for barrier", "error", err)
		goto WAIT
	}
	metrics.MeasureSince([]string{"leader", "barrier"}, start)

	// Check if we need to handle initial leadership actions
	if !establishedLeader {
		if err := s.establishLeadership(stopCtx); err != nil {
			s.logger.Error("failed to establish leadership", "error", err)
			// Immediately revoke leadership since we didn't successfully
			// establish leadership.
			s.revokeLeadership()

			// attempt to transfer leadership. If successful it is
			// time to leave the leaderLoop since this node is no
			// longer the leader. If leadershipTransfer() fails, we
			// will try to acquire it again after
			// 5 seconds.
			if err := s.leadershipTransfer(); err != nil {
				s.logger.Error("failed to transfer leadership", "error", err)
				interval = time.After(5 * time.Second)
				goto WAIT
			}
			return
		}
		establishedLeader = true
		defer s.revokeLeadership()
	}

	// Reconcile any missing data
	if err := s.reconcile(); err != nil {
		s.logger.Error("failed to reconcile", "error", err)
		goto WAIT
	}

	// Initial reconcile worked, now we can process the channel
	// updates
	reconcileCh = s.reconcileCh

WAIT:
	// Poll the stop channel to give it priority so we don't waste time
	// trying to perform the other operations if we have been asked to shut
	// down.
	select {
	case <-stopCh:
		return
	default:
	}

	// Periodically reconcile as long as we are the leader,
	// or when Serf events arrive
	for {
		select {
		case <-stopCh:
			return
		case <-s.shutdownCh:
			return
		case <-interval:
			goto RECONCILE
		case member := <-reconcileCh:
			s.reconcileMember(member)
		case index := <-s.tombstoneGC.ExpireCh():
			go s.reapTombstones(index)
		case errCh := <-s.reassertLeaderCh:
			// we can get into this state when the initial
			// establishLeadership has failed as well as the follow
			// up leadershipTransfer. Afterwards we will be waiting
			// for the interval to trigger a reconciliation and can
			// potentially end up here. There is no point to
			// reassert because this agent was never leader in the
			// first place.
			if !establishedLeader {
				errCh <- fmt.Errorf("leadership has not been established")
				continue
			}

			// continue to reassert only if we previously were the
			// leader, which means revokeLeadership followed by an
			// establishLeadership().
			s.revokeLeadership()
			err := s.establishLeadership(stopCtx)
			errCh <- err

			// in case establishLeadership failed, we will try to
			// transfer leadership. At this time raft thinks we are
			// the leader, but consul disagrees.
			if err != nil {
				if err := s.leadershipTransfer(); err != nil {
					// establishedLeader was true before,
					// but it no longer is since it revoked
					// leadership and Leadership transfer
					// also failed. Which is why it stays
					// in the leaderLoop, but now
					// establishedLeader needs to be set to
					// false.
					establishedLeader = false
					interval = time.After(5 * time.Second)
					goto WAIT
				}

				// leadershipTransfer was successful and it is
				// time to leave the leaderLoop.
				return
			}

		}
	}
}

// establishLeadership is invoked once we become leader and are able
// to invoke an initial barrier. The barrier is used to ensure any
// previously inflight transactions have been committed and that our
// state is up-to-date.
func (s *Server) establishLeadership(ctx context.Context) error {
	start := time.Now()
	if err := s.initializeACLs(ctx); err != nil {
		return err
	}

	// Hint the tombstone expiration timer. When we freshly establish leadership
	// we become the authoritative timer, and so we need to start the clock
	// on any pending GC events.
	s.tombstoneGC.SetEnabled(true)
	lastIndex := s.raft.LastIndex()
	s.tombstoneGC.Hint(lastIndex)

	// Setup the session timers. This is done both when starting up or when
	// a leader fail over happens. Since the timers are maintained by the leader
	// node along, effectively this means all the timers are renewed at the
	// time of failover. The TTL contract is that the session will not be expired
	// before the TTL, so expiring it later is allowable.
	//
	// This MUST be done after the initial barrier to ensure the latest Sessions
	// are available to be initialized. Otherwise initialization may use stale
	// data.
	if err := s.initializeSessionTimers(); err != nil {
		return err
	}

	if err := s.establishEnterpriseLeadership(ctx); err != nil {
		return err
	}

	s.getOrCreateAutopilotConfig()
	s.autopilot.Start(ctx)

	s.startConfigReplication(ctx)

	s.startFederationStateReplication(ctx)

	s.startFederationStateAntiEntropy(ctx)

	if err := s.startConnectLeader(ctx); err != nil {
		return err
	}

	// Attempt to bootstrap config entries. We wait until after starting the
	// Connect leader tasks so we hopefully have transitioned to supporting
	// service-intentions.
	if err := s.bootstrapConfigEntries(s.config.ConfigEntryBootstrap); err != nil {
		return err
	}

	s.setConsistentReadReady()

	s.logger.Debug("successfully established leadership", "duration", time.Since(start))
	return nil
}

// revokeLeadership is invoked once we step down as leader.
// This is used to cleanup any state that may be specific to a leader.
func (s *Server) revokeLeadership() {
	// Disable the tombstone GC, since it is only useful as a leader
	s.tombstoneGC.SetEnabled(false)

	// Clear the session timers on either shutdown or step down, since we
	// are no longer responsible for session expirations.
	s.clearAllSessionTimers()

	s.revokeEnterpriseLeadership()

	s.stopFederationStateAntiEntropy()

	s.stopFederationStateReplication()

	s.stopConfigReplication()

	s.stopACLReplication()

	s.stopConnectLeader()

	s.stopACLTokenReaping()

	s.stopACLUpgrade()

	s.resetConsistentReadReady()

	// Stop returns a chan and we want to block until it is closed
	// which indicates that autopilot is actually stopped.
	<-s.autopilot.Stop()
}

// initializeACLs is used to setup the ACLs if we are the leader
// and need to do this.
func (s *Server) initializeACLs(ctx context.Context) error {
	if !s.config.ACLsEnabled {
		return nil
	}

	// Purge the cache, since it could've changed while we were not the
	// leader.
	s.acls.cache.Purge()

	// Purge the auth method validators since they could've changed while we
	// were not leader.
	s.aclAuthMethodValidators.Purge()

	// Remove any token affected by CVE-2019-8336
	if !s.InPrimaryDatacenter() {
		_, token, err := s.fsm.State().ACLTokenGetBySecret(nil, redactedToken, nil)
		if err == nil && token != nil {
			req := structs.ACLTokenBatchDeleteRequest{
				TokenIDs: []string{token.AccessorID},
			}

			_, err := s.raftApply(structs.ACLTokenDeleteRequestType, &req)
			if err != nil {
				return fmt.Errorf("failed to remove token with a redacted secret: %v", err)
			}
		}
	}

	if s.InPrimaryDatacenter() {
		s.logger.Info("initializing acls")

		// Create/Upgrade the builtin global-management policy
		_, policy, err := s.fsm.State().ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, structs.DefaultEnterpriseMetaInDefaultPartition())
		if err != nil {
			return fmt.Errorf("failed to get the builtin global-management policy")
		}
		if policy == nil || policy.Rules != structs.ACLPolicyGlobalManagement {
			newPolicy := structs.ACLPolicy{
				ID:             structs.ACLPolicyGlobalManagementID,
				Name:           "global-management",
				Description:    "Builtin Policy that grants unlimited access",
				Rules:          structs.ACLPolicyGlobalManagement,
				Syntax:         acl.SyntaxCurrent,
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			if policy != nil {
				newPolicy.Name = policy.Name
				newPolicy.Description = policy.Description
			}

			newPolicy.SetHash(true)

			req := structs.ACLPolicyBatchSetRequest{
				Policies: structs.ACLPolicies{&newPolicy},
			}
			_, err := s.raftApply(structs.ACLPolicySetRequestType, &req)
			if err != nil {
				return fmt.Errorf("failed to create global-management policy: %v", err)
			}
			s.logger.Info("Created ACL 'global-management' policy")
		}

		// Check for configured initial management token.
		if initialManagement := s.config.ACLInitialManagementToken; len(initialManagement) > 0 {
			state := s.fsm.State()
			if _, err := uuid.ParseUUID(initialManagement); err != nil {
				s.logger.Warn("Configuring a non-UUID initial management token is deprecated")
			}

			_, token, err := state.ACLTokenGetBySecret(nil, initialManagement, nil)
			if err != nil {
				return fmt.Errorf("failed to get initial management token: %v", err)
			}
			// Ignoring expiration times to avoid an insertion collision.
			if token == nil {
				accessor, err := lib.GenerateUUID(s.checkTokenUUID)
				if err != nil {
					return fmt.Errorf("failed to generate the accessor ID for the initial management token: %v", err)
				}

				token := structs.ACLToken{
					AccessorID:  accessor,
					SecretID:    initialManagement,
					Description: "Initial Management Token",
					Policies: []structs.ACLTokenPolicyLink{
						{
							ID: structs.ACLPolicyGlobalManagementID,
						},
					},
					CreateTime:     time.Now(),
					Local:          false,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				}

				token.SetHash(true)

				done := false
				if canBootstrap, _, err := state.CanBootstrapACLToken(); err == nil && canBootstrap {
					req := structs.ACLTokenBootstrapRequest{
						Token:      token,
						ResetIndex: 0,
					}
					if _, err := s.raftApply(structs.ACLBootstrapRequestType, &req); err == nil {
						s.logger.Info("Bootstrapped ACL initial management token from configuration")
						done = true
					} else {
						if err.Error() != structs.ACLBootstrapNotAllowedErr.Error() &&
							err.Error() != structs.ACLBootstrapInvalidResetIndexErr.Error() {
							return fmt.Errorf("failed to bootstrap initial management token: %v", err)
						}
					}
				}

				if !done {
					// either we didn't attempt to or setting the token with a bootstrap request failed.
					req := structs.ACLTokenBatchSetRequest{
						Tokens: structs.ACLTokens{&token},
						CAS:    false,
					}
					if _, err := s.raftApply(structs.ACLTokenSetRequestType, &req); err != nil {
						return fmt.Errorf("failed to create initial management token: %v", err)
					}

					s.logger.Info("Created ACL initial management token from configuration")
				}
			}
		}

		state := s.fsm.State()
		_, token, err := state.ACLTokenGetBySecret(nil, anonymousToken, nil)
		if err != nil {
			return fmt.Errorf("failed to get anonymous token: %v", err)
		}
		// Ignoring expiration times to avoid an insertion collision.
		if token == nil {
			token = &structs.ACLToken{
				AccessorID:     structs.ACLTokenAnonymousID,
				SecretID:       anonymousToken,
				Description:    "Anonymous Token",
				CreateTime:     time.Now(),
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			token.SetHash(true)

			req := structs.ACLTokenBatchSetRequest{
				Tokens: structs.ACLTokens{token},
				CAS:    false,
			}
			_, err := s.raftApply(structs.ACLTokenSetRequestType, &req)
			if err != nil {
				return fmt.Errorf("failed to create anonymous token: %v", err)
			}
			s.logger.Info("Created ACL anonymous token from configuration")
		}
		// launch the upgrade go routine to generate accessors for everything
		s.startACLUpgrade(ctx)
	} else {
		s.startACLReplication(ctx)
	}

	s.startACLTokenReaping(ctx)

	return nil
}

// legacyACLTokenUpgrade runs a single time to upgrade any tokens that may
// have been created immediately before the Consul upgrade, or any legacy tokens
// from a restored snapshot.
// TODO(ACL-Legacy-Compat): remove in phase 2
func (s *Server) legacyACLTokenUpgrade(ctx context.Context) error {
	// aclUpgradeRateLimit is the number of batch upgrade requests per second allowed.
	const aclUpgradeRateLimit rate.Limit = 1.0

	// aclUpgradeBatchSize controls how many tokens we look at during each round of upgrading. Individual raft logs
	// will be further capped using the aclBatchUpsertSize. This limit just prevents us from creating a single slice
	// with all tokens in it.
	const aclUpgradeBatchSize = 128

	limiter := rate.NewLimiter(aclUpgradeRateLimit, int(aclUpgradeRateLimit))
	for {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		// actually run the upgrade here
		state := s.fsm.State()
		tokens, _, err := state.ACLTokenListUpgradeable(aclUpgradeBatchSize)
		if err != nil {
			s.logger.Warn("encountered an error while searching for tokens without accessor ids", "error", err)
		}
		// No need to check expiration time here, as that only exists for v2 tokens.

		if len(tokens) == 0 {
			// No new legacy tokens can be created, so we can exit
			s.stopACLUpgrade() // required to prevent goroutine leak, according to TestAgentLeaks_Server
			return nil
		}

		var newTokens structs.ACLTokens
		for _, token := range tokens {
			// This should be entirely unnecessary but is just a small safeguard against changing accessor IDs
			if token.AccessorID != "" {
				continue
			}

			newToken := *token
			if token.SecretID == anonymousToken {
				newToken.AccessorID = structs.ACLTokenAnonymousID
			} else {
				accessor, err := lib.GenerateUUID(s.checkTokenUUID)
				if err != nil {
					s.logger.Warn("failed to generate accessor during token auto-upgrade", "error", err)
					continue
				}
				newToken.AccessorID = accessor
			}

			// Assign the global-management policy to legacy management tokens
			if len(newToken.Policies) == 0 &&
				len(newToken.ServiceIdentities) == 0 &&
				len(newToken.NodeIdentities) == 0 &&
				len(newToken.Roles) == 0 &&
				newToken.Type == "management" {
				newToken.Policies = append(newToken.Policies, structs.ACLTokenPolicyLink{ID: structs.ACLPolicyGlobalManagementID})
			}

			// need to copy these as we are going to do a CAS operation.
			newToken.CreateIndex = token.CreateIndex
			newToken.ModifyIndex = token.ModifyIndex

			newToken.SetHash(true)

			newTokens = append(newTokens, &newToken)
		}

		req := &structs.ACLTokenBatchSetRequest{Tokens: newTokens, CAS: true}

		_, err = s.raftApply(structs.ACLTokenSetRequestType, req)
		if err != nil {
			s.logger.Error("failed to apply acl token upgrade batch", "error", err)
		}
	}
}

// TODO(ACL-Legacy-Compat): remove in phase 2. Keeping it for now so that we
// can upgrade any tokens created immediately before the upgrade happens.
func (s *Server) startACLUpgrade(ctx context.Context) {
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		// token upgrades should only run in the primary
		return
	}

	s.leaderRoutineManager.Start(ctx, aclUpgradeRoutineName, s.legacyACLTokenUpgrade)
}

func (s *Server) stopACLUpgrade() {
	s.leaderRoutineManager.Stop(aclUpgradeRoutineName)
}

func (s *Server) startACLReplication(ctx context.Context) {
	if s.InPrimaryDatacenter() {
		return
	}

	// unlike some other leader routines this initializes some extra state
	// and therefore we want to prevent re-initialization if things are already
	// running
	if s.leaderRoutineManager.IsRunning(aclPolicyReplicationRoutineName) {
		return
	}

	s.initReplicationStatus()
	s.leaderRoutineManager.Start(ctx, aclPolicyReplicationRoutineName, s.runACLPolicyReplicator)
	s.leaderRoutineManager.Start(ctx, aclRoleReplicationRoutineName, s.runACLRoleReplicator)

	if s.config.ACLTokenReplication {
		s.leaderRoutineManager.Start(ctx, aclTokenReplicationRoutineName, s.runACLTokenReplicator)
		s.updateACLReplicationStatusRunning(structs.ACLReplicateTokens)
	} else {
		s.updateACLReplicationStatusRunning(structs.ACLReplicatePolicies)
	}
}

type replicateFunc func(ctx context.Context, logger hclog.Logger, lastRemoteIndex uint64) (uint64, bool, error)

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLPolicyReplicator(ctx context.Context) error {
	policyLogger := s.aclReplicationLogger(structs.ACLReplicatePolicies.SingularNoun())
	policyLogger.Info("started ACL Policy replication")
	return s.runACLReplicator(ctx, policyLogger, structs.ACLReplicatePolicies, s.replicateACLPolicies, "acl-policies")
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLRoleReplicator(ctx context.Context) error {
	roleLogger := s.aclReplicationLogger(structs.ACLReplicateRoles.SingularNoun())
	roleLogger.Info("started ACL Role replication")
	return s.runACLReplicator(ctx, roleLogger, structs.ACLReplicateRoles, s.replicateACLRoles, "acl-roles")
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLTokenReplicator(ctx context.Context) error {
	tokenLogger := s.aclReplicationLogger(structs.ACLReplicateTokens.SingularNoun())
	tokenLogger.Info("started ACL Token replication")
	return s.runACLReplicator(ctx, tokenLogger, structs.ACLReplicateTokens, s.replicateACLTokens, "acl-tokens")
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLReplicator(
	ctx context.Context,
	logger hclog.Logger,
	replicationType structs.ACLReplicationType,
	replicateFunc replicateFunc,
	metricName string,
) error {
	var failedAttempts uint
	limiter := rate.NewLimiter(rate.Limit(s.config.ACLReplicationRate), s.config.ACLReplicationBurst)

	var lastRemoteIndex uint64
	for {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		if s.tokens.ReplicationToken() == "" {
			continue
		}

		index, exit, err := replicateFunc(ctx, logger, lastRemoteIndex)
		if exit {
			return nil
		}

		if err != nil {
			metrics.SetGauge([]string{"leader", "replication", metricName, "status"},
				0,
			)
			lastRemoteIndex = 0
			s.updateACLReplicationStatusError(err.Error())
			logger.Warn("ACL replication error (will retry if still leader)",
				"error", err,
			)
			if (1 << failedAttempts) < aclReplicationMaxRetryBackoff {
				failedAttempts++
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After((1 << failedAttempts) * time.Second):
				// do nothing
			}
		} else {
			metrics.SetGauge([]string{"leader", "replication", metricName, "status"},
				1,
			)
			metrics.SetGauge([]string{"leader", "replication", metricName, "index"},
				float32(index),
			)
			lastRemoteIndex = index
			s.updateACLReplicationStatusIndex(replicationType, index)
			logger.Debug("ACL replication completed through remote index",
				"index", index,
			)
			failedAttempts = 0
		}
	}
}

func (s *Server) aclReplicationLogger(singularNoun string) hclog.Logger {
	return s.loggers.
		Named(logging.Replication).
		Named(logging.ACL).
		Named(singularNoun)
}

func (s *Server) stopACLReplication() {
	// these will be no-ops when not started
	s.leaderRoutineManager.Stop(aclPolicyReplicationRoutineName)
	s.leaderRoutineManager.Stop(aclRoleReplicationRoutineName)
	s.leaderRoutineManager.Stop(aclTokenReplicationRoutineName)
}

func (s *Server) startConfigReplication(ctx context.Context) {
	if s.config.PrimaryDatacenter == "" || s.config.PrimaryDatacenter == s.config.Datacenter {
		// replication shouldn't run in the primary DC
		return
	}

	s.leaderRoutineManager.Start(ctx, configReplicationRoutineName, s.configReplicator.Run)
}

func (s *Server) stopConfigReplication() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(configReplicationRoutineName)
}

func (s *Server) startFederationStateReplication(ctx context.Context) {
	if s.config.PrimaryDatacenter == "" || s.config.PrimaryDatacenter == s.config.Datacenter {
		// replication shouldn't run in the primary DC
		return
	}

	if s.gatewayLocator != nil {
		s.gatewayLocator.SetUseReplicationSignal(true)
		s.gatewayLocator.SetLastFederationStateReplicationError(nil, false)
	}

	s.leaderRoutineManager.Start(ctx, federationStateReplicationRoutineName, s.federationStateReplicator.Run)
}

func (s *Server) stopFederationStateReplication() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(federationStateReplicationRoutineName)

	if s.gatewayLocator != nil {
		s.gatewayLocator.SetUseReplicationSignal(false)
		s.gatewayLocator.SetLastFederationStateReplicationError(nil, false)
	}
}

// getOrCreateAutopilotConfig is used to get the autopilot config, initializing it if necessary
func (s *Server) getOrCreateAutopilotConfig() *structs.AutopilotConfig {
	logger := s.loggers.Named(logging.Autopilot)
	state := s.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		logger.Error("failed to get config", "error", err)
		return nil
	}
	if config != nil {
		return config
	}

	config = s.config.AutopilotConfig
	req := structs.AutopilotSetConfigRequest{Config: *config}
	if _, err = s.raftApply(structs.AutopilotRequestType, req); err != nil {
		logger.Error("failed to initialize config", "error", err)
		return nil
	}

	return config
}

func (s *Server) bootstrapConfigEntries(entries []structs.ConfigEntry) error {
	if s.config.PrimaryDatacenter != "" && s.config.PrimaryDatacenter != s.config.Datacenter {
		// only bootstrap in the primary datacenter
		return nil
	}

	if len(entries) < 1 {
		// nothing to initialize
		return nil
	}

	if ok, _ := ServersInDCMeetMinimumVersion(s, s.config.Datacenter, minCentralizedConfigVersion); !ok {
		s.loggers.
			Named(logging.CentralConfig).
			Warn("config: can't initialize until all servers >=" + minCentralizedConfigVersion.String())
		return nil
	}

	state := s.fsm.State()

	// Do some quick preflight checks to see if someone is doing something
	// that's not allowed at this time:
	//
	// - Trying to upgrade from an older pre-1.9.0 version of consul with
	// intentions AND are trying to bootstrap a service-intentions config entry
	// at the same time.
	//
	// - Trying to insert service-intentions config entries when connect is
	// disabled.

	usingConfigEntries, err := s.fsm.State().AreIntentionsInConfigEntries()
	if err != nil {
		return fmt.Errorf("Failed to determine if we are migrating intentions yet: %v", err)
	}

	if !usingConfigEntries || !s.config.ConnectEnabled {
		for _, entry := range entries {
			if entry.GetKind() == structs.ServiceIntentions {
				if !s.config.ConnectEnabled {
					return fmt.Errorf("Refusing to apply configuration entry %q / %q because Connect must be enabled to bootstrap intentions",
						entry.GetKind(), entry.GetName())
				}
				if !usingConfigEntries {
					return fmt.Errorf("Refusing to apply configuration entry %q / %q because intentions are still being migrated to config entries",
						entry.GetKind(), entry.GetName())
				}
			}
		}
	}

	for _, entry := range entries {
		// avoid a round trip through Raft if we know the CAS is going to fail
		_, existing, err := state.ConfigEntry(nil, entry.GetKind(), entry.GetName(), entry.GetEnterpriseMeta())
		if err != nil {
			return fmt.Errorf("Failed to determine whether the configuration for %q / %q already exists: %v", entry.GetKind(), entry.GetName(), err)
		}

		if existing == nil {
			// ensure the ModifyIndex is set to 0 for the CAS request
			entry.GetRaftIndex().ModifyIndex = 0

			req := structs.ConfigEntryRequest{
				Op:         structs.ConfigEntryUpsertCAS,
				Datacenter: s.config.Datacenter,
				Entry:      entry,
			}

			_, err := s.raftApply(structs.ConfigEntryRequestType, &req)
			if err != nil {
				return fmt.Errorf("Failed to apply configuration entry %q / %q: %v", entry.GetKind(), entry.GetName(), err)
			}
		}
	}
	return nil
}

// reconcileReaped is used to reconcile nodes that have failed and been reaped
// from Serf but remain in the catalog. This is done by looking for unknown nodes with serfHealth checks registered.
// We generate a "reap" event to cause the node to be cleaned up.
func (s *Server) reconcileReaped(known map[string]struct{}, nodeEntMeta *structs.EnterpriseMeta) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	state := s.fsm.State()
	_, checks, err := state.ChecksInState(nil, api.HealthAny, nodeEntMeta)
	if err != nil {
		return err
	}
	for _, check := range checks {
		// Ignore any non serf checks
		if check.CheckID != structs.SerfCheckID {
			continue
		}

		// Check if this node is "known" by serf
		if _, ok := known[check.Node]; ok {
			continue
		}

		// Get the node services, look for ConsulServiceID
		_, services, err := state.NodeServices(nil, check.Node, nodeEntMeta)
		if err != nil {
			return err
		}
		serverPort := 0
		serverAddr := ""
		serverID := ""

	CHECKS:
		for _, service := range services.Services {
			if service.ID == structs.ConsulServiceID {
				_, node, err := state.GetNode(check.Node, nodeEntMeta)
				if err != nil {
					s.logger.Error("Unable to look up node with name", "name", check.Node, "error", err)
					continue CHECKS
				}

				serverAddr = node.Address
				serverPort = service.Port
				lookupAddr := net.JoinHostPort(serverAddr, strconv.Itoa(serverPort))
				svr := s.serverLookup.Server(raft.ServerAddress(lookupAddr))
				if svr != nil {
					serverID = svr.ID
				}
				break
			}
		}

		// Create a fake member
		member := serf.Member{
			Name: check.Node,
			Tags: map[string]string{
				"dc":   s.config.Datacenter,
				"role": "node",
			},
		}
		addEnterpriseSerfTags(member.Tags, nodeEntMeta)

		// Create the appropriate tags if this was a server node
		if serverPort > 0 {
			member.Tags["role"] = "consul"
			member.Tags["port"] = strconv.FormatUint(uint64(serverPort), 10)
			member.Tags["id"] = serverID
			member.Addr = net.ParseIP(serverAddr)
		}

		// Attempt to reap this member
		if err := s.handleReapMember(member, nodeEntMeta); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMember is used to do an async reconcile of a single
// serf member
func (s *Server) reconcileMember(member serf.Member) error {
	// Check if this is a member we should handle
	if !s.shouldHandleMember(member) {
		s.logger.Warn("skipping reconcile of node",
			"member", member,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}
	defer metrics.MeasureSince([]string{"leader", "reconcileMember"}, time.Now())

	nodeEntMeta := getSerfMemberEnterpriseMeta(member)

	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.handleAliveMember(member, nodeEntMeta)
	case serf.StatusFailed:
		err = s.handleFailedMember(member, nodeEntMeta)
	case serf.StatusLeft:
		err = s.handleLeftMember(member, nodeEntMeta)
	case StatusReap:
		err = s.handleReapMember(member, nodeEntMeta)
	}
	if err != nil {
		s.logger.Error("failed to reconcile member",
			"member", member,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
			"error", err,
		)

		// Permission denied should not bubble up
		if acl.IsErrPermissionDenied(err) {
			return nil
		}
	}

	return nil
}

// shouldHandleMember checks if this is a Consul pool member
func (s *Server) shouldHandleMember(member serf.Member) bool {
	if valid, dc := isConsulNode(member); valid && dc == s.config.Datacenter {
		return true
	}
	if valid, parts := metadata.IsConsulServer(member); valid &&
		parts.Segment == "" &&
		parts.Datacenter == s.config.Datacenter {
		return true
	}
	return false
}

// handleAliveMember is used to ensure the node
// is registered, with a passing health check.
func (s *Server) handleAliveMember(member serf.Member, nodeEntMeta *structs.EnterpriseMeta) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Register consul service if a server
	var service *structs.NodeService
	if valid, parts := metadata.IsConsulServer(member); valid {
		service = &structs.NodeService{
			ID:      structs.ConsulServiceID,
			Service: structs.ConsulServiceName,
			Port:    parts.Port,
			Weights: &structs.Weights{
				Passing: 1,
				Warning: 1,
			},
			EnterpriseMeta: *nodeEntMeta,
			Meta: map[string]string{
				// DEPRECATED - remove nonvoter in favor of read_replica in a future version of consul
				"non_voter":             strconv.FormatBool(member.Tags["nonvoter"] == "1"),
				"read_replica":          strconv.FormatBool(member.Tags["read_replica"] == "1"),
				"raft_version":          strconv.Itoa(parts.RaftVersion),
				"serf_protocol_current": strconv.FormatUint(uint64(member.ProtocolCur), 10),
				"serf_protocol_min":     strconv.FormatUint(uint64(member.ProtocolMin), 10),
				"serf_protocol_max":     strconv.FormatUint(uint64(member.ProtocolMax), 10),
				"version":               parts.Build.String(),
			},
		}

		// Attempt to join the consul server
		if err := s.joinConsulServer(member, parts); err != nil {
			return err
		}
	}

	// Check if the node exists
	state := s.fsm.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta)
	if err != nil {
		return err
	}
	if node != nil && node.Address == member.Addr.String() {
		// Check if the associated service is available
		if service != nil {
			match := false
			_, services, err := state.NodeServices(nil, member.Name, nodeEntMeta)
			if err != nil {
				return err
			}
			if services != nil {
				for id, serv := range services.Services {
					if id == service.ID {
						// If metadata are different, be sure to update it
						match = reflect.DeepEqual(serv.Meta, service.Meta)
					}
				}
			}
			if !match {
				goto AFTER_CHECK
			}
		}

		// Check if the serfCheck is in the passing state
		_, checks, err := state.NodeChecks(nil, member.Name, nodeEntMeta)
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.CheckID == structs.SerfCheckID && check.Status == api.HealthPassing {
				return nil
			}
		}
	}
AFTER_CHECK:
	s.logger.Info("member joined, marking health alive",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	// Register with the catalog.
	req := structs.RegisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
		ID:         types.NodeID(member.Tags["id"]),
		Address:    member.Addr.String(),
		Service:    service,
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthPassing,
			Output:  structs.SerfCheckAliveOutput,
		},
		EnterpriseMeta: *nodeEntMeta,
	}
	if node != nil {
		req.TaggedAddresses = node.TaggedAddresses
		req.NodeMeta = node.Meta
	}

	_, err = s.raftApply(structs.RegisterRequestType, &req)
	return err
}

// handleFailedMember is used to mark the node's status
// as being critical, along with all checks as unknown.
func (s *Server) handleFailedMember(member serf.Member, nodeEntMeta *structs.EnterpriseMeta) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Check if the node exists
	state := s.fsm.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta)
	if err != nil {
		return err
	}

	if node == nil {
		s.logger.Info("ignoring failed event for member because it does not exist in the catalog",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}

	if node.Address == member.Addr.String() {
		// Check if the serfCheck is in the critical state
		_, checks, err := state.NodeChecks(nil, member.Name, nodeEntMeta)
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.CheckID == structs.SerfCheckID && check.Status == api.HealthCritical {
				return nil
			}
		}
	}
	s.logger.Info("member failed, marking health critical",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter:     s.config.Datacenter,
		Node:           member.Name,
		EnterpriseMeta: *nodeEntMeta,
		ID:             types.NodeID(member.Tags["id"]),
		Address:        member.Addr.String(),
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthCritical,
			Output:  structs.SerfCheckFailedOutput,
		},

		// If there's existing information about the node, do not
		// clobber it.
		SkipNodeUpdate: true,
	}
	_, err = s.raftApply(structs.RegisterRequestType, &req)
	return err
}

// handleLeftMember is used to handle members that gracefully
// left. They are deregistered if necessary.
func (s *Server) handleLeftMember(member serf.Member, nodeEntMeta *structs.EnterpriseMeta) error {
	return s.handleDeregisterMember("left", member, nodeEntMeta)
}

// handleReapMember is used to handle members that have been
// reaped after a prolonged failure. They are deregistered.
func (s *Server) handleReapMember(member serf.Member, nodeEntMeta *structs.EnterpriseMeta) error {
	return s.handleDeregisterMember("reaped", member, nodeEntMeta)
}

// handleDeregisterMember is used to deregister a member of a given reason
func (s *Server) handleDeregisterMember(reason string, member serf.Member, nodeEntMeta *structs.EnterpriseMeta) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Do not deregister ourself. This can only happen if the current leader
	// is leaving. Instead, we should allow a follower to take-over and
	// deregister us later.
	//
	// TODO(partitions): check partitions here too? server names should be unique in general though
	if member.Name == s.config.NodeName {
		s.logger.Warn("deregistering self should be done by follower",
			"name", s.config.NodeName,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}

	// Remove from Raft peers if this was a server
	if valid, _ := metadata.IsConsulServer(member); valid {
		if err := s.removeConsulServer(member); err != nil {
			return err
		}
	}

	// Check if the node does not exist
	state := s.fsm.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}

	// Deregister the node
	s.logger.Info("deregistering member",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		"reason", reason,
	)
	req := structs.DeregisterRequest{
		Datacenter:     s.config.Datacenter,
		Node:           member.Name,
		EnterpriseMeta: *nodeEntMeta,
	}
	_, err = s.raftApply(structs.DeregisterRequestType, &req)
	return err
}

// joinConsulServer is used to try to join another consul server
func (s *Server) joinConsulServer(m serf.Member, parts *metadata.Server) error {
	// Check for possibility of multiple bootstrap nodes
	if parts.Bootstrap {
		members := s.serfLAN.Members()
		for _, member := range members {
			valid, p := metadata.IsConsulServer(member)
			if valid && member.Name != m.Name && p.Bootstrap {
				s.logger.Error("Two nodes are in bootstrap mode. Only one node should be in bootstrap mode, not adding Raft peer.",
					"node_to_add", m.Name,
					"other", member.Name,
				)
				return nil
			}
		}
	}

	// We used to do a check here and prevent adding the server if the cluster size was too small (1 or 2 servers) as a means
	// of preventing the case where we may remove ourselves and cause a loss of leadership. The Autopilot AddServer function
	// will now handle simple address updates better and so long as the address doesn't conflict with another node
	// it will not require a removal but will instead just update the address. If it would require a removal of other nodes
	// due to conflicts then the logic regarding cluster sizes will kick in and prevent doing anything dangerous that could
	// cause loss of leadership.

	// get the autpilot library version of a server from the serf member
	apServer, err := s.autopilotServer(m)
	if err != nil {
		return err
	}

	// now ask autopilot to add it
	return s.autopilot.AddServer(apServer)
}

// removeConsulServer is used to try to remove a consul server that has left
func (s *Server) removeConsulServer(m serf.Member) error {
	server, err := s.autopilotServer(m)
	if err != nil || server == nil {
		return err
	}

	return s.autopilot.RemoveServer(server.ID)
}

// reapTombstones is invoked by the current leader to manage garbage
// collection of tombstones. When a key is deleted, we trigger a tombstone
// GC clock. Once the expiration is reached, this routine is invoked
// to clear all tombstones before this index. This must be replicated
// through Raft to ensure consistency. We do this outside the leader loop
// to avoid blocking.
func (s *Server) reapTombstones(index uint64) {
	defer metrics.MeasureSince([]string{"leader", "reapTombstones"}, time.Now())
	req := structs.TombstoneRequest{
		Datacenter: s.config.Datacenter,
		Op:         structs.TombstoneReap,
		ReapIndex:  index,
	}
	_, err := s.raftApply(structs.TombstoneRequestType, &req)
	if err != nil {
		s.logger.Error("failed to reap tombstones up to index",
			"index", index,
			"error", err,
		)
	}
}

func (s *Server) setDatacenterSupportsFederationStates() {
	atomic.StoreInt32(&s.dcSupportsFederationStates, 1)
}

func (s *Server) DatacenterSupportsFederationStates() bool {
	if atomic.LoadInt32(&s.dcSupportsFederationStates) != 0 {
		return true
	}

	state := serversFederationStatesInfo{
		supported: true,
		found:     false,
	}

	// if we are in a secondary, check if they are supported in the primary dc
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		s.router.CheckServers(s.config.PrimaryDatacenter, state.update)

		if !state.supported || !state.found {
			s.logger.Debug("federation states are not enabled in the primary dc")
			return false
		}
	}

	// check the servers in the local DC
	s.router.CheckServers(s.config.Datacenter, state.update)

	if state.supported && state.found {
		s.setDatacenterSupportsFederationStates()
		return true
	}

	s.logger.Debug("federation states are not enabled in this datacenter", "datacenter", s.config.Datacenter)
	return false
}

type serversFederationStatesInfo struct {
	// supported indicates whether every processed server supports federation states
	supported bool

	// found indicates that at least one server was processed
	found bool
}

func (s *serversFederationStatesInfo) update(srv *metadata.Server) bool {
	if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
		// they are left or something so regardless we treat these servers as meeting
		// the version requirement
		return true
	}

	// mark that we processed at least one server
	s.found = true

	if supported, ok := srv.FeatureFlags["fs"]; ok && supported == 1 {
		return true
	}

	// mark that at least one server does not support federation states
	s.supported = false

	// prevent continuing server evaluation
	return false
}

func (s *Server) setDatacenterSupportsIntentionsAsConfigEntries() {
	atomic.StoreInt32(&s.dcSupportsIntentionsAsConfigEntries, 1)
}

func (s *Server) DatacenterSupportsIntentionsAsConfigEntries() bool {
	if atomic.LoadInt32(&s.dcSupportsIntentionsAsConfigEntries) != 0 {
		return true
	}

	state := serversIntentionsAsConfigEntriesInfo{
		supported: true,
		found:     false,
	}

	// if we are in a secondary, check if they are supported in the primary dc
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		s.router.CheckServers(s.config.PrimaryDatacenter, state.update)

		if !state.supported || !state.found {
			s.logger.Debug("intentions have not been migrated to config entries in the primary dc yet")
			return false
		}
	}

	// check the servers in the local DC
	s.router.CheckServers(s.config.Datacenter, state.update)

	if state.supported && state.found {
		s.setDatacenterSupportsIntentionsAsConfigEntries()
		return true
	}

	s.logger.Debug("intentions cannot be migrated to config entries in this datacenter", "datacenter", s.config.Datacenter)
	return false
}

type serversIntentionsAsConfigEntriesInfo struct {
	// supported indicates whether every processed server supports intentions as config entries
	supported bool

	// found indicates that at least one server was processed
	found bool
}

func (s *serversIntentionsAsConfigEntriesInfo) update(srv *metadata.Server) bool {
	if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
		// they are left or something so regardless we treat these servers as meeting
		// the version requirement
		return true
	}

	// mark that we processed at least one server
	s.found = true

	if supported, ok := srv.FeatureFlags["si"]; ok && supported == 1 {
		return true
	}

	// mark that at least one server does not support service-intentions
	s.supported = false

	// prevent continuing server evaluation
	return false
}
