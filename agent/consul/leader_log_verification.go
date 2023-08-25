package consul

import (
	"context"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) startLogVerification(ctx context.Context) error {
	return s.leaderRoutineManager.Start(ctx, raftLogVerifierRoutineName, s.runLogVerification)
}

func (s *Server) stopLogVerification() {
	s.leaderRoutineManager.Stop(raftLogVerifierRoutineName)
}

func (s *Server) runLogVerification(ctx context.Context) error {
	// This shouldn't be possible but bit of a safety check
	if !s.config.LogStoreConfig.Verification.Enabled ||
		s.config.LogStoreConfig.Verification.Interval == 0 {
		return nil
	}
	ticker := time.NewTicker(s.config.LogStoreConfig.Verification.Interval)
	defer ticker.Stop()

	logger := s.logger.Named("raft.logstore.verifier")
	for {
		select {
		case <-ticker.C:
			// Attempt to send a checkpoint message
			typ := structs.RaftLogVerifierCheckpoint | structs.IgnoreUnknownTypeFlag
			raw, err := s.raftApplyMsgpack(typ, nil)
			if err != nil {
				logger.Error("sending verification checkpoint failed", "err", err)
			} else {
				index, ok := raw.(uint64)
				if !ok {
					index = 0
				}
				logger.Debug("sent verification checkpoint", "index", int64(index))
			}

		case <-ctx.Done():
			return nil
		}
	}
}
