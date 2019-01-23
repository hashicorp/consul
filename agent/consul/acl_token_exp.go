package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

const maxExpiredTokensPerLoop = 100

func (s *Server) reapExpiredACLTokens(local, global bool, nowFunc func() time.Time) (int, error) {
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

	s.logger.Printf("[INFO] acltokenreaper: scanning for expired %s tokens", locality)
	defer s.logger.Printf("[INFO] acltokenreaper: scanning for expired %s tokens [DONE]", locality)

	now := nowFunc()

	state := s.fsm.State()

	minExpiredTime, err := state.ACLTokenMinExpirationTime(local)
	if err != nil {
		return 0, err
	}

	if minExpiredTime.After(now) {
		return 0, nil // nothing to do
	}

	tokens, _, err := state.ACLTokenListExpired(local, now, maxExpiredTokensPerLoop)
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

	s.logger.Printf("[INFO] acltokenreaper: deleting %d expired %s tokens", len(req.TokenIDs), locality)
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
