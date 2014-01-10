package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
	"time"
)

const (
	serfCheckID   = "serfHealth"
	serfCheckName = "Serf Health Status"
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
RECONCILE:
	// Apply a raft barrier to ensure our FSM is caught up
	barrier := s.raft.Barrier(0)
	if err := barrier.Error(); err != nil {
		s.logger.Printf("[ERR] consul: failed to wait for barrier: %v", err)
		goto WAIT
	}

	// Reconcile any missing data
	if err := s.reconcile(); err != nil {
		s.logger.Printf("[ERR] consul: failed to reconcile: %v", err)
		goto WAIT
	}

WAIT:
	// Periodically reconcile as long as we are the leader
	select {
	case <-time.After(s.config.ReconcileInterval):
		goto RECONCILE
	case <-stopCh:
		return
	case <-s.shutdownCh:
		return
	}
}

// reconcile is used to reconcile the differences between Serf
// membership and what is reflected in our strongly consistent store.
// Mainly we need to ensure all live nodes are registered, all failed
// nodes are marked as such, and all left nodes are de-registered.
func (s *Server) reconcile() (err error) {
	members := s.serfLAN.Members()
	for _, member := range members {
		if err := s.reconcileMember(member); err != nil {
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
		s.logger.Printf("[WARN] Skipping reconcile of node %v", member)
		return nil
	}
	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.handleAliveMember(member)
	case serf.StatusFailed:
		err = s.handleFailedMember(member)
	case serf.StatusLeft:
		err = s.handleLeftMember(member)
	}
	if err != nil {
		s.logger.Printf("[ERR] Failed to reconcile member: %v: %v",
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
	if valid, dc, _ := isConsulServer(member); valid && dc == s.config.Datacenter {
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
	if valid, _, port := isConsulServer(member); valid {
		service = &structs.NodeService{
			Service: "consul",
			Port:    port,
		}
	}

	// Check if the node exists
	found, addr := state.GetNode(member.Name)
	if found && addr == member.Addr.String() && service == nil {
		// Check if the serfCheck is in the passing state
		checks := state.NodeChecks(member.Name)
		for _, check := range checks {
			if check.CheckID == serfCheckID && check.Status == structs.HealthPassing {
				return nil
			}
		}
	}
	s.logger.Printf("[INFO] consul: member '%s' joined, marking health alive", member.Name)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
		Address:    member.Addr.String(),
		Service:    service,
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: serfCheckID,
			Name:    serfCheckName,
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
	found, addr := state.GetNode(member.Name)
	if found && addr == member.Addr.String() {
		// Check if the serfCheck is in the critical state
		checks := state.NodeChecks(member.Name)
		for _, check := range checks {
			if check.CheckID == serfCheckID && check.Status == structs.HealthCritical {
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
			CheckID: serfCheckID,
			Name:    serfCheckName,
			Status:  structs.HealthCritical,
		},
	}
	var out struct{}
	return s.endpoints.Catalog.Register(&req, &out)
}

// handleLeftMember is used to handle members that gracefully
// left. They are deregistered if necessary.
func (s *Server) handleLeftMember(member serf.Member) error {
	state := s.fsm.State()

	// Check if the node does not exists
	found, _ := state.GetNode(member.Name)
	if !found {
		return nil
	}
	s.logger.Printf("[INFO] consul: member '%s' left, deregistering", member.Name)

	// Deregister the node
	req := structs.DeregisterRequest{
		Datacenter: s.config.Datacenter,
		Node:       member.Name,
	}
	var out struct{}
	return s.endpoints.Catalog.Deregister(&req, &out)
}
