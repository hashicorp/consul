// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) reapExpiredTokens(ctx context.Context) error {
	limiter := rate.NewLimiter(aclTokenReapingRateLimit, aclTokenReapingBurst)
	for {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		if s.LocalTokensEnabled() {
			if _, err := s.reapExpiredLocalACLTokens(); err != nil {
				s.logger.Error("error reaping expired local ACL tokens", "error", err)
			}
		}
		if s.InPrimaryDatacenter() {
			if _, err := s.reapExpiredGlobalACLTokens(); err != nil {
				s.logger.Error("error reaping expired global ACL tokens", "error", err)
			}
		}
	}
}

func (s *Server) startACLTokenReaping(ctx context.Context) {
	// Do a quick check for config settings that would imply the goroutine
	// below will just spin forever.
	//
	// We can only check the config settings here that cannot change without a
	// restart, so we omit the check for a non-empty replication token as that
	// can be changed at runtime.
	if !s.InPrimaryDatacenter() && !s.config.ACLTokenReplication {
		return
	}

	s.leaderRoutineManager.Start(ctx, aclTokenReapingRoutineName, s.reapExpiredTokens)
}

func (s *Server) stopACLTokenReaping() {
	s.leaderRoutineManager.Stop(aclTokenReapingRoutineName)
}

func (s *Server) reapExpiredGlobalACLTokens() (int, error) {
	return s.reapExpiredACLTokens(false, true)
}
func (s *Server) reapExpiredLocalACLTokens() (int, error) {
	return s.reapExpiredACLTokens(true, false)
}
func (s *Server) reapExpiredACLTokens(local, global bool) (int, error) {
	if !s.config.ACLsEnabled {
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

	s.logger.Info("deleting expired ACL tokens",
		"amount", len(req.TokenIDs),
		"locality", locality,
	)

	_, err = s.leaderRaftApply("ACL.TokenDelete", structs.ACLTokenDeleteRequestType, &req)
	if err != nil {
		return 0, fmt.Errorf("Failed to apply token expiration deletions: %v", err)
	}

	// Purge the identities from the cache
	for _, secretID := range secretIDs {
		s.ACLResolver.cache.RemoveIdentityWithSecretToken(secretID)
	}

	return len(req.TokenIDs), nil
}

func localityName(local bool) string {
	if local {
		return "local"
	}
	return "global"
}
