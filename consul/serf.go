package consul

import (
	"github.com/hashicorp/serf/serf"
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
				s.localLeave(e.(serf.MemberEvent))
			case serf.EventMemberFailed:
				s.localFailed(e.(serf.MemberEvent))
			case serf.EventUser:
				s.localEvent(e.(serf.UserEvent))
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
				s.remoteLeave(e.(serf.MemberEvent))
			case serf.EventMemberFailed:
				s.remoteFailed(e.(serf.MemberEvent))
			case serf.EventUser:
				s.remoteEvent(e.(serf.UserEvent))
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
}

// localLeave is used to handle leave events on the lan serf cluster
func (s *Server) localLeave(me serf.MemberEvent) {
}

// localFailed is used to handle fail events on the lan serf cluster
func (s *Server) localFailed(me serf.MemberEvent) {
}

// localEvent is used to handle events on the lan serf cluster
func (s *Server) localEvent(ue serf.UserEvent) {
}

// remoteJoin is used to handle join events on the wan serf cluster
func (s *Server) remoteJoin(me serf.MemberEvent) {
}

// remoteLeave is used to handle leave events on the wan serf cluster
func (s *Server) remoteLeave(me serf.MemberEvent) {
}

// remoteFailed is used to handle fail events on the wan serf cluster
func (s *Server) remoteFailed(me serf.MemberEvent) {
}

// remoteEvent is used to handle events on the wan serf cluster
func (s *Server) remoteEvent(ue serf.UserEvent) {
}
