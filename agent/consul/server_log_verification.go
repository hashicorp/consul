package consul

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-wal/verifier"

	"github.com/hashicorp/consul/agent/structs"
)

var _ verifier.IsCheckpointFn = isLogVerifyCheckpoint

// isLogVerifyCheckpoint is the verifier.IsCheckpointFn that can decode our raft logs for
// their type.
func isLogVerifyCheckpoint(l *raft.Log) (bool, error) {
	if len(l.Data) < 1 {
		// Shouldn't be possible! But no need to make it an error if it wasn't one
		// before.
		return false, nil
	}
	// Allow for the "ignore missing" bit to be set.
	typ := structs.MessageType(l.Data[0])
	if typ&structs.IgnoreUnknownTypeFlag == structs.IgnoreUnknownTypeFlag {
		typ &= ^structs.IgnoreUnknownTypeFlag
	}
	return typ == structs.RaftLogVerifierCheckpoint, nil
}

func makeLogVerifyReportFn(logger hclog.Logger) verifier.ReportFn {
	return func(r verifier.VerificationReport) {
		if r.SkippedRange != nil {
			logger.Warn("verification skipped range, consider decreasing validation interval if this is frequent",
				"rangeStart", int64(r.SkippedRange.Start),
				"rangeEnd", int64(r.SkippedRange.End),
			)
		}

		l2 := logger.With(
			"rangeStart", int64(r.Range.Start),
			"rangeEnd", int64(r.Range.End),
			"leaderChecksum", fmt.Sprintf("%08x", r.ExpectedSum),
			"elapsed", r.Elapsed,
		)

		if r.Err == nil {
			l2.Info("verification checksum OK",
				"readChecksum", fmt.Sprintf("%08x", r.ReadSum),
			)
			return
		}

		if r.Err == verifier.ErrRangeMismatch {
			l2.Warn("verification checksum skipped as we don't have all logs in range")
			return
		}

		var csErr verifier.ErrChecksumMismatch
		if errors.As(r.Err, &csErr) {
			if r.WrittenSum > 0 && r.WrittenSum != r.ExpectedSum {
				// The failure occurred before the follower wrote to the log so it
				// must be corrupted in flight from the leader!
				l2.Error("verification checksum FAILED: in-flight corruption",
					"followerWriteChecksum", fmt.Sprintf("%08x", r.WrittenSum),
					"readChecksum", fmt.Sprintf("%08x", r.ReadSum),
				)
			} else {
				l2.Error("verification checksum FAILED: storage corruption",
					"followerWriteChecksum", fmt.Sprintf("%08x", r.WrittenSum),
					"readChecksum", fmt.Sprintf("%08x", r.ReadSum),
				)
			}
			return
		}

		// Some other unknown error occurred
		l2.Error(r.Err.Error())
	}
}
