package consul

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// AutopilotPolicy is the interface for the Autopilot mechanism
type AutopilotPolicy interface {
	// PromoteNonVoters defines the handling of non-voting servers
	PromoteNonVoters(*structs.AutopilotConfig) error
}

func (s *Server) startAutopilot() {
	s.autopilotShutdownCh = make(chan struct{})
	s.autopilotWaitGroup = sync.WaitGroup{}
	s.autopilotWaitGroup.Add(1)

	go s.autopilotLoop()
}

func (s *Server) stopAutopilot() {
	close(s.autopilotShutdownCh)
	s.autopilotWaitGroup.Wait()
}

var minAutopilotVersion = version.Must(version.NewVersion("0.8.0"))

// autopilotLoop periodically looks for nonvoting servers to promote and dead servers to remove.
func (s *Server) autopilotLoop() {
	defer s.autopilotWaitGroup.Done()

	// Monitor server health until shutdown
	ticker := time.NewTicker(s.config.AutopilotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.autopilotShutdownCh:
			return
		case <-ticker.C:
			autopilotConfig, ok := s.getOrCreateAutopilotConfig()
			if !ok {
				continue
			}

			if err := s.autopilotPolicy.PromoteNonVoters(autopilotConfig); err != nil {
				s.logger.Printf("[ERR] autopilot: Error checking for non-voters to promote: %s", err)
			}

			if err := s.pruneDeadServers(autopilotConfig); err != nil {
				s.logger.Printf("[ERR] autopilot: Error checking for dead servers to remove: %s", err)
			}
		case <-s.autopilotRemoveDeadCh:
			autopilotConfig, ok := s.getOrCreateAutopilotConfig()
			if !ok {
				continue
			}

			if err := s.pruneDeadServers(autopilotConfig); err != nil {
				s.logger.Printf("[ERR] autopilot: Error checking for dead servers to remove: %s", err)
			}
		}
	}
}

// fmtServer prints info about a server in a standard way for logging.
func fmtServer(server raft.Server) string {
	return fmt.Sprintf("Server (ID: %q Address: %q)", server.ID, server.Address)
}

// pruneDeadServers removes up to numPeers/2 failed servers
func (s *Server) pruneDeadServers(autopilotConfig *structs.AutopilotConfig) error {
	if !autopilotConfig.CleanupDeadServers {
		return nil
	}

	// Failed servers are known to Serf and marked failed, and stale servers
	// are known to Raft but not Serf.
	var failed []string
	staleRaftServers := make(map[string]raft.Server)
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}
	for _, server := range future.Configuration().Servers {
		staleRaftServers[string(server.Address)] = server
	}
	for _, member := range s.serfLAN.Members() {
		valid, parts := metadata.IsConsulServer(member)
		if valid {
			if _, ok := staleRaftServers[parts.Addr.String()]; ok {
				delete(staleRaftServers, parts.Addr.String())
			}

			if member.Status == serf.StatusFailed {
				failed = append(failed, member.Name)
			}
		}
	}

	// We can bail early if there's nothing to do.
	removalCount := len(failed) + len(staleRaftServers)
	if removalCount == 0 {
		return nil
	}

	// Only do removals if a minority of servers will be affected.
	peers, err := s.numPeers()
	if err != nil {
		return err
	}
	if removalCount < peers/2 {
		for _, node := range failed {
			s.logger.Printf("[INFO] autopilot: Attempting removal of failed server node %q", node)
			go s.serfLAN.RemoveFailedNode(node)
		}

		minRaftProtocol, err := ServerMinRaftProtocol(s.serfLAN.Members())
		if err != nil {
			return err
		}
		for _, raftServer := range staleRaftServers {
			s.logger.Printf("[INFO] autopilot: Attempting removal of stale %s", fmtServer(raftServer))
			var future raft.Future
			if minRaftProtocol >= 2 {
				future = s.raft.RemoveServer(raftServer.ID, 0, 0)
			} else {
				future = s.raft.RemovePeer(raftServer.Address)
			}
			if err := future.Error(); err != nil {
				return err
			}
		}
	} else {
		s.logger.Printf("[DEBUG] autopilot: Failed to remove dead servers: too many dead servers: %d/%d", removalCount, peers)
	}

	return nil
}

