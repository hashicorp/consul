package structs

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type QueryOptions struct {
	// NOTE: fields omitted from upstream if not necessary for compilation check
	MinQueryIndex uint64
	MaxQueryTime  time.Duration
}

func (q QueryOptions) HasTimedOut(start time.Time, rpcHoldTimeout, maxQueryTime, defaultQueryTime time.Duration) (bool, error) {
	// NOTE: body was omitted from upstream; we only need the signature to verify it compiles
	return false, nil
}

type RPCInfo interface {
	// NOTE: methods omitted from upstream if not necessary for compilation check
}

type QueryBackend int

const (
	QueryBackendBlocking QueryBackend = iota
	QueryBackendStreaming
)

func DurationToProto(d time.Duration) *duration.Duration {
	return ptypes.DurationProto(d)
}

func DurationFromProto(d *duration.Duration) time.Duration {
	ret, _ := ptypes.Duration(d)
	return ret

}

func TimeFromProto(s *timestamp.Timestamp) time.Time {
	ret, _ := ptypes.Timestamp(s)
	return ret
}

func TimeToProto(s time.Time) *timestamp.Timestamp {
	ret, _ := ptypes.TimestampProto(s)
	return ret
}

// IsZeroProtoTime returns true if the time is the minimum protobuf timestamp
// (the Unix epoch).
func IsZeroProtoTime(t *timestamp.Timestamp) bool {
	return t.Seconds == 0 && t.Nanos == 0
}
