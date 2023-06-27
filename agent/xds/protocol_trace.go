// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/mitchellh/copystructure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (s *ResourceGenerator) logTraceRequest(msg string, pb proto.Message) {
	s.logTraceProto(msg, pb, false)
}

func (s *ResourceGenerator) logTraceResponse(msg string, pb proto.Message) {
	s.logTraceProto(msg, pb, true)
}

func (s *ResourceGenerator) logTraceProto(msg string, pb proto.Message, response bool) {
	if !s.Logger.IsTrace() {
		return
	}

	dir := "request"
	if response {
		dir = "response"
	}

	// Duplicate the request so we can scrub the huge Node field for logging.
	// If the cloning fails, then log anyway but don't scrub the node field.
	if dup, err := copystructure.Copy(pb); err == nil {
		pb = dup.(proto.Message)

		// strip the node field
		switch x := pb.(type) {
		case *envoy_discovery_v3.DiscoveryRequest:
			x.Node = nil
		case *envoy_discovery_v3.DeltaDiscoveryRequest:
			x.Node = nil
		}
	}

	m := protojson.MarshalOptions{
		Indent: "  ",
	}

	out := ""
	outBytes, err := m.Marshal(pb)
	if err != nil {
		out = "<ERROR: " + err.Error() + ">"
	} else {
		out = string(outBytes)
	}

	s.Logger.Trace(msg, "direction", dir, "protobuf", out)
}
