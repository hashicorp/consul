package consul

import (
	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"net"
	"strconv"
	"time"
)

const (
	SerfCheckID       = "serfHealth"
	SerfCheckName     = "Serf Health Status"
	ConsulServiceID   = "consul"
	ConsulServiceName = "consul"
	newLeaderEvent    = "consul:new-leader"
)

// monitorLeadership is used to monitor if we acquire or lose our role
// as the leader in the Raft cluster. There is some work the leader is
// expected to do, so we must react to changes
func (s *Server) monitorLeadership() {
	leaderCh := s.raft.LeaderCh()
	var stopCh chan struct{}
	for {
		select {
		case isLeader := <-leaderCh:
			if isLeader {
				stopCh = make(chan struct{})
				go s.leaderLoop(stopCh)
				s.logger.Printf("[INFO] consul: cluster leadership acquired")
			} else if stopCh != nil {
				close(stopCh)
				stopCh = nil
				s.logger.Printf("[INFO] consul: cluster leadership lost")
			}
		case <-s.shutdownCh:
			return
		}
	}
}

// leaderLoop runs as long as we are the leader to run various
// maintence activities
func (s *Server) leaderLoop(stopCh chan struct{}) {
	// Fire a user event indicating a new leader
	payload := []byte(s.config.NodeName)
	if err := s.serfLAN.UserEvent(newLeaderEvent, payload, false); err != nil {
		s.logger.Printf("[WARN] consul: failed to broadcast new leader event: %v", err)
	}

	// Reconcile channel is only used once initial reconcile
	// has succeeded
	var reconcileCh chan serf.Member

RECONCILE:
	// Setup a reconciliation timer
	reconcileCh = nil
	interval := time.After(s.config.ReconcileInterval)

	// Apply a raft barrier to ensure our FSM is caught up
	start := time.Now()
	barrier := s.raft.Barrier(0)
	if err := barrier.Error(); err != nil {
		s.logger.Printf("[ERR] consul: failed to wait for barrier: %v", err)
		goto WAIT
	}
	metrics.MeasureSince([]string{"consul", "leader", "barrier"}, start)

	// Reconcile any missing data
	if err := s.reconcile(); err != nil {
		s.logger.Printf("[ERR] consul: failed to reconcile: %v", err)
		goto WAIT
	}

	// Initial reconcile worked, now we can process the channel
	// updates
	reconcileCh = s.reconcileCh

WAIT:
	// Periodically reconcile as long as we are the leader,
	// or when Serf events arrive
	for {
		select {
		case <-stopCh:
			return
		case <-s.shutdownCh:
			return
		case <-interval:
			goto RECONCILE
		case member := <-reconcileCh:
			s.reconcileMember(member)
		}
	}
}

// reconcile is used to reconcile the differences between Serf
// membership and what is reflected in our strongly consistent store.
// Mainly we need to ensure all live nodes are registered, all failed
// nodes are marked as such, and all left nodes are de-registered.
func (s *Server) reconcile() (err error) {
	defer metrics.MeasureSince([]string{"consul", "leader", "reconcile"}, time.Now())
	members := s.serfLAN.Members()
	knownMembers := make(map[string]struct{})
	for _, member := range members {
		if err := s.reconcileMember(member); err != nil {
			return err
		}
		knownMembers[member.Name] = struct{}{}
	}

	// Reconcile any members that have been reaped while we were not the leader
	return s.reconcileReaped(knownMembers)
}

// reconcileReaped is used to reconcile nodes that have failed and been reaped
// from Serf but remain in the catalog. This is done by looking for SerfCheckID
// in a crticial state that does not correspond to a known Serf member. We generate
// a "reap" event to cause the node to be cleaned up.
func (s *Server) reconcileReaped(known map[string]struct{}) error {
	state := s.fsm.State()
	_, critical := state.ChecksInState(structs.HealthCritical)
	for _, check := range critical {
		// Ignore any non serf checks
		if check.CheckID != SerfCheckID {
			continue
		}

		// Check if this node is "known" by serf
		if _, ok := known[check.Node]; ok {
			continue
		}

		// Create a fake member
		member := serf.Member{
			Name: check.Node,
			Tags: map[string]string{
				"dc":   s.config.Datacenter,
				"role": "node",
			},
		}

		// Get the node services, look for ConsulServiceID
		_, services := state.NodeServices(check.Node)
		serverPort := 0
		for _, service := range services.Services {
			if service.ID == ConsulServiceID {
				serverPort = service.Port
				break
			}
		}

		// Create the appropriate tags if this was a server node
		if serverPort > 0 {
			member.Tags["role"] = "consul"
			member.Tags["port"] = strconv.FormatUint(uint64(serverPort), 10)
		}

		// Attempt to reap this member
		if err := s.handleReapMember(member); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMember is used to do an async reconcile of a single
// serf member
func (s *Server) reconcileMember(member serf.Member) error {
	// Check if this is a member we should handle
	if !s.shouldHandleMember(member) {
		s.logger.Printf("[WARN] consul: skipping reconcile of node %v", member)
		return nil
	}
	defer metrics.MeasureSince([]string{"consul", "leader", "reconcileMember"}, time.Now())
	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.handleAliveMember(member)
	case serf.StatusFailed:
		err = s.handleFailedMember(member)
	case serf.StatusLeft:
		err = s.handleLeftMember(member)
	case StatusReap:
		err = s.handleReapMember(member)
	}
	if err != nil {
		s.logger.Printf("[ERR] consul: failed to reconcile member: %v: %v",
			member, err)
		return err
	}
	return nil
}

// shouldHandleMember checks if this is a Consul pool member
func (s *Server) shouldHandleMember(member serf.Member) bool {
	if valid, dc := isConsulNode(member); valid && dc == s.config.Datacenter {
		return true
	}
	if valid, parts := isConsulServer(member); valid && parts.Datacenter == s.config.Datacenter {
		return true
	}
	return false
}

// handleAliveMember is used to ensure the node
// is registered, with a passing health check.
func (s *Server) handleAliveMember(member serf.Member) error {
	state := s.fsm.State()

	// Register consul service if a server
	var service *structs.NodeService
	if valid, parts := isConsulServer(member); valid {
		service = &structs.NodeService{
			ID:      ConsulServiceID,
			Service: ConsulServiceName,
			Port:    parts.Port,
		}

		// Attempt to join the consul server
		if err := s.joinConsulServer(member, parts); err != nil {
			return err
		}
	}

	// Check if the node exists
	_, found, addr := state.GetNode(member.Name)
	if found && addr == member.Addr.String() {
		// Check if the associated service is available
		if service != nil {
			match := false
			_, services := state.NodeServices(member.Name)
			if services != nil {
				for id := range services.Services {
					if id == service.ID {
						match = true
					}
				}
			}
			if !match {
				goto AFTER_CHECK
			}
		}

		// Check if the serfCheck is in the passing state
		_, checks := state.NodeChecks(member.Name)
		for _, check := range checks {
			if check.CheckID == SerfCheckID && check.Status == structs.HealthPassing {
				return nil
			}
		}
	}
AFTER_CHECK:
	s.logger.Printf("[INFO] consul: member '%s' joined, marking health alive", member.Name)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
		Address:    member.Addr.String(),
		Service:    service,
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: SerfCheckID,
			Name:    SerfCheckName,
			Status:  structs.HealthPassing,
		},
	}
	var out struct{}
	return s.endpoints.Catalog.Register(&req, &out)
}

