package connsul

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			s.logger.Printf("[INFO] LAN Event: %v", e)
		case <-s.shutdownCh:
			return
		}
	}
}

// wanEventHandler is used to handle events from the wan Serf cluster
func (s *Server) wanEventHandler() {
	for {
		select {
		case e := <-s.eventChWAN:
			s.logger.Printf("[INFO] WAN Event: %v", e)
		case <-s.shutdownCh:
			return
		}
	}
}
