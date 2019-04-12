package consul

import "google.golang.org/grpc"

func init() {
	registerEndpoint(func(s *Server) interface{} { return &ACL{s} })
	registerEndpoint(func(s *Server) interface{} { return &Catalog{s} })
	registerEndpoint(func(s *Server) interface{} { return NewCoordinate(s) })
	registerEndpoint(func(s *Server) interface{} { return &ConfigEntry{s} })
	registerEndpoint(func(s *Server) interface{} { return &ConnectCA{srv: s} })
	registerEndpoint(func(s *Server) interface{} { return &Health{s} })
	registerGRPCEndpoint(func(s *Server, grpcServer *grpc.Server) {
		RegisterHealthServer(grpcServer, &HealthGRPCAdapter{Health{s}})
	})
	registerEndpoint(func(s *Server) interface{} { return &Intention{s} })
	registerEndpoint(func(s *Server) interface{} { return &Internal{s} })
	registerEndpoint(func(s *Server) interface{} { return &KVS{s} })
	registerEndpoint(func(s *Server) interface{} { return &Operator{s} })
	registerEndpoint(func(s *Server) interface{} { return &PreparedQuery{s} })
	registerEndpoint(func(s *Server) interface{} { return &Session{s} })
	registerEndpoint(func(s *Server) interface{} { return &Status{s} })
	registerEndpoint(func(s *Server) interface{} { return &Txn{s} })
}