// BasicAutopilot defines a policy for promoting non-voting servers in a way
// that maintains an odd-numbered voter count.
type BasicAutopilot struct {
	server *Server
}

// PromoteNonVoters promotes eligible non-voting servers to voters.
func (b *BasicAutopilot) PromoteNonVoters(autopilotConfig *structs.AutopilotConfig) error {
	// If we don't meet the minimum version for non-voter features, bail
	// early.
	minRaftProtocol, err := ServerMinRaftProtocol(b.server.LANMembers())
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol < 3 {
		return nil
	}

	// Find any non-voters eligible for promotion.
	now := time.Now()
	var promotions []raft.Server
	future := b.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %v", err)
	}
	for _, server := range future.Configuration().Servers {
		if !isVoter(server.Suffrage) {
			health := b.server.getServerHealth(string(server.ID))
			if health.IsStable(now, autopilotConfig) {
				promotions = append(promotions, server)
			}
		}
	}

	if err := b.server.handlePromotions(promotions); err != nil {
		return err
	}
	return nil
}

// handlePromotions is a helper shared with Consul Enterprise that attempts to
// apply desired server promotions to the Raft configuration.
func (s *Server) handlePromotions(promotions []raft.Server) error {
	// This used to wait to only promote to maintain an odd quorum of
	// servers, but this was at odds with the dead server cleanup when doing
	// rolling updates (add one new server, wait, and then kill an old
	// server). The dead server cleanup would still count the old server as
	// a peer, which is conservative and the right thing to do, and this
	// would wait to promote, so you could get into a stalemate. It is safer
	// to promote early than remove early, so by promoting as soon as
	// possible we have chosen that as the solution here.
	for _, server := range promotions {
		s.logger.Printf("[INFO] autopilot: Promoting %s to voter", fmtServer(server))
		addFuture := s.raft.AddVoter(server.ID, server.Address, 0, 0)
		if err := addFuture.Error(); err != nil {
			return fmt.Errorf("failed to add raft peer: %v", err)
		}
	}

	// If we promoted a server, trigger a check to remove dead servers.
	if len(promotions) > 0 {
		select {
		case s.autopilotRemoveDeadCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// serverHealthLoop monitors the health of the servers in the cluster
func (s *Server) serverHealthLoop() {
	// Monitor server health until shutdown
	ticker := time.NewTicker(s.config.ServerHealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			if err := s.updateClusterHealth(); err != nil {
				s.logger.Printf("[ERR] autopilot: Error updating cluster health: %s", err)
			}
		}
	}
}

// updateClusterHealth fetches the Raft stats of the other servers and updates
// s.clusterHealth based on the configured Autopilot thresholds
func (s *Server) updateClusterHealth() error {
	// Don't do anything if the min Raft version is too low
	minRaftProtocol, err := ServerMinRaftProtocol(s.LANMembers())
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol < 3 {
		return nil
	}

	state := s.fsm.State()
	_, autopilotConf, err := state.AutopilotConfig()
	if err != nil {
		return fmt.Errorf("error retrieving autopilot config: %s", err)
	}
	// Bail early if autopilot config hasn't been initialized yet
	if autopilotConf == nil {
		return nil
	}

	// Get the the serf members which are Consul servers
	serverMap := make(map[string]*metadata.Server)
	for _, member := range s.LANMembers() {
		if member.Status == serf.StatusLeft {
			continue
		}

		valid, parts := metadata.IsConsulServer(member)
		if valid {
			serverMap[parts.ID] = parts
		}
	}

	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return fmt.Errorf("error getting Raft configuration %s", err)
	}
	servers := future.Configuration().Servers

	// Fetch the health for each of the servers in parallel so we get as
	// consistent of a sample as possible. We capture the leader's index
	// here as well so it roughly lines up with the same point in time.
	targetLastIndex := s.raft.LastIndex()
	var fetchList []*metadata.Server
	for _, server := range servers {
		if parts, ok := serverMap[string(server.ID)]; ok {
			fetchList = append(fetchList, parts)
		}
	}
	d := time.Now().Add(s.config.ServerHealthInterval / 2)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	fetchedStats := s.statsFetcher.Fetch(ctx, fetchList)

	// Build a current list of server healths
	leader := s.raft.Leader()
	var clusterHealth structs.OperatorHealthReply
	voterCount := 0
	healthyCount := 0
	healthyVoterCount := 0
	for _, server := range servers {
		health := structs.ServerHealth{
			ID:          string(server.ID),
			Address:     string(server.Address),
			Leader:      server.Address == leader,
			LastContact: -1,
			Voter:       server.Suffrage == raft.Voter,
		}

		parts, ok := serverMap[string(server.ID)]
		if ok {
			health.Name = parts.Name
			health.SerfStatus = parts.Status
			health.Version = parts.Build.String()
			if stats, ok := fetchedStats[string(server.ID)]; ok {
				if err := s.updateServerHealth(&health, parts, stats, autopilotConf, targetLastIndex); err != nil {
					s.logger.Printf("[WARN] autopilot: Error updating server %s health: %s", fmtServer(server), err)
				}
			}
		} else {
			health.SerfStatus = serf.StatusNone
		}

		if health.Voter {
			voterCount++
		}
		if health.Healthy {
			healthyCount++
			if health.Voter {
				healthyVoterCount++
			}
		}

		clusterHealth.Servers = append(clusterHealth.Servers, health)
	}
	clusterHealth.Healthy = healthyCount == len(servers)

	// If we have extra healthy voters, update FailureTolerance
	requiredQuorum := voterCount/2 + 1
	if healthyVoterCount > requiredQuorum {
		clusterHealth.FailureTolerance = healthyVoterCount - requiredQuorum
	}

	// Heartbeat a metric for monitoring if we're the leader
	if s.IsLeader() {
		metrics.SetGauge([]string{"consul", "autopilot", "failure_tolerance"}, float32(clusterHealth.FailureTolerance))
		metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(clusterHealth.FailureTolerance))
		if clusterHealth.Healthy {
			metrics.SetGauge([]string{"consul", "autopilot", "healthy"}, 1)
			metrics.SetGauge([]string{"autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"consul", "autopilot", "healthy"}, 0)
			metrics.SetGauge([]string{"autopilot", "healthy"}, 0)
		}
	}

	s.clusterHealthLock.Lock()
	s.clusterHealth = clusterHealth
	s.clusterHealthLock.Unlock()

	return nil
}

// updateServerHealth computes the resulting health of the server based on its
// fetched stats and the state of the leader.
func (s *Server) updateServerHealth(health *structs.ServerHealth,
	server *metadata.Server, stats *structs.ServerStats,
	autopilotConf *structs.AutopilotConfig, targetLastIndex uint64) error {

	health.LastTerm = stats.LastTerm
	health.LastIndex = stats.LastIndex

	if stats.LastContact != "never" {
		var err error
		health.LastContact, err = time.ParseDuration(stats.LastContact)
		if err != nil {
			return fmt.Errorf("error parsing last_contact duration: %s", err)
		}
	}

	lastTerm, err := strconv.ParseUint(s.raft.Stats()["last_log_term"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing last_log_term: %s", err)
	}
	health.Healthy = health.IsHealthy(lastTerm, targetLastIndex, autopilotConf)

	// If this is a new server or the health changed, reset StableSince
	lastHealth := s.getServerHealth(server.ID)
	if lastHealth == nil || lastHealth.Healthy != health.Healthy {
		health.StableSince = time.Now()
	} else {
		health.StableSince = lastHealth.StableSince
	}

	return nil
}

func (s *Server) getClusterHealth() structs.OperatorHealthReply {
	s.clusterHealthLock.RLock()
	defer s.clusterHealthLock.RUnlock()
	return s.clusterHealth
}

func (s *Server) getServerHealth(id string) *structs.ServerHealth {
	s.clusterHealthLock.RLock()
	defer s.clusterHealthLock.RUnlock()
	for _, health := range s.clusterHealth.Servers {
		if health.ID == id {
			return &health
		}
	}
	return nil
}

func isVoter(suffrage raft.ServerSuffrage) bool {
	switch suffrage {
	case raft.Voter, raft.Staging:
		return true
	default:
		return false
	}
}
