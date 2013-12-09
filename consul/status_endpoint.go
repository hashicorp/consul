package consul

// Status endpoint is used to check on server status
type Status struct {
	server *Server
}

// Ping is used to just check for connectivity
func (s *Status) Ping(args struct{}, reply *struct{}) error {
	return nil
}
