// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/protoutil"
)

// Deprecated: see protoutil.Clone
func protoClone[T proto.Message](v T) T {
	return protoutil.Clone(v)
}

// Deprecated: see protoutil.CloneSlice
func protoSliceClone[T proto.Message](in []T) []T {
	return protoutil.CloneSlice(in)
}
