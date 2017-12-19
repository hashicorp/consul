package consul

import "github.com/hashicorp/consul/agent/consul/autopilot"

func init() {
	registerEndpoint(func(s *Server) interface{} { return &ACL{s} })
	registerEndpoint(func(s *Server) interface{} { return &Catalog{s} })
	registerEndpoint(func(s *Server) interface{} { return NewCoordinate(s) })
	registerEndpoint(func(s *Server) interface{} { return &Health{s} })
	registerEndpoint(func(s *Server) interface{} { return &Internal{s} })
	registerEndpoint(func(s *Server) interface{} { return &KVS{s} })
	registerEndpoint(func(s *Server) interface{} { return &Operator{s} })
	registerEndpoint(func(s *Server) interface{} { return &PreparedQuery{s} })
	registerEndpoint(func(s *Server) interface{} { return &Session{s} })
	registerEndpoint(func(s *Server) interface{} { return &Status{s} })
	registerEndpoint(func(s *Server) interface{} { return &Txn{s} })
}

func (s *Server) startServerEnterprise(config *Config) {
	// Set up autopilot
	apDelegate := &AutopilotDelegate{s}
	s.autopilot = autopilot.NewAutopilot(s.logger, apDelegate, config.AutopilotInterval, config.ServerHealthInterval)
}