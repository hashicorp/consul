package consul

import (
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"net"
	"time"
)

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.localJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave:
			case serf.EventMemberFailed:
			case serf.EventUser:
			default:
				s.logger.Printf("[WARN] Unhandled LAN Serf Event: %#v", e)
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
				s.logger.Printf("[WARN] Unhandled LAN Serf Event: %#v", e)
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// localJoin is used to handle join events on the lan serf cluster
func (s *Server) localJoin(me serf.MemberEvent) {
	// Check for consul members
	for _, m := range me.Members {
		ok, dc, port := isConsulServer(m)
		if !ok {
			continue
		}
		if dc != s.config.Datacenter {
			s.logger.Printf("[WARN] Consul server %s for datacenter %s has joined wrong cluster",
				m.Name, dc)
			continue
		}
		go s.joinConsulServer(m, port)
	}
}

// remoteJoin is used to handle join events on the wan serf cluster
func (s *Server) remoteJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, dc, port := isConsulServer(m)
		if !ok {
			s.logger.Printf("[WARN] Non-Consul server in WAN pool: %s %s", m.Name)
			continue
		}
		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: port}
		s.logger.Printf("[INFO] Adding Consul server (Datacenter: %s) (Addr: %s)", dc, addr)

		// Check if this server is known
		found := false
		s.remoteLock.Lock()
		existing := s.remoteConsuls[dc]
		for _, e := range existing {
			if e.String() == addr.String() {
				found = true
				break
			}
		}

		// Add ot the list if not known
		if !found {
			s.remoteConsuls[dc] = append(existing, addr)
		}
		s.remoteLock.Unlock()
	}
}

// remoteFailed is used to handle fail events on the wan serf cluster
func (s *Server) remoteFailed(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, dc, port := isConsulServer(m)
		if !ok {
			continue
		}
		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: port}
		s.logger.Printf("[INFO] Removing Consul server (Datacenter: %s) (Addr: %s)", dc, addr)

		// Remove the server if known
		s.remoteLock.Lock()
		existing := s.remoteConsuls[dc]
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
			delete(s.remoteConsuls, dc)
		} else {
			s.remoteConsuls[dc] = existing
		}
		s.remoteLock.Unlock()
	}
}

// joinConsulServer is used to try to join another consul server
func (s *Server) joinConsulServer(m serf.Member, port int) {
	if m.Name == s.config.NodeName {
		return
	}
	var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: port}
	var future raft.Future

CHECK:
	// Get the Raft peers
	peers, err := s.raftPeers.Peers()
	if err != nil {
		s.logger.Printf("[ERR] Failed to get raft peers: %v", err)
		goto WAIT
	}

	// Bail if this node is already a peer
	for _, p := range peers {
		if p.String() == addr.String() {
			return
		}
	}

	// Bail if the node is not alive
	if memberStatus(s.serfLAN.Members(), m.Name) != serf.StatusAlive {
		return
	}

	// Attempt to add as a peer
	future = s.raft.AddPeer(addr)
	if err := future.Error(); err != nil {
		s.logger.Printf("[ERR] Failed to add raft peer: %v", err)
	} else {
		return
	}

WAIT:
	time.Sleep(500 * time.Millisecond)
	select {
	case <-s.shutdownCh:
		return
	default:
		goto CHECK
	}
}

// memberStatus scans a list of members for a matching one,
// returning the status or StatusNone
func memberStatus(members []serf.Member, name string) serf.MemberStatus {
	for _, m := range members {
		if m.Name == name {
			return m.Status
		}
	}
	return serf.StatusNone
}
