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
