// +build !ent

package consul

// initializeCA sets up the CA provider when gaining leadership, bootstrapping
// the root in the state store if necessary.
func (s *Server) initializeCA() error {
	// Bail if connect isn't enabled.
	if !s.config.ConnectEnabled {
		return nil
	}

	conf, err := s.initializeCAConfig()
	if err != nil {
		return err
	}

	// Initialize the provider based on the current config.
	provider, err := s.createCAProvider(conf)
	if err != nil {
		return err
	}

	return s.initializeRootCA(provider, conf)
}

// Stub methods, only present in Consul Enterprise.
func (s *Server) startEnterpriseLeader() {}
func (s *Server) stopEnterpriseLeader()  {}
