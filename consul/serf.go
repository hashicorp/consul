package consul

import (
	"github.com/hashicorp/serf/serf"
	"net"
)

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			switch e.EventType() {
			case serf.EventMemberJoin:
				fallthrough
			case serf.EventMemberLeave:
				fallthrough
			case serf.EventMemberFailed:
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventUser:
			default:
				s.logger.Printf("[WARN] consul: unhandled LAN Serf Event: %#v", e)
			}

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
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.remoteJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave:
				fallthrough
			case serf.EventMemberFailed:
				s.remoteFailed(e.(serf.MemberEvent))
			case serf.EventUser:
			default:
				s.logger.Printf("[WARN] consul: unhandled WAN Serf Event: %#v", e)
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// localMemberEvent is used to reconcile Serf events with the strongly
// consistent store if we are the current leader
func (s *Server) localMemberEvent(me serf.MemberEvent) {
	// Do nothing if we are not the leader
	if !s.IsLeader() {
		return
	}

	// Queue the members for reconciliation
	for _, m := range me.Members {
		select {
		case s.reconcileCh <- m:
		default:
		}
	}
}

// remoteJoin is used to handle join events on the wan serf cluster
func (s *Server) remoteJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			s.logger.Printf("[WARN] consul: non-server in WAN pool: %s %s", m.Name)
			continue
		}
		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: parts.Port}
		s.logger.Printf("[INFO] consul: adding server for datacenter: %s, addr: %s", parts.Datacenter, addr)

		// Check if this server is known
		found := false
		s.remoteLock.Lock()
		existing := s.remoteConsuls[parts.Datacenter]
		for _, e := range existing {
			if e.String() == addr.String() {
				found = true
				break
			}
		}

		// Add ot the list if not known
		if !found {
			s.remoteConsuls[parts.Datacenter] = append(existing, addr)
		}
		s.remoteLock.Unlock()
	}
}

// remoteFailed is used to handle fail events on the wan serf cluster
func (s *Server) remoteFailed(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			continue
		}
		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: parts.Port}
		s.logger.Printf("[INFO] consul: removing server for datacenter: %s, addr: %s", parts.Datacenter, addr)

		// Remove the server if known
		s.remoteLock.Lock()
		existing := s.remoteConsuls[parts.Datacenter]
		n := len(existing)
		for i := 0; i < n; i++ {
			if existing[i].String() == addr.String() {
				existing[i], existing[n-1] = existing[n-1], nil
				existing = existing[:n-1]
				n--
				break
			}
		}

		// Trim the list if all known consuls are dead
		if n == 0 {
			delete(s.remoteConsuls, parts.Datacenter)
		} else {
			s.remoteConsuls[parts.Datacenter] = existing
		}
		s.remoteLock.Unlock()
	}
}
