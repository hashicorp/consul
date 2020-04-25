package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// maxInvalidateAttempts limits how many invalidate attempts are made
	maxInvalidateAttempts = 6

	// invalidateRetryBase is a baseline retry time
	invalidateRetryBase = 10 * time.Second
)

// initializeSessionTimers is used when a leader is newly elected to create
// a new map to track session expiration and to reset all the timers from
// the previously known set of timers.
func (s *Server) initializeSessionTimers() error {
	// Scan all sessions and reset their timer
	state := s.fsm.State()

	_, sessions, err := state.SessionList(nil, structs.WildcardEnterpriseMeta())
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err := s.resetSessionTimer(session.ID, session); err != nil {
			return err
		}
	}
	return nil
}

// resetSessionTimer is used to renew the TTL of a session.
// This can be used for new sessions and existing ones. A session
// will be faulted in if not given.
func (s *Server) resetSessionTimer(id string, session *structs.Session) error {
	// Fault the session in if not given
	if session == nil {
		state := s.fsm.State()
		_, s, err := state.SessionGet(nil, id, nil)
		if err != nil {
			return err
		}
		if s == nil {
			return fmt.Errorf("Session '%s' not found", id)
		}
		session = s
	}

	// Bail if the session has no TTL, fast-path some common inputs
	switch session.TTL {
	case "", "0", "0s", "0m", "0h":
		return nil
	}

	// Parse the TTL, and skip if zero time
	ttl, err := time.ParseDuration(session.TTL)
	if err != nil {
		return fmt.Errorf("Invalid Session TTL '%s': %v", session.TTL, err)
	}
	if ttl == 0 {
		return nil
	}

	s.createSessionTimer(session.ID, ttl, &session.EnterpriseMeta)
	return nil
}

func (s *Server) createSessionTimer(id string, ttl time.Duration, entMeta *structs.EnterpriseMeta) {
	// Reset the session timer
	// Adjust the given TTL by the TTL multiplier. This is done
	// to give a client a grace period and to compensate for network
	// and processing delays. The contract is that a session is not expired
	// before the TTL, but there is no explicit promise about the upper
	// bound so this is allowable.
	ttl = ttl * structs.SessionTTLMultiplier
	s.sessionTimers.ResetOrCreate(id, ttl, func() { s.invalidateSession(id, entMeta) })
}

// invalidateSession is invoked when a session TTL is reached and we
// need to invalidate the session.
func (s *Server) invalidateSession(id string, entMeta *structs.EnterpriseMeta) {
	defer metrics.MeasureSince([]string{"session_ttl", "invalidate"}, time.Now())

	// Clear the session timer
	s.sessionTimers.Del(id)

	// Create a session destroy request
	args := structs.SessionRequest{
		Datacenter: s.config.Datacenter,
		Op:         structs.SessionDestroy,
		Session: structs.Session{
			ID: id,
		},
	}
	if entMeta != nil {
		args.Session.EnterpriseMeta = *entMeta
	}

	// Retry with exponential backoff to invalidate the session
	for attempt := uint(0); attempt < maxInvalidateAttempts; attempt++ {
		_, err := s.raftApply(structs.SessionRequestType, args)
		if err == nil {
			s.logger.Debug("Session TTL expired", "session", id)
			return
		}

		s.logger.Error("Invalidation failed", "error", err)
		time.Sleep((1 << attempt) * invalidateRetryBase)
	}
	s.logger.Error("maximum revoke attempts reached for session", "error", id)
}

// clearSessionTimer is used to clear the session time for
// a single session. This is used when a session is destroyed
// explicitly and no longer needed.
func (s *Server) clearSessionTimer(id string) error {
	s.sessionTimers.Stop(id)
	return nil
}

// clearAllSessionTimers is used when a leader is stepping
// down and we no longer need to track any session timers.
func (s *Server) clearAllSessionTimers() {
	s.sessionTimers.StopAll()
}

// updateMetrics is a long running routine used to uddate a
// number of server periodic metrics
func (s *Server) updateMetrics() {
	for {
		select {
		case <-time.After(time.Second):
			metrics.SetGauge([]string{"session_ttl", "active"}, float32(s.sessionTimers.Len()))

			metrics.SetGauge([]string{"raft", "applied_index"}, float32(s.raft.AppliedIndex()))
			metrics.SetGauge([]string{"raft", "last_index"}, float32(s.raft.LastIndex()))
		case <-s.shutdownCh:
			return
		}
	}
}
