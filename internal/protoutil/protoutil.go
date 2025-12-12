// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package protoutil

import (
	"google.golang.org/protobuf/proto"
)

func Clone[T proto.Message](v T) T {
	return proto.Clone(v).(T)
}

func CloneSlice[T proto.Message](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, Clone[T](v))
	}
	return out
}
