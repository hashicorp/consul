package consul

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"golang.org/x/time/rate"
)

func (s *Server) startACLTokenReaping() {
	s.aclTokenReapLock.Lock()
	defer s.aclTokenReapLock.Unlock()

	if s.aclTokenReapEnabled {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.aclTokenReapCancel = cancel

	// Do a quick check for config settings that would imply the goroutine
	// below will just spin forever.
	//
	// We can only check the config settings here that cannot change without a
	// restart, so we omit the check for a non-empty replication token as that
	// can be changed at runtime.
	if !s.InACLDatacenter() && !s.config.ACLTokenReplication {
		return
	}

	go func() {
		limiter := rate.NewLimiter(aclTokenReapingRateLimit, aclTokenReapingBurst)

		for {
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			if s.LocalTokensEnabled() {
				if _, err := s.reapExpiredLocalACLTokens(); err != nil {
					s.logger.Printf("[ERR] acl: error reaping expired local ACL tokens: %v", err)
				}
			}
			if s.InACLDatacenter() {
				if _, err := s.reapExpiredGlobalACLTokens(); err != nil {
					s.logger.Printf("[ERR] acl: error reaping expired global ACL tokens: %v", err)
				}
			}
		}
	}()

	s.aclTokenReapEnabled = true
}

func (s *Server) stopACLTokenReaping() {
	s.aclTokenReapLock.Lock()
	defer s.aclTokenReapLock.Unlock()

	if !s.aclTokenReapEnabled {
		return
	}

	s.aclTokenReapCancel()
	s.aclTokenReapCancel = nil
	s.aclTokenReapEnabled = false
}

func (s *Server) reapExpiredGlobalACLTokens() (int, error) {
	return s.reapExpiredACLTokens(false, true)
}
func (s *Server) reapExpiredLocalACLTokens() (int, error) {
	return s.reapExpiredACLTokens(true, false)
}
func (s *Server) reapExpiredACLTokens(local, global bool) (int, error) {
	if !s.ACLsEnabled() {
		return 0, nil
	}
	if s.UseLegacyACLs() {
		return 0, nil
	}
	if local == global {
		return 0, fmt.Errorf("cannot reap both local and global tokens in the same request")
	}

	locality := localityName(local)

	minExpiredTime, err := s.fsm.State().ACLTokenMinExpirationTime(local)
	if err != nil {
		return 0, err
	}

	now := time.Now()

	if minExpiredTime.After(now) {
		return 0, nil // nothing to do
	}

	tokens, _, err := s.fsm.State().ACLTokenListExpired(local, now, aclBatchDeleteSize)
	if err != nil {
		return 0, err
	}

	if len(tokens) == 0 {
		return 0, nil
	}

	var (
		secretIDs []string
		req       structs.ACLTokenBatchDeleteRequest
	)
	for _, token := range tokens {
		if token.Local != local {
			return 0, fmt.Errorf("expired index for local=%v returned a mismatched token with local=%v: %s", local, token.Local, token.AccessorID)
		}
		req.TokenIDs = append(req.TokenIDs, token.AccessorID)
		secretIDs = append(secretIDs, token.SecretID)
	}

	s.logger.Printf("[INFO] acl: deleting %d expired %s tokens", len(req.TokenIDs), locality)
	resp, err := s.raftApply(structs.ACLTokenDeleteRequestType, &req)
	if err != nil {
		return 0, fmt.Errorf("Failed to apply token expiration deletions: %v", err)
	}

	// Purge the identities from the cache
	for _, secretID := range secretIDs {
		s.acls.cache.RemoveIdentity(secretID)
	}

	if respErr, ok := resp.(error); ok {
		return 0, respErr
	}

	return len(req.TokenIDs), nil
}

func localityName(local bool) string {
	if local {
		return "local"
	}
	return "global"
}
