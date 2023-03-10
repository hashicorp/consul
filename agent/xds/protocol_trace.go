package xds

import (
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/mitchellh/copystructure"
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

	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	out, err := m.MarshalToString(pb)
	if err != nil {
		out = "<ERROR: " + err.Error() + ">"
	}

	s.Logger.Trace(msg, "direction", dir, "protobuf", out)
}
