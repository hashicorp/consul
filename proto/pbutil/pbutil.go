package pbutil

import (
	"time"

	"github.com/gogo/protobuf/types"
)

func DurationToProto(d time.Duration) *types.Duration {
	return types.DurationProto(d)
}

func DurationFromProto(d *types.Duration) (time.Duration, error) {
	return types.DurationFromProto(d)
}

func TimeFromProto(s *types.Timestamp) (time.Time, error) {
	return types.TimestampFromProto(s)
}

func TimeToProto(s time.Time) (*types.Timestamp, error) {
	return types.TimestampProto(s)
}
