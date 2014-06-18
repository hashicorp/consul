package consul

import (
	"net"
	"strings"

	"github.com/hashicorp/serf/serf"
)

const (
	// StatusReap is used to update the status of a node if we
	// are handling a EventMemberReap
	StatusReap = serf.MemberStatus(-1)
)

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.nodeJoin(e.(serf.MemberEvent), false)
				s.localMemberEvent(e.(serf.MemberEvent))

			case serf.EventMemberLeave:
				fallthrough
			case serf.EventMemberFailed:
				s.nodeFailed(e.(serf.MemberEvent), false)
				s.localMemberEvent(e.(serf.MemberEvent))

			case serf.EventMemberReap:
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventUser:
				s.localEvent(e.(serf.UserEvent))
			case serf.EventMemberUpdate: // Ignore
			case serf.EventQuery: // Ignore
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
				s.nodeJoin(e.(serf.MemberEvent), true)
			case serf.EventMemberLeave:
				fallthrough
			case serf.EventMemberFailed:
				s.nodeFailed(e.(serf.MemberEvent), true)
			case serf.EventMemberUpdate: // Ignore
			case serf.EventMemberReap: // Ignore
			case serf.EventUser:
			case serf.EventQuery: // Ignore
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

	// Check if this is a reap event
	isReap := me.EventType() == serf.EventMemberReap

	// Queue the members for reconciliation
	for _, m := range me.Members {
		// Change the status if this is a reap event
		if isReap {
			m.Status = StatusReap
		}
		select {
		case s.reconcileCh <- m:
		default:
		}
	}
}

// localEvent is called when we receive an event on the local Serf
func (s *Server) localEvent(event serf.UserEvent) {
	// Handle only consul events
	if !strings.HasPrefix(event.Name, "consul:") {
		return
	}

	switch event.Name {
	case newLeaderEvent:
		s.logger.Printf("[INFO] consul: New leader elected: %s", event.Payload)

		// Trigger the callback
		if s.config.ServerUp != nil {
			s.config.ServerUp()
		}
	default:
		s.logger.Printf("[WARN] consul: Unhandled local event: %v", event)
	}
}

// nodeJoin is used to handle join events on the both serf clusters
func (s *Server) nodeJoin(me serf.MemberEvent, wan bool) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			if wan {
				s.logger.Printf("[WARN] consul: non-server in WAN pool: %s %s", m.Name)
			}
			continue
		}
		s.logger.Printf("[INFO] consul: adding server %s", parts)

		// Check if this server is known
		found := false
		s.remoteLock.Lock()
		existing := s.remoteConsuls[parts.Datacenter]
		for idx, e := range existing {
			if e.Name == parts.Name {
				existing[idx] = parts
				found = true
				break
			}
		}

		// Add ot the list if not known
		if !found {
			s.remoteConsuls[parts.Datacenter] = append(existing, parts)
		}
		s.remoteLock.Unlock()

		// Add to the local list as well
		if !wan {
			s.localLock.Lock()
			s.localConsuls[parts.Addr.String()] = parts
			s.localLock.Unlock()
		}

		// If we're still expecting, and they are too, check servers.
		if s.config.Expect != 0 && parts.Expect != 0 {
			index, err := s.raftStore.LastIndex()
			if err == nil && index == 0 {
				members := s.serfLAN.Members()
				addrs := make([]net.Addr, 0)
				for _, member := range members {
					valid, p := isConsulServer(member)
					if valid && p.Datacenter == parts.Datacenter {
						if p.Expect != parts.Expect {
							s.logger.Printf("[ERR] consul: '%v' and '%v' have different expect values. All expect nodes should have the same value, will never leave expect mode", m.Name, member.Name)
							return
						} else {
							addrs = append(addrs, &net.TCPAddr{IP: member.Addr, Port: p.Port})
						}
					}
				}

				if len(addrs) >= s.config.Expect {
					// we have enough nodes, set peers.

					future := s.raft.SetPeers(addrs)

					if err := future.Error(); err != nil {
						s.logger.Printf("[ERR] consul: failed to leave expect mode and set peers: %v", err)
					} else {
						// we've left expect mode, don't enter this again
						s.config.Expect = 0
					}
				}
			} else if err != nil {
				s.logger.Printf("[ERR] consul: error retrieving index: %v", err)
			}
		}
	}
}

// nodeFailed is used to handle fail events on both the serf clustes
func (s *Server) nodeFailed(me serf.MemberEvent, wan bool) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			continue
		}
		s.logger.Printf("[INFO] consul: removing server %s", parts)

		// Remove the server if known
		s.remoteLock.Lock()
		existing := s.remoteConsuls[parts.Datacenter]
		n := len(existing)
		for i := 0; i < n; i++ {
			if existing[i].Name == parts.Name {
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

		// Remove from the local list as well
		if !wan {
			s.localLock.Lock()
			delete(s.localConsuls, parts.Addr.String())
			s.localLock.Unlock()
		}
	}
}
