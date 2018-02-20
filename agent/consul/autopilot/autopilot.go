package autopilot

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// Delegate is the interface for the Autopilot mechanism
type Delegate interface {
	AutopilotConfig() *Config
	FetchStats(context.Context, []serf.Member) map[string]*ServerStats
	IsServer(serf.Member) (*ServerInfo, error)
	NotifyHealth(OperatorHealthReply)
	PromoteNonVoters(*Config, OperatorHealthReply) ([]raft.Server, error)
	Raft() *raft.Raft
	Serf() *serf.Serf
}

// Autopilot is a mechanism for automatically managing the Raft
// quorum using server health information along with updates from Serf gossip.
// For more information, see https://www.consul.io/docs/guides/autopilot.html
type Autopilot struct {
	logger   *log.Logger
	delegate Delegate

	interval       time.Duration
	healthInterval time.Duration

	clusterHealth     OperatorHealthReply
	clusterHealthLock sync.RWMutex

	enabled      bool
	removeDeadCh chan struct{}
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
	waitGroup    sync.WaitGroup
}

type ServerInfo struct {
	Name   string
	ID     string
	Addr   net.Addr
	Build  version.Version
	Status serf.MemberStatus
}

func NewAutopilot(logger *log.Logger, delegate Delegate, interval, healthInterval time.Duration) *Autopilot {
	return &Autopilot{
		logger:         logger,
		delegate:       delegate,
		interval:       interval,
		healthInterval: healthInterval,
		removeDeadCh:   make(chan struct{}),
	}
}

func (a *Autopilot) Start() {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	// Nothing to do
	if a.enabled {
		return
	}

	a.shutdownCh = make(chan struct{})
	a.waitGroup = sync.WaitGroup{}
	a.clusterHealth = OperatorHealthReply{}

	a.waitGroup.Add(2)
	go a.run()
	go a.serverHealthLoop()
	a.enabled = true
}

func (a *Autopilot) Stop() {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	// Nothing to do
	if !a.enabled {
		return
	}

	close(a.shutdownCh)
	a.waitGroup.Wait()
	a.enabled = false
}

// run periodically looks for nonvoting servers to promote and dead servers to remove.
func (a *Autopilot) run() {
	defer a.waitGroup.Done()

	// Monitor server health until shutdown
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.shutdownCh:
			return
		case <-ticker.C:
			if err := a.promoteServers(); err != nil {
				a.logger.Printf("[ERR] autopilot: Error promoting servers: %v", err)
			}

			if err := a.pruneDeadServers(); err != nil {
				a.logger.Printf("[ERR] autopilot: Error checking for dead servers to remove: %s", err)
			}
		case <-a.removeDeadCh:
			if err := a.pruneDeadServers(); err != nil {
				a.logger.Printf("[ERR] autopilot: Error checking for dead servers to remove: %s", err)
			}
		}
	}
}

// promoteServers asks the delegate for any promotions and carries them out.
func (a *Autopilot) promoteServers() error {
	conf := a.delegate.AutopilotConfig()
	if conf == nil {
		return nil
	}

	// Skip the non-voter promotions unless all servers support the new APIs
	minRaftProtocol, err := a.MinRaftProtocol()
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol >= 3 {
		promotions, err := a.delegate.PromoteNonVoters(conf, a.GetClusterHealth())
		if err != nil {
			return fmt.Errorf("error checking for non-voters to promote: %s", err)
		}
		if err := a.handlePromotions(promotions); err != nil {
			return fmt.Errorf("error handling promotions: %s", err)
		}
	}

	return nil
}

// fmtServer prints info about a server in a standard way for logging.
func fmtServer(server raft.Server) string {
	return fmt.Sprintf("Server (ID: %q Address: %q)", server.ID, server.Address)
}

// NumPeers counts the number of voting peers in the given raft config.
func NumPeers(raftConfig raft.Configuration) int {
	var numPeers int
	for _, server := range raftConfig.Servers {
		if server.Suffrage == raft.Voter {
			numPeers++
		}
	}
	return numPeers
}

// RemoveDeadServers triggers a pruning of dead servers in a non-blocking way.
func (a *Autopilot) RemoveDeadServers() {
	select {
	case a.removeDeadCh <- struct{}{}:
	default:
	}
}

