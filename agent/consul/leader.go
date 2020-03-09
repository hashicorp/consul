package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"
)

const (
	newLeaderEvent      = "consul:new-leader"
	barrierWriteTimeout = 2 * time.Minute
)

var (
	// caRootPruneInterval is how often we check for stale CARoots to remove.
	caRootPruneInterval = time.Hour

	// minAutopilotVersion is the minimum Consul version in which Autopilot features
	// are supported.
	minAutopilotVersion = version.Must(version.NewVersion("0.8.0"))

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

	aclModeCheckWait := aclModeCheckMinInterval
	var aclUpgradeCh <-chan time.Time
	if s.ACLsEnabled() {
		aclUpgradeCh = time.After(aclModeCheckWait)
	}
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
		case <-aclUpgradeCh:
			if atomic.LoadInt32(&s.useNewACLs) == 0 {
				aclModeCheckWait = aclModeCheckWait * 2
				if aclModeCheckWait > aclModeCheckMaxInterval {
					aclModeCheckWait = aclModeCheckMaxInterval
				}
				aclUpgradeCh = time.After(aclModeCheckWait)

				if canUpgrade := s.canUpgradeToNewACLs(weAreLeaderCh != nil); canUpgrade {
					if weAreLeaderCh != nil {
						if err := s.initializeACLs(true); err != nil {
							s.logger.Error("error transitioning to using new ACLs", "error", err)
							continue
						}
					}

					s.logger.Debug("transitioning out of legacy ACL mode")
					atomic.StoreInt32(&s.useNewACLs, 1)
					s.updateACLAdvertisement()

					// setting this to nil ensures that we will never hit this case again
					aclUpgradeCh = nil
				}
			} else {
				// establishLeadership probably transitioned us
				aclUpgradeCh = nil
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
	// Fire a user event indicating a new leader
	payload := []byte(s.config.NodeName)
	for name, segment := range s.LANSegments() {
		if err := segment.UserEvent(newLeaderEvent, payload, false); err != nil {
			s.logger.Warn("failed to broadcast new leader event on segment",
				"segment", name,
				"error", err,
			)
		}
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
		if err := s.establishLeadership(); err != nil {
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
			err := s.establishLeadership()
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
func (s *Server) establishLeadership() error {
	// check for the upgrade here - this helps us transition to new ACLs much
	// quicker if this is a new cluster or this is a test agent
	if canUpgrade := s.canUpgradeToNewACLs(true); canUpgrade {
		if err := s.initializeACLs(true); err != nil {
			return err
		}
		atomic.StoreInt32(&s.useNewACLs, 1)
		s.updateACLAdvertisement()
	} else if err := s.initializeACLs(false); err != nil {
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

	if err := s.establishEnterpriseLeadership(); err != nil {
		return err
	}

	// attempt to bootstrap config entries
	if err := s.bootstrapConfigEntries(s.config.ConfigEntryBootstrap); err != nil {
		return err
	}

	s.getOrCreateAutopilotConfig()
	s.autopilot.Start()

	// todo(kyhavlov): start a goroutine here for handling periodic CA rotation
	if err := s.initializeCA(); err != nil {
		return err
	}

	s.startConfigReplication()

	s.startFederationStateReplication()

	s.startFederationStateAntiEntropy()

	s.startConnectLeader()

	s.setConsistentReadReady()
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

	s.stopConnectLeader()

	s.setCAProvider(nil, nil)

	s.stopACLTokenReaping()

	s.stopACLUpgrade()

	s.resetConsistentReadReady()

	s.autopilot.Stop()
}

// DEPRECATED (ACL-Legacy-Compat) - Remove once old ACL compatibility is removed
func (s *Server) initializeLegacyACL() error {
	if !s.ACLsEnabled() {
		return nil
	}

	authDC := s.config.ACLDatacenter

	// Create anonymous token if missing.
	state := s.fsm.State()
	_, token, err := state.ACLTokenGetBySecret(nil, anonymousToken, nil)
	if err != nil {
		return fmt.Errorf("failed to get anonymous token: %v", err)
	}
	// Ignoring expiration times to avoid an insertion collision.
	if token == nil {
		req := structs.ACLRequest{
			Datacenter: authDC,
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				ID:   anonymousToken,
				Name: "Anonymous Token",
				Type: structs.ACLTokenTypeClient,
			},
		}
		_, err := s.raftApply(structs.ACLRequestType, &req)
		if err != nil {
			return fmt.Errorf("failed to create anonymous token: %v", err)
		}
		s.logger.Info("Created the anonymous token")
	}

	// Check for configured master token.
	if master := s.config.ACLMasterToken; len(master) > 0 {
		_, token, err = state.ACLTokenGetBySecret(nil, master, nil)
		if err != nil {
			return fmt.Errorf("failed to get master token: %v", err)
		}
		// Ignoring expiration times to avoid an insertion collision.
		if token == nil {
			req := structs.ACLRequest{
				Datacenter: authDC,
				Op:         structs.ACLSet,
				ACL: structs.ACL{
					ID:   master,
					Name: "Master Token",
					Type: structs.ACLTokenTypeManagement,
				},
			}
			_, err := s.raftApply(structs.ACLRequestType, &req)
			if err != nil {
				return fmt.Errorf("failed to create master token: %v", err)
			}
			s.logger.Info("Created ACL master token from configuration")
		}
	}

	// Check to see if we need to initialize the ACL bootstrap info. This
	// needs a Consul version check since it introduces a new Raft operation
	// that'll produce an error on older servers, and it also makes a piece
	// of state in the state store that will cause problems with older
	// servers consuming snapshots, so we have to wait to create it.
	var minVersion = version.Must(version.NewVersion("0.9.1"))
	if ServersMeetMinimumVersion(s.LANMembers(), minVersion) {
		canBootstrap, _, err := state.CanBootstrapACLToken()
		if err != nil {
			return fmt.Errorf("failed looking for ACL bootstrap info: %v", err)
		}
		if canBootstrap {
			req := structs.ACLRequest{
				Datacenter: authDC,
				Op:         structs.ACLBootstrapInit,
			}
			resp, err := s.raftApply(structs.ACLRequestType, &req)
			if err != nil {
				return fmt.Errorf("failed to initialize ACL bootstrap: %v", err)
			}
			switch v := resp.(type) {
			case error:
				return fmt.Errorf("failed to initialize ACL bootstrap: %v", v)

			case bool:
				if v {
					s.logger.Info("ACL bootstrap enabled")
				} else {
					s.logger.Info("ACL bootstrap disabled, existing management tokens found")
				}

			default:
				return fmt.Errorf("unexpected response trying to initialize ACL bootstrap: %T", v)
			}
		}
	} else {
		s.logger.Warn("Can't initialize ACL bootstrap until all servers are >= " + minVersion.String())
	}

	return nil
}

// initializeACLs is used to setup the ACLs if we are the leader
// and need to do this.
func (s *Server) initializeACLs(upgrade bool) error {
	if !s.ACLsEnabled() {
		return nil
	}

	// Purge the cache, since it could've changed while we were not the
	// leader.
	s.acls.cache.Purge()

	// Purge the auth method validators since they could've changed while we
	// were not leader.
	s.aclAuthMethodValidators.Purge()

	// Remove any token affected by CVE-2019-8336
	if !s.InACLDatacenter() {
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

	if s.InACLDatacenter() {
		if s.UseLegacyACLs() && !upgrade {
			s.logger.Info("initializing legacy acls")
			return s.initializeLegacyACL()
		}

		s.logger.Info("initializing acls")

		// Create/Upgrade the builtin global-management policy
		_, policy, err := s.fsm.State().ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, structs.DefaultEnterpriseMeta())
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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

		// Check for configured master token.
		if master := s.config.ACLMasterToken; len(master) > 0 {
			state := s.fsm.State()
			if _, err := uuid.ParseUUID(master); err != nil {
				s.logger.Warn("Configuring a non-UUID master token is deprecated")
			}

			_, token, err := state.ACLTokenGetBySecret(nil, master, nil)
			if err != nil {
				return fmt.Errorf("failed to get master token: %v", err)
			}
			// Ignoring expiration times to avoid an insertion collision.
			if token == nil {
				accessor, err := lib.GenerateUUID(s.checkTokenUUID)
				if err != nil {
					return fmt.Errorf("failed to generate the accessor ID for the master token: %v", err)
				}

				token := structs.ACLToken{
					AccessorID:  accessor,
					SecretID:    master,
					Description: "Master Token",
					Policies: []structs.ACLTokenPolicyLink{
						{
							ID: structs.ACLPolicyGlobalManagementID,
						},
					},
					CreateTime: time.Now(),
					Local:      false,

					// DEPRECATED (ACL-Legacy-Compat) - only needed for compatibility
					Type:           structs.ACLTokenTypeManagement,
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				}

				token.SetHash(true)

				done := false
				if canBootstrap, _, err := state.CanBootstrapACLToken(); err == nil && canBootstrap {
					req := structs.ACLTokenBootstrapRequest{
						Token:      token,
						ResetIndex: 0,
					}
					if _, err := s.raftApply(structs.ACLBootstrapRequestType, &req); err == nil {
						s.logger.Info("Bootstrapped ACL master token from configuration")
						done = true
					} else {
						if err.Error() != structs.ACLBootstrapNotAllowedErr.Error() &&
							err.Error() != structs.ACLBootstrapInvalidResetIndexErr.Error() {
							return fmt.Errorf("failed to bootstrap master token: %v", err)
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
						return fmt.Errorf("failed to create master token: %v", err)
					}

					s.logger.Info("Created ACL master token from configuration")
				}
			}
		}

		state := s.fsm.State()
		_, token, err := state.ACLTokenGetBySecret(nil, structs.ACLTokenAnonymousID, nil)
		if err != nil {
			return fmt.Errorf("failed to get anonymous token: %v", err)
		}
		// Ignoring expiration times to avoid an insertion collision.
		if token == nil {
			// DEPRECATED (ACL-Legacy-Compat) - Don't need to query for previous "anonymous" token
			// check for legacy token that needs an upgrade
			_, legacyToken, err := state.ACLTokenGetBySecret(nil, anonymousToken, nil)
			if err != nil {
				return fmt.Errorf("failed to get anonymous token: %v", err)
			}
			// Ignoring expiration times to avoid an insertion collision.

			// the token upgrade routine will take care of upgrading the token if a legacy version exists
			if legacyToken == nil {
				token = &structs.ACLToken{
					AccessorID:     structs.ACLTokenAnonymousID,
					SecretID:       anonymousToken,
					Description:    "Anonymous Token",
					CreateTime:     time.Now(),
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
		}
		// launch the upgrade go routine to generate accessors for everything
		s.startACLUpgrade()
	} else {
		if s.UseLegacyACLs() && !upgrade {
			if s.IsACLReplicationEnabled() {
				s.startLegacyACLReplication()
			}
		}

		if upgrade {
			s.stopACLReplication()
		}

		// ACL replication is now mandatory
		s.startACLReplication()
	}

	s.startACLTokenReaping()

	return nil
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) legacyACLTokenUpgrade(ctx context.Context) error {
	limiter := rate.NewLimiter(aclUpgradeRateLimit, int(aclUpgradeRateLimit))
	for {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		// actually run the upgrade here
		state := s.fsm.State()
		tokens, waitCh, err := state.ACLTokenListUpgradeable(aclUpgradeBatchSize)
		if err != nil {
			s.logger.Warn("encountered an error while searching for tokens without accessor ids", "error", err)
		}
		// No need to check expiration time here, as that only exists for v2 tokens.

		if len(tokens) == 0 {
			ws := memdb.NewWatchSet()
			ws.Add(state.AbandonCh())
			ws.Add(waitCh)
			ws.Add(ctx.Done())

			// wait for more tokens to need upgrading or the aclUpgradeCh to be closed
			ws.Watch(nil)
			continue
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
				len(newToken.Roles) == 0 &&
				newToken.Type == structs.ACLTokenTypeManagement {
				newToken.Policies = append(newToken.Policies, structs.ACLTokenPolicyLink{ID: structs.ACLPolicyGlobalManagementID})
			}

			// need to copy these as we are going to do a CAS operation.
			newToken.CreateIndex = token.CreateIndex
			newToken.ModifyIndex = token.ModifyIndex

			newToken.SetHash(true)

			newTokens = append(newTokens, &newToken)
		}

		req := &structs.ACLTokenBatchSetRequest{Tokens: newTokens, CAS: true}

		resp, err := s.raftApply(structs.ACLTokenSetRequestType, req)
		if err != nil {
			s.logger.Error("failed to apply acl token upgrade batch", "error", err)
		}

		if err, ok := resp.(error); ok {
			s.logger.Error("failed to apply acl token upgrade batch", "error", err)
		}
	}
}

func (s *Server) startACLUpgrade() {
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		// token upgrades should only run in the primary
		return
	}

	s.leaderRoutineManager.Start(aclUpgradeRoutineName, s.legacyACLTokenUpgrade)
}

func (s *Server) stopACLUpgrade() {
	s.leaderRoutineManager.Stop(aclUpgradeRoutineName)
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runLegacyACLReplication(ctx context.Context) error {
	var lastRemoteIndex uint64
	legacyACLLogger := s.aclReplicationLogger(logging.Legacy)
	limiter := rate.NewLimiter(rate.Limit(s.config.ACLReplicationRate), s.config.ACLReplicationBurst)

	for {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		if s.tokens.ReplicationToken() == "" {
			continue
		}

		index, exit, err := s.replicateLegacyACLs(ctx, legacyACLLogger, lastRemoteIndex)
		if exit {
			return nil
		}

		if err != nil {
			lastRemoteIndex = 0
			s.updateACLReplicationStatusError()
			legacyACLLogger.Warn("Legacy ACL replication error (will retry if still leader)", "error", err)
		} else {
			lastRemoteIndex = index
			s.updateACLReplicationStatusIndex(structs.ACLReplicateLegacy, index)
			legacyACLLogger.Debug("Legacy ACL replication completed through remote index", "index", index)
		}
	}
}

func (s *Server) startLegacyACLReplication() {
	if s.InACLDatacenter() {
		return
	}

	// unlike some other leader routines this initializes some extra state
	// and therefore we want to prevent re-initialization if things are already
	// running
	if s.leaderRoutineManager.IsRunning(legacyACLReplicationRoutineName) {
		return
	}

	s.initReplicationStatus()

	s.leaderRoutineManager.Start(legacyACLReplicationRoutineName, s.runLegacyACLReplication)
	s.logger.Info("started legacy ACL replication")
	s.updateACLReplicationStatusRunning(structs.ACLReplicateLegacy)
}

func (s *Server) startACLReplication() {
	if s.InACLDatacenter() {
		return
	}

	// unlike some other leader routines this initializes some extra state
	// and therefore we want to prevent re-initialization if things are already
	// running
	if s.leaderRoutineManager.IsRunning(aclPolicyReplicationRoutineName) {
		return
	}

	s.initReplicationStatus()
	s.leaderRoutineManager.Start(aclPolicyReplicationRoutineName, s.runACLPolicyReplicator)
	s.leaderRoutineManager.Start(aclRoleReplicationRoutineName, s.runACLRoleReplicator)

	if s.config.ACLTokenReplication {
		s.leaderRoutineManager.Start(aclTokenReplicationRoutineName, s.runACLTokenReplicator)
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
	return s.runACLReplicator(ctx, policyLogger, structs.ACLReplicatePolicies, s.replicateACLPolicies)
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLRoleReplicator(ctx context.Context) error {
	roleLogger := s.aclReplicationLogger(structs.ACLReplicateRoles.SingularNoun())
	roleLogger.Info("started ACL Role replication")
	return s.runACLReplicator(ctx, roleLogger, structs.ACLReplicateRoles, s.replicateACLRoles)
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLTokenReplicator(ctx context.Context) error {
	tokenLogger := s.aclReplicationLogger(structs.ACLReplicateTokens.SingularNoun())
	tokenLogger.Info("started ACL Token replication")
	return s.runACLReplicator(ctx, tokenLogger, structs.ACLReplicateTokens, s.replicateACLTokens)
}

// This function is only intended to be run as a managed go routine, it will block until
// the context passed in indicates that it should exit.
func (s *Server) runACLReplicator(
	ctx context.Context,
	logger hclog.Logger,
	replicationType structs.ACLReplicationType,
	replicateFunc replicateFunc,
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
			lastRemoteIndex = 0
			s.updateACLReplicationStatusError()
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
	s.leaderRoutineManager.Stop(legacyACLReplicationRoutineName)
	s.leaderRoutineManager.Stop(aclPolicyReplicationRoutineName)
	s.leaderRoutineManager.Stop(aclRoleReplicationRoutineName)
	s.leaderRoutineManager.Stop(aclTokenReplicationRoutineName)
}

func (s *Server) startConfigReplication() {
	if s.config.PrimaryDatacenter == "" || s.config.PrimaryDatacenter == s.config.Datacenter {
		// replication shouldn't run in the primary DC
		return
	}

	s.leaderRoutineManager.Start(configReplicationRoutineName, s.configReplicator.Run)
}

func (s *Server) stopConfigReplication() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(configReplicationRoutineName)
}

func (s *Server) startFederationStateReplication() {
	if s.config.PrimaryDatacenter == "" || s.config.PrimaryDatacenter == s.config.Datacenter {
		// replication shouldn't run in the primary DC
		return
	}

	s.leaderRoutineManager.Start(federationStateReplicationRoutineName, s.federationStateReplicator.Run)
}

func (s *Server) stopFederationStateReplication() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(federationStateReplicationRoutineName)
}

// getOrCreateAutopilotConfig is used to get the autopilot config, initializing it if necessary
func (s *Server) getOrCreateAutopilotConfig() *autopilot.Config {
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

	if !ServersMeetMinimumVersion(s.LANMembers(), minAutopilotVersion) {
		logger.Warn("can't initialize until all servers are >= " + minAutopilotVersion.String())
		return nil
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

	if !ServersMeetMinimumVersion(s.LANMembers(), minCentralizedConfigVersion) {
		s.loggers.
			Named(logging.CentralConfig).
			Warn("config: can't initialize until all servers >=" + minCentralizedConfigVersion.String())
		return nil
	}

	state := s.fsm.State()
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

			resp, err := s.raftApply(structs.ConfigEntryRequestType, &req)
			if err == nil {
				if respErr, ok := resp.(error); ok {
					err = respErr
				}
			}
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
func (s *Server) reconcileReaped(known map[string]struct{}) error {
	state := s.fsm.State()
	_, checks, err := state.ChecksInState(nil, api.HealthAny, structs.DefaultEnterpriseMeta())
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
		_, services, err := state.NodeServices(nil, check.Node, structs.DefaultEnterpriseMeta())
		if err != nil {
			return err
		}
		serverPort := 0
		serverAddr := ""
		serverID := ""

	CHECKS:
		for _, service := range services.Services {
			if service.ID == structs.ConsulServiceID {
				_, node, err := state.GetNode(check.Node)
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

		// Create the appropriate tags if this was a server node
		if serverPort > 0 {
			member.Tags["role"] = "consul"
			member.Tags["port"] = strconv.FormatUint(uint64(serverPort), 10)
			member.Tags["id"] = serverID
			member.Addr = net.ParseIP(serverAddr)
		}

		// Attempt to reap this member
		if err := s.handleReapMember(member); err != nil {
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
		s.logger.Warn("skipping reconcile of node", "member", member)
		return nil
	}
	defer metrics.MeasureSince([]string{"leader", "reconcileMember"}, time.Now())
	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.handleAliveMember(member)
	case serf.StatusFailed:
		err = s.handleFailedMember(member)
	case serf.StatusLeft:
		err = s.handleLeftMember(member)
	case StatusReap:
		err = s.handleReapMember(member)
	}
	if err != nil {
		s.logger.Error("failed to reconcile member",
			"member", member,
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
func (s *Server) handleAliveMember(member serf.Member) error {
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
			Meta: map[string]string{
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
	_, node, err := state.GetNode(member.Name)
	if err != nil {
		return err
	}
	if node != nil && node.Address == member.Addr.String() {
		// Check if the associated service is available
		if service != nil {
			match := false
			_, services, err := state.NodeServices(nil, member.Name, structs.DefaultEnterpriseMeta())
			if err != nil {
				return err
			}
			if services != nil {
				for id := range services.Services {
					if id == service.ID {
						match = true
					}
				}
			}
			if !match {
				goto AFTER_CHECK
			}
		}

		// Check if the serfCheck is in the passing state
		_, checks, err := state.NodeChecks(nil, member.Name, structs.DefaultEnterpriseMeta())
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
	s.logger.Info("member joined, marking health alive", "member", member.Name)

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
func (s *Server) handleFailedMember(member serf.Member) error {
	// Check if the node exists
	state := s.fsm.State()
	_, node, err := state.GetNode(member.Name)
	if err != nil {
		return err
	}

	if node == nil {
		s.logger.Info("ignoring failed event for member because it does not exist in the catalog", "member", member.Name)
		return nil
	}

	if node.Address == member.Addr.String() {
		// Check if the serfCheck is in the critical state
		_, checks, err := state.NodeChecks(nil, member.Name, structs.DefaultEnterpriseMeta())
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.CheckID == structs.SerfCheckID && check.Status == api.HealthCritical {
				return nil
			}
		}
	}
	s.logger.Info("member failed, marking health critical", "member", member.Name)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
		ID:         types.NodeID(member.Tags["id"]),
		Address:    member.Addr.String(),
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
func (s *Server) handleLeftMember(member serf.Member) error {
	return s.handleDeregisterMember("left", member)
}

// handleReapMember is used to handle members that have been
// reaped after a prolonged failure. They are deregistered.
func (s *Server) handleReapMember(member serf.Member) error {
	return s.handleDeregisterMember("reaped", member)
}

// handleDeregisterMember is used to deregister a member of a given reason
func (s *Server) handleDeregisterMember(reason string, member serf.Member) error {
	// Do not deregister ourself. This can only happen if the current leader
	// is leaving. Instead, we should allow a follower to take-over and
	// deregister us later.
	if member.Name == s.config.NodeName {
		s.logger.Warn("deregistering self should be done by follower", "name", s.config.NodeName)
		return nil
	}

	// Remove from Raft peers if this was a server
	if valid, parts := metadata.IsConsulServer(member); valid {
		if err := s.removeConsulServer(member, parts.Port); err != nil {
			return err
		}
	}

	// Check if the node does not exist
	state := s.fsm.State()
	_, node, err := state.GetNode(member.Name)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}

	// Deregister the node
	s.logger.Info("deregistering member", "member", member.Name, "reason", reason)
	req := structs.DeregisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
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

	// Processing ourselves could result in trying to remove ourselves to
	// fix up our address, which would make us step down. This is only
	// safe to attempt if there are multiple servers available.
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error("failed to get raft configuration", "error", err)
		return err
	}
	if m.Name == s.config.NodeName {
		if l := len(configFuture.Configuration().Servers); l < 3 {
			s.logger.Debug("Skipping self join check for node since the cluster is too small", "node", m.Name)
			return nil
		}
	}

	// See if it's already in the configuration. It's harmless to re-add it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries. If the address is the same but the ID changed, remove the
	// old server before adding the new one.
	addr := (&net.TCPAddr{IP: m.Addr, Port: parts.Port}).String()
	minRaftProtocol, err := s.autopilot.MinRaftProtocol()
	if err != nil {
		return err
	}
	for _, server := range configFuture.Configuration().Servers {
		// No-op if the raft version is too low
		if server.Address == raft.ServerAddress(addr) && (minRaftProtocol < 2 || parts.RaftVersion < 3) {
			return nil
		}

		// If the address or ID matches an existing server, see if we need to remove the old one first
		if server.Address == raft.ServerAddress(addr) || server.ID == raft.ServerID(parts.ID) {
			// Exit with no-op if this is being called on an existing server
			if server.Address == raft.ServerAddress(addr) && server.ID == raft.ServerID(parts.ID) {
				return nil
			}
			future := s.raft.RemoveServer(server.ID, 0, 0)
			if server.Address == raft.ServerAddress(addr) {
				if err := future.Error(); err != nil {
					return fmt.Errorf("error removing server with duplicate address %q: %s", server.Address, err)
				}
				s.logger.Info("removed server with duplicate address", "address", server.Address)
			} else {
				if err := future.Error(); err != nil {
					return fmt.Errorf("error removing server with duplicate ID %q: %s", server.ID, err)
				}
				s.logger.Info("removed server with duplicate ID", "id", server.ID)
			}
		}
	}

	// Attempt to add as a peer
	switch {
	case minRaftProtocol >= 3:
		addFuture := s.raft.AddNonvoter(raft.ServerID(parts.ID), raft.ServerAddress(addr), 0, 0)
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	case minRaftProtocol == 2 && parts.RaftVersion >= 3:
		addFuture := s.raft.AddVoter(raft.ServerID(parts.ID), raft.ServerAddress(addr), 0, 0)
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	default:
		addFuture := s.raft.AddPeer(raft.ServerAddress(addr))
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	}

	// Trigger a check to remove dead servers
	s.autopilot.RemoveDeadServers()

	return nil
}

// removeConsulServer is used to try to remove a consul server that has left
func (s *Server) removeConsulServer(m serf.Member, port int) error {
	addr := (&net.TCPAddr{IP: m.Addr, Port: port}).String()

	// See if it's already in the configuration. It's harmless to re-remove it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries.
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error("failed to get raft configuration", "error", err)
		return err
	}

	minRaftProtocol, err := s.autopilot.MinRaftProtocol()
	if err != nil {
		return err
	}

	_, parts := metadata.IsConsulServer(m)

	// Pick which remove API to use based on how the server was added.
	for _, server := range configFuture.Configuration().Servers {
		// If we understand the new add/remove APIs and the server was added by ID, use the new remove API
		if minRaftProtocol >= 2 && server.ID == raft.ServerID(parts.ID) {
			s.logger.Info("removing server by ID", "id", server.ID)
			future := s.raft.RemoveServer(raft.ServerID(parts.ID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove raft peer",
					"id", server.ID,
					"error", err,
				)
				return err
			}
			break
		} else if server.Address == raft.ServerAddress(addr) {
			// If not, use the old remove API
			s.logger.Info("removing server by address", "address", server.Address)
			future := s.raft.RemovePeer(raft.ServerAddress(addr))
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove raft peer",
					"address", addr,
					"error", err,
				)
				return err
			}
			break
		}
	}

	return nil
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
