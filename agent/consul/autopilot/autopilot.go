package autopilot

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"net"
	"strconv"
	"sync"
	"time"
)

// Delegate is the interface for the Autopilot mechanism
type Delegate interface {
	AutopilotConfig() *Config
	FetchStats(context.Context, []serf.Member) map[string]*ServerStats
	IsServer(serf.Member) (*ServerInfo, error)
	NotifyHealth(OperatorHealthReply)
	PromoteNonVoters(*Config, OperatorHealthReply) ([]raft.Server, error)
	Raft() *raft.Raft
	SerfLAN() *serf.Serf
	SerfWAN() *serf.Serf
}

// Autopilot is a mechanism for automatically managing the Raft
// quorum using server health information along with updates from Serf gossip.
// For more information, see https://www.consul.io/docs/guides/autopilot.html
type Autopilot struct {
	logger   hclog.Logger
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

func NewAutopilot(logger hclog.Logger, delegate Delegate, interval, healthInterval time.Duration) *Autopilot {
	return &Autopilot{
		logger:         logger.Named(logging.Autopilot),
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
				a.logger.Error("Error promoting servers", "error", err)
			}

			if err := a.pruneDeadServers(); err != nil {
				a.logger.Error("Error checking for dead servers to remove", "error", err)
			}
		case <-a.removeDeadCh:
			if err := a.pruneDeadServers(); err != nil {
				a.logger.Error("Error checking for dead servers to remove", "error", err)
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

func canRemoveServers(peers, minQuorum, deadServers int) (bool, string) {
	if peers-deadServers < int(minQuorum) {
		return false, fmt.Sprintf("denied, because removing %d/%d servers would leave less then minimal allowed quorum of %d servers", deadServers, peers, minQuorum)
	}

	// Only do removals if a minority of servers will be affected.
	// For failure tolerance of F we need n = 2F+1 servers.
	// This means we can safely remove up to (n-1)/2 servers.
	if deadServers > (peers-1)/2 {
		return false, fmt.Sprintf("denied, because removing the majority of servers %d/%d is not safe", deadServers, peers)
	}
	return true, fmt.Sprintf("allowed, because removing %d/%d servers leaves a majority of servers above the minimal allowed quorum %d", deadServers, peers, minQuorum)
}

// pruneDeadServers removes up to numPeers/2 failed servers
func (a *Autopilot) pruneDeadServers() error {
	conf := a.delegate.AutopilotConfig()
	if conf == nil || !conf.CleanupDeadServers {
		return nil
	}

	// Failed servers are known to Serf and marked failed, and stale servers
	// are known to Raft but not Serf.
	var failed []serf.Member
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
	serfWAN := a.delegate.SerfWAN()
	serfLAN := a.delegate.SerfLAN()
	for _, member := range serfLAN.Members() {
		server, err := a.delegate.IsServer(member)
		if err != nil {
			a.logger.Warn("Error parsing server info", "name", member.Name, "error", err)
			continue
		}
		if server != nil {
			// todo(kyhavlov): change this to index by UUID
			s, found := staleRaftServers[server.Addr.String()]
			if found {
				delete(staleRaftServers, server.Addr.String())
			}

			if member.Status == serf.StatusFailed {
				// If the node is a nonvoter, we can remove it immediately.
				if found && s.Suffrage == raft.Nonvoter {
					a.logger.Info("Attempting removal of failed server node", "name", member.Name)
					go serfLAN.RemoveFailedNode(member.Name)
					if serfWAN != nil {
						go serfWAN.RemoveFailedNode(member.Name)
					}
				} else {
					failed = append(failed, member)

				}
			}
		}
	}

	deadServers := len(failed) + len(staleRaftServers)

	// nothing to do
	if deadServers == 0 {
		return nil
	}

	if ok, msg := canRemoveServers(NumPeers(raftConfig), int(conf.MinQuorum), deadServers); !ok {
		a.logger.Debug("Failed to remove dead servers", "error", msg)
		return nil
	}

	for _, node := range failed {
		a.logger.Info("Attempting removal of failed server node", "name", node.Name)
		go serfLAN.RemoveFailedNode(node.Name)
		if serfWAN != nil {
			go serfWAN.RemoveFailedNode(fmt.Sprintf("%s.%s", node.Name, node.Tags["dc"]))
		}

	}

	minRaftProtocol, err := a.MinRaftProtocol()
	if err != nil {
		return err
	}
	for _, raftServer := range staleRaftServers {
		a.logger.Info("Attempting removal of stale server", "server", fmtServer(raftServer))
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

	return nil
}

// MinRaftProtocol returns the lowest supported Raft protocol among alive servers
func (a *Autopilot) MinRaftProtocol() (int, error) {
	return minRaftProtocol(a.delegate.SerfLAN().Members(), a.delegate.IsServer)
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
		a.logger.Info("Promoting server to voter", "server", fmtServer(server))
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
				a.logger.Error("Error updating cluster health", "error", err)
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
	for _, member := range a.delegate.SerfLAN().Members() {
		if member.Status == serf.StatusLeft {
			continue
		}

		server, err := a.delegate.IsServer(member)
		if err != nil {
			a.logger.Warn("Error parsing server info", "name", member.Name, "error", err)
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
					a.logger.Warn("Error updating server health", "server", fmtServer(server), "error", err)
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
