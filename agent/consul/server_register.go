package consul

import "github.com/hashicorp/consul/logging"

func init() {
	registerEndpoint(func(s *Server) interface{} { return &ACL{s, s.loggers.Named(logging.ACL)} })
	registerEndpoint(func(s *Server) interface{} { return &Catalog{s, s.loggers.Named(logging.Catalog)} })
	registerEndpoint(func(s *Server) interface{} { return NewCoordinate(s, s.logger) })
	registerEndpoint(func(s *Server) interface{} { return &ConfigEntry{s, s.loggers.Named(logging.ConfigEntry)} })
	registerEndpoint(func(s *Server) interface{} { return &ConnectCA{srv: s, logger: s.loggers.Named(logging.Connect)} })
	registerEndpoint(func(s *Server) interface{} { return &FederationState{s} })
	registerEndpoint(func(s *Server) interface{} { return &DiscoveryChain{s} })
	registerEndpoint(func(s *Server) interface{} { return &Health{s, s.loggers.Named(logging.Health)} })
	registerEndpoint(func(s *Server) interface{} { return &Intention{s, s.loggers.Named(logging.Intentions)} })
	registerEndpoint(func(s *Server) interface{} { return &Internal{s, s.loggers.Named(logging.Internal)} })
	registerEndpoint(func(s *Server) interface{} { return &KVS{s, s.loggers.Named(logging.KV)} })
	registerEndpoint(func(s *Server) interface{} { return &Operator{s, s.loggers.Named(logging.Operator)} })
	registerEndpoint(func(s *Server) interface{} { return &PreparedQuery{s, s.loggers.Named(logging.PreparedQuery)} })
	registerEndpoint(func(s *Server) interface{} { return &Session{s, s.loggers.Named(logging.Session)} })
	registerEndpoint(func(s *Server) interface{} { return &Status{s} })
	registerEndpoint(func(s *Server) interface{} { return &Txn{s, s.loggers.Named(logging.Transaction)} })
}
