package consul

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/consul/consul/agent"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

func (s *Server) startAutopilot() {
	s.autopilotShutdownCh = make(chan struct{})

	go s.serverHealthLoop()
	go s.removeDeadLoop()
}

func (s *Server) stopAutopilot() {
	close(s.autopilotShutdownCh)
}

// serverHealthLoop monitors the health of the servers in the cluster
func (s *Server) serverHealthLoop() {
	// Monitor server health until shutdown
	ticker := time.NewTicker(s.config.ServerHealthInterval)
	for {
		select {
		case <-s.autopilotShutdownCh:
			ticker.Stop()
			return
		case <-ticker.C:
			serverHealths := make(map[string]*structs.ServerHealth)

			state := s.fsm.State()
			_, autopilotConf, err := state.AutopilotConfig()
			if err != nil {
				s.logger.Printf("[ERR] consul: error retrieving autopilot config: %s", err)
			}

			// Build an updated map of server healths
			for _, member := range s.LANMembers() {
				if member.Status == serf.StatusLeft {
					continue
				}

				valid, parts := agent.IsConsulServer(member)
				if valid {
					health := s.queryServerHealth(member, parts, autopilotConf)
					serverHealths[parts.Addr.String()] = health
				}
			}

			s.autopilotLock.Lock()
			s.autopilotHealth = serverHealths
			s.autopilotLock.Unlock()

			if err := s.promoteNonVoters(autopilotConf); err != nil {
				s.logger.Printf("[ERR] consul: error checking for non-voters to promote: %s", err)
			}
		}
	}
}

// removeDeadLoop checks for dead servers periodically, or when receiving on autopilotRemoveDeadCh
func (s *Server) removeDeadLoop() {
	ticker := time.NewTicker(s.config.RemoveDeadInterval)
	for {
		select {
		case <-s.autopilotShutdownCh:
			ticker.Stop()
			return
		case <-ticker.C:
			if err := s.pruneDeadServers(); err != nil {
				s.logger.Printf("[ERR] consul: error checking for dead servers to remove: %s", err)
			}
		case <-s.autopilotRemoveDeadCh:
			if err := s.pruneDeadServers(); err != nil {
				s.logger.Printf("[ERR] consul: error checking for dead servers to remove: %s", err)
			}
		}
	}
}

// pruneDeadServers removes up to numPeers/2 failed servers
func (s *Server) pruneDeadServers() error {
	state := s.fsm.State()
	_, autopilotConf, err := state.AutopilotConfig()
	if err != nil {
		return err
	}

	// Look for dead servers to clean up
	peers, err := s.numPeers()
	if err != nil {
		return err
	}

	removed := 0
	if autopilotConf.CleanupDeadServers {
		for _, member := range s.serfLAN.Members() {
			// Exit early if we already removed the max amount of servers
			if removed == peers/2 {
				break
			}

			valid, _ := agent.IsConsulServer(member)
			if valid && member.Status == serf.StatusFailed {
				removed++
				s.logger.Printf("[INFO] consul: Attempting removal of failed server: %v", member.Name)
				go s.serfLAN.RemoveFailedNode(member.Name)
			}
		}
	}

	return nil
}

// promoteNonVoters promotes eligible non-voting servers to voters.
func (s *Server) promoteNonVoters(autopilotConf *structs.AutopilotConfig) error {
	minRaftProtocol, err := ServerMinRaftProtocol(s.LANMembers())
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}

	if minRaftProtocol >= 3 {
		future := s.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to get raft configuration: %v", err)
		}

		var promotions []raft.Server
		raftServers := future.Configuration().Servers
		voterCount := 0
		for _, server := range raftServers {
			// If this server has been stable and passing for long enough, promote it to a voter
			if server.Suffrage == raft.Nonvoter {
				health := s.getServerHealth(string(server.Address))
				if health != nil && health.Healthy && time.Now().Sub(health.StableSince) >= autopilotConf.ServerStabilizationTime {
					promotions = append(promotions, server)
				}
			} else {
				voterCount++
			}
		}

		// Exit early if there's nothing to promote
		if len(promotions) == 0 {
			return nil
		}

		// If there's currently an even number of servers, we can promote the first server in the list
		// to get to an odd-sized quorum
		newServers := false
		if voterCount%2 == 0 {
			addFuture := s.raft.AddVoter(promotions[0].ID, promotions[0].Address, 0, 0)
			if err := addFuture.Error(); err != nil {
				return fmt.Errorf("failed to add raft peer: %v", err)
			}
			promotions = promotions[1:]
			newServers = true
		}

		// Promote remaining servers in twos to maintain an odd quorum size
		for i := 0; i < len(promotions)-1; i += 2 {
			addFirst := s.raft.AddVoter(promotions[i].ID, promotions[i].Address, 0, 0)
			if err := addFirst.Error(); err != nil {
				return fmt.Errorf("failed to add raft peer: %v", err)
			}
			addSecond := s.raft.AddVoter(promotions[i+1].ID, promotions[i+1].Address, 0, 0)
			if err := addSecond.Error(); err != nil {
				return fmt.Errorf("failed to add raft peer: %v", err)
			}
			newServers = true
		}

		// If we added a new server, trigger a check to remove dead servers
		if newServers {
			go func() {
				s.autopilotRemoveDeadCh <- struct{}{}
			}()
		}
	}

	return nil
}