// pruneDeadServers removes up to numPeers/2 failed servers
func (a *Autopilot) pruneDeadServers() error {
	conf := a.delegate.AutopilotConfig()
	if conf == nil || !conf.CleanupDeadServers {
		return nil
	}

	// Failed servers are known to Serf and marked failed, and stale servers
	// are known to Raft but not Serf.
	var failed []string
	staleRaftServers := make(map[string]raft.Server)
	raftNode := a.delegate.Raft()
	future := raftNode.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	raftConfig := future.Configuration()
	for _, server := range raftConfig.Servers {
		staleRaftServers[string(server.Address)] = server
	}

	serfLAN := a.delegate.Serf()
	for _, member := range serfLAN.Members() {
		server, err := a.delegate.IsServer(member)
		if err != nil {
			a.logger.Printf("[INFO] autopilot: Error parsing server info for %q: %s", member.Name, err)
			continue
		}
		if server != nil {
			// todo(kyhavlov): change this to index by UUID
			if _, ok := staleRaftServers[server.Addr.String()]; ok {
				delete(staleRaftServers, server.Addr.String())
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
	peers := NumPeers(raftConfig)
	if removalCount < peers/2 {
		for _, node := range failed {
			a.logger.Printf("[INFO] autopilot: Attempting removal of failed server node %q", node)
			go serfLAN.RemoveFailedNode(node)
		}

		minRaftProtocol, err := a.MinRaftProtocol()
		if err != nil {
			return err
		}
		for _, raftServer := range staleRaftServers {
			a.logger.Printf("[INFO] autopilot: Attempting removal of stale %s", fmtServer(raftServer))
			var future raft.Future
			if minRaftProtocol >= 2 {
				future = raftNode.RemoveServer(raftServer.ID, 0, 0)
			} else {
				future = raftNode.RemovePeer(raftServer.Address)
			}
			if err := future.Error(); err != nil {
				return err
			}
		}
	} else {
		a.logger.Printf("[DEBUG] autopilot: Failed to remove dead servers: too many dead servers: %d/%d", removalCount, peers)
	}

	return nil
}

// MinRaftProtocol returns the lowest supported Raft protocol among alive servers
func (a *Autopilot) MinRaftProtocol() (int, error) {
	return minRaftProtocol(a.delegate.Serf().Members(), a.delegate.IsServer)
}

func minRaftProtocol(members []serf.Member, serverFunc func(serf.Member) (*ServerInfo, error)) (int, error) {
	minVersion := -1
	for _, m := range members {
		if m.Status != serf.StatusAlive {
			continue
		}

		server, err := serverFunc(m)
		if err != nil {
			return -1, err
		}
		if server == nil {
			continue
		}

		vsn, ok := m.Tags["raft_vsn"]
		if !ok {
			vsn = "1"
		}
		raftVsn, err := strconv.Atoi(vsn)
		if err != nil {
			return -1, err
		}

		if minVersion == -1 || raftVsn < minVersion {
			minVersion = raftVsn
		}
	}

	if minVersion == -1 {
		return minVersion, fmt.Errorf("No servers found")
	}

	return minVersion, nil
}

// handlePromotions is a helper shared with Consul Enterprise that attempts to
// apply desired server promotions to the Raft configuration.
func (a *Autopilot) handlePromotions(promotions []raft.Server) error {
	// This used to wait to only promote to maintain an odd quorum of
	// servers, but this was at odds with the dead server cleanup when doing
	// rolling updates (add one new server, wait, and then kill an old
	// server). The dead server cleanup would still count the old server as
	// a peer, which is conservative and the right thing to do, and this
	// would wait to promote, so you could get into a stalemate. It is safer
	// to promote early than remove early, so by promoting as soon as
	// possible we have chosen that as the solution here.
	for _, server := range promotions {
		a.logger.Printf("[INFO] autopilot: Promoting %s to voter", fmtServer(server))
		addFuture := a.delegate.Raft().AddVoter(server.ID, server.Address, 0, 0)
		if err := addFuture.Error(); err != nil {
			return fmt.Errorf("failed to add raft peer: %v", err)
		}
	}

	// If we promoted a server, trigger a check to remove dead servers.
	if len(promotions) > 0 {
		select {
		case a.removeDeadCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// serverHealthLoop monitors the health of the servers in the cluster
func (a *Autopilot) serverHealthLoop() {
	defer a.waitGroup.Done()

	// Monitor server health until shutdown
	ticker := time.NewTicker(a.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.shutdownCh:
			return
		case <-ticker.C:
			if err := a.updateClusterHealth(); err != nil {
				a.logger.Printf("[ERR] autopilot: Error updating cluster health: %s", err)
			}
		}
	}
}

// updateClusterHealth fetches the Raft stats of the other servers and updates
// s.clusterHealth based on the configured Autopilot thresholds
func (a *Autopilot) updateClusterHealth() error {
	// Don't do anything if the min Raft version is too low
	minRaftProtocol, err := a.MinRaftProtocol()
	if err != nil {
		return fmt.Errorf("error getting server raft protocol versions: %s", err)
	}
	if minRaftProtocol < 3 {
		return nil
	}

	autopilotConf := a.delegate.AutopilotConfig()
	// Bail early if autopilot config hasn't been initialized yet
	if autopilotConf == nil {
		return nil
	}

	// Get the the serf members which are Consul servers
	var serverMembers []serf.Member
	serverMap := make(map[string]*ServerInfo)
	for _, member := range a.delegate.Serf().Members() {
		if member.Status == serf.StatusLeft {
			continue
		}

		server, err := a.delegate.IsServer(member)
		if err != nil {
			a.logger.Printf("[INFO] autopilot: Error parsing server info for %q: %s", member.Name, err)
			continue
		}
		if server != nil {
			serverMap[server.ID] = server
			serverMembers = append(serverMembers, member)
		}
	}

	raftNode := a.delegate.Raft()
	future := raftNode.GetConfiguration()
	if err := future.Error(); err != nil {
		return fmt.Errorf("error getting Raft configuration %s", err)
	}
	servers := future.Configuration().Servers

	// Fetch the health for each of the servers in parallel so we get as
	// consistent of a sample as possible. We capture the leader's index
	// here as well so it roughly lines up with the same point in time.
	targetLastIndex := raftNode.LastIndex()
	var fetchList []*ServerInfo
	for _, server := range servers {
		if parts, ok := serverMap[string(server.ID)]; ok {
			fetchList = append(fetchList, parts)
		}
	}
	d := time.Now().Add(a.healthInterval / 2)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	fetchedStats := a.delegate.FetchStats(ctx, serverMembers)

	// Build a current list of server healths
	leader := raftNode.Leader()
	var clusterHealth OperatorHealthReply
	voterCount := 0
	healthyCount := 0
	healthyVoterCount := 0
	for _, server := range servers {
		health := ServerHealth{
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
				if err := a.updateServerHealth(&health, parts, stats, autopilotConf, targetLastIndex); err != nil {
					a.logger.Printf("[WARN] autopilot: Error updating server %s health: %s", fmtServer(server), err)
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

	a.delegate.NotifyHealth(clusterHealth)

	a.clusterHealthLock.Lock()
	a.clusterHealth = clusterHealth
	a.clusterHealthLock.Unlock()

	return nil
}

// updateServerHealth computes the resulting health of the server based on its
// fetched stats and the state of the leader.
func (a *Autopilot) updateServerHealth(health *ServerHealth,
	server *ServerInfo, stats *ServerStats,
	autopilotConf *Config, targetLastIndex uint64) error {

	health.LastTerm = stats.LastTerm
	health.LastIndex = stats.LastIndex

	if stats.LastContact != "never" {
		var err error
		health.LastContact, err = time.ParseDuration(stats.LastContact)
		if err != nil {
			return fmt.Errorf("error parsing last_contact duration: %s", err)
		}
	}

	raftNode := a.delegate.Raft()
	lastTerm, err := strconv.ParseUint(raftNode.Stats()["last_log_term"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing last_log_term: %s", err)
	}
	health.Healthy = health.IsHealthy(lastTerm, targetLastIndex, autopilotConf)

	// If this is a new server or the health changed, reset StableSince
	lastHealth := a.GetServerHealth(server.ID)
	if lastHealth == nil || lastHealth.Healthy != health.Healthy {
		health.StableSince = time.Now()
	} else {
		health.StableSince = lastHealth.StableSince
	}

	return nil
}

func (a *Autopilot) GetClusterHealth() OperatorHealthReply {
	a.clusterHealthLock.RLock()
	defer a.clusterHealthLock.RUnlock()
	return a.clusterHealth
}

func (a *Autopilot) GetServerHealth(id string) *ServerHealth {
	a.clusterHealthLock.RLock()
	defer a.clusterHealthLock.RUnlock()
	return a.clusterHealth.ServerHealth(id)
}

func IsPotentialVoter(suffrage raft.ServerSuffrage) bool {
	switch suffrage {
	case raft.Voter, raft.Staging:
		return true
	default:
		return false
	}
}
