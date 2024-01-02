// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func DurationToProto(d time.Duration) *durationpb.Duration {
	return durationpb.New(d)
}

func DurationFromProto(d *durationpb.Duration) time.Duration {
	return d.AsDuration()
}

func TimeFromProto(s *timestamppb.Timestamp) time.Time {
	return s.AsTime()
}

func TimeToProto(s time.Time) *timestamppb.Timestamp {
	return timestamppb.New(s)
}

// IsZeroProtoTime returns true if the time is the minimum protobuf timestamp
// (the Unix epoch).
func IsZeroProtoTime(t *timestamppb.Timestamp) bool {
	return t.Seconds == 0 && t.Nanos == 0
}