// handleFailedMember is used to mark the node's status
// as being critical, along with all checks as unknown.
func (s *Server) handleFailedMember(member serf.Member) error {
	state := s.fsm.State()

	// Check if the node exists
	_, found, addr := state.GetNode(member.Name)
	if found && addr == member.Addr.String() {
		// Check if the serfCheck is in the critical state
		_, checks := state.NodeChecks(member.Name)
		for _, check := range checks {
			if check.CheckID == SerfCheckID && check.Status == structs.HealthCritical {
				return nil
			}
		}
	}
	s.logger.Printf("[INFO] consul: member '%s' failed, marking health critical", member.Name)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
		Address:    member.Addr.String(),
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: SerfCheckID,
			Name:    SerfCheckName,
			Status:  structs.HealthCritical,
		},
	}
	var out struct{}
	return s.endpoints.Catalog.Register(&req, &out)
}

// handleLeftMember is used to handle members that gracefully
// left. They are deregistered if necessary.
func (s *Server) handleLeftMember(member serf.Member) error {
	return s.handleDeregisterMember("left", member)
}

// handleReapMember is used to handle members that have been
// reaped after a prolonged failure. They are deregistered.
func (s *Server) handleReapMember(member serf.Member) error {
	return s.handleDeregisterMember("reaped", member)
}

// handleDeregisterMember is used to deregister a member of a given reason
func (s *Server) handleDeregisterMember(reason string, member serf.Member) error {
	state := s.fsm.State()

	// Check if the node does not exists
	_, found, _ := state.GetNode(member.Name)
	if !found {
		return nil
	}
	s.logger.Printf("[INFO] consul: member '%s' %s, deregistering", member.Name, reason)

	// Remove from Raft peers if this was a server
	if valid, parts := isConsulServer(member); valid {
		if err := s.removeConsulServer(member, parts.Port); err != nil {
			return err
		}
	}

	// Deregister the node
	req := structs.DeregisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
	}
	var out struct{}
	return s.endpoints.Catalog.Deregister(&req, &out)
}

// joinConsulServer is used to try to join another consul server
func (s *Server) joinConsulServer(m serf.Member, parts *serverParts) error {
	// Do not join ourself
	if m.Name == s.config.NodeName {
		return nil
	}

	// Check for possibility of multiple bootstrap nodes
	if parts.Bootstrap {
		members := s.serfLAN.Members()
		for _, member := range members {
			valid, p := isConsulServer(member)
			if valid && member.Name != m.Name && p.Bootstrap {
				s.logger.Printf("[ERR] consul: '%v' and '%v' are both in bootstrap mode. Only one node should be in bootstrap mode, not adding Raft peer.", m.Name, member.Name)
				return nil
			}
		}
	}

	// Attempt to add as a peer
	var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: parts.Port}
	future := s.raft.AddPeer(addr)
	if err := future.Error(); err != nil && err != raft.KnownPeer {
		s.logger.Printf("[ERR] consul: failed to add raft peer: %v", err)
		return err
	}
	return nil
}

// removeConsulServer is used to try to remove a consul server that has left
func (s *Server) removeConsulServer(m serf.Member, port int) error {
	// Do not remove ourself
	if m.Name == s.config.NodeName {
		return nil
	}

	// Attempt to remove as peer
	peer := &net.TCPAddr{IP: m.Addr, Port: port}
	future := s.raft.RemovePeer(peer)
	if err := future.Error(); err != nil && err != raft.UnknownPeer {
		s.logger.Printf("[ERR] consul: failed to remove raft peer '%v': %v",
			peer, err)
		return err
	}
	return nil
}
