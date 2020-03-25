package consul

import "github.com/hashicorp/consul/logging"

// GRPCService is the implementation of the gRPC Consul service defined in
// agentpb/consul.proto. Each RPC is implemented in a separate *_grpc_endpoint
// files as methods on this object.
type GRPCService struct {
	srv *Server

	// gRPC needs each RPC in the service definition attached to a single object
	// as a method to implement the interface. We want to use a separate named
	// logger for each endpit to match net/rpc usage but also would be nice to be
	// able to just use the standard s.logger for calls rather than seperately
	// named loggers for each RPC method. So each RPC method is actually defined
	// on a separate object with a `logger` field and then those are all ebedded
	// here to make this object implement the full interface.
	GRPCSubscribeHandler
}

func NewGRPCService(s *Server) *GRPCService {
	return &GRPCService{
		GRPCSubscribeHandler: GRPCSubscribeHandler{
			srv:    s,
			logger: s.loggers.Named(logging.Subscribe),
		},
	}
}
