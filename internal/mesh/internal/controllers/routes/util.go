// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/xroutemapper"
)

func protoSliceClone[V any, PV interface {
	proto.Message
	*V
}](in []PV) []PV {
	if in == nil {
		return nil
	}
	out := make([]PV, 0, len(in))
	for _, v := range in {
		out = append(out, proto.Clone(v).(PV))
	}
	return out
}

// Deprecated: xroutemapper.DeduplicateRequests
func deduplicate(reqs []controller.Request) []controller.Request {
	return xroutemapper.DeduplicateRequests(reqs)
}
