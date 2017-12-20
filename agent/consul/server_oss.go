package consul

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
