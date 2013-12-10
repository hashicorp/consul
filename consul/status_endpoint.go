package consul

// Status endpoint is used to check on server status
type Status struct {
	server *Server
}

// Ping is used to just check for connectivity
func (s *Status) Ping(args struct{}, reply *struct{}) error {
	return nil
}

// Leader is used to get the address of the leader
func (s *Status) Leader(args struct{}, reply *string) error {
	leader := s.server.raft.Leader()
	if leader != nil {
		*reply = leader.String()
	} else {
		*reply = ""
	}
	return nil
}