// queryServerHealth fetches the raft stats for the given server and uses them
// to update its ServerHealth
func (s *Server) queryServerHealth(member serf.Member, server *agent.Server, autopilotConf *structs.AutopilotConfig) *structs.ServerHealth {
	health := &structs.ServerHealth{
		ID:             server.ID,
		Name:           server.Name,
		SerfStatusRaw:  member.Status,
		SerfStatus:     member.Status.String(),
		LastContactRaw: -1,
		LastContact:    "never",
	}

	stats, err := s.getServerStats(server)
	if err != nil {
		s.logger.Printf("[DEBUG] consul: error getting server's raft stats: %s", err)
	}

	if v, ok := stats["last_contact"]; ok && v != "never" {
		health.LastContactRaw, err = time.ParseDuration(v)
		if err != nil {
			s.logger.Printf("[DEBUG] consul: error parsing server's last_contact value: %s", err)
		}
		health.LastContact = health.LastContactRaw.String()
	}
	// Set LastContact to 0 if we're the leader
	if s.config.NodeName == member.Name {
		health.LastContactRaw = 0
		health.LastContact = "leader"
	}

	if v, ok := stats["last_log_index"]; ok {
		health.LastIndex, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			s.logger.Printf("[DEBUG] consul: error parsing server's last_log_index value: %s", err)
		}
	}

	if v, ok := stats["last_log_term"]; ok {
		health.LastTerm, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			s.logger.Printf("[DEBUG] consul: error parsing server's last_log_term value: %s", err)
		}
	}

	health.Healthy = s.isServerHealthy(health, autopilotConf)

	// If this is a new server or the health changed, reset StableSince
	lastHealth := s.getServerHealth(server.Addr.String())
	if lastHealth == nil || lastHealth.Healthy != health.Healthy {
		health.StableSince = time.Now()
	} else {
		health.StableSince = lastHealth.StableSince
	}

	return health
}

func (s *Server) getServerHealth(addr string) *structs.ServerHealth {
	s.autopilotLock.RLock()
	defer s.autopilotLock.RUnlock()
	h, ok := s.autopilotHealth[addr]
	if !ok {
		return nil
	}
	return h
}

func (s *Server) getServerStats(server *agent.Server) (map[string]string, error) {
	var args struct{}
	var reply map[string]string
	err := s.connPool.RPC(s.config.Datacenter, server.Addr, server.Version, "Operator.RaftStats", &args, &reply)
	return reply, err
}

// isServerHealthy determines whether the given ServerHealth is healthy
// based on the current Autopilot config
func (s *Server) isServerHealthy(health *structs.ServerHealth, autopilotConf *structs.AutopilotConfig) bool {
	if health.SerfStatusRaw != serf.StatusAlive {
		return false
	}

	if health.LastContactRaw > autopilotConf.LastContactThreshold || health.LastContactRaw < 0 {
		return false
	}

	lastTerm, _ := strconv.ParseUint(s.raft.Stats()["last_log_term"], 10, 64)
	if health.LastTerm != lastTerm {
		return false
	}

	if s.raft.LastIndex() > autopilotConf.MaxTrailingLogs &&
		health.LastIndex < s.raft.LastIndex()-autopilotConf.MaxTrailingLogs {
		return false
	}

	return true
}
