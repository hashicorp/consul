package server_manager

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/consul/server_details"
	"github.com/hashicorp/consul/lib"
)

type consulServerEventTypes int

const (
	// clientRPCJitterFraction determines the amount of jitter added to
	// clientRPCMinReuseDuration before a connection is expired and a new
	// connection is established in order to rebalance load across consul
	// servers.  The cluster-wide number of connections per second from
	// rebalancing is applied after this jitter to ensure the CPU impact
	// is always finite.  See newRebalanceConnsPerSecPerServer's comment
	// for additional commentary.
	//
	// For example, in a 10K consul cluster with 5x servers, this default
	// averages out to ~13 new connections from rebalancing per server
	// per second (each connection is reused for 120s to 180s).
	clientRPCJitterFraction = 2

	// clientRPCMinReuseDuration controls the minimum amount of time RPC
	// queries are sent over an established connection to a single server
	clientRPCMinReuseDuration = 120 * time.Second

	// initialRebalanceTimeoutHours is the initial value for the
	// rebalanceTimer.  This value is discarded immediately after the
	// client becomes aware of the first server.
	initialRebalanceTimeoutHours = 24

	// Limit the number of new connections a server receives per second
	// for connection rebalancing.  This limit caps the load caused by
	// continual rebalancing efforts when a cluster is in equilibrium.  A
	// lower value comes at the cost of increased recovery time after a
	// partition.  This parameter begins to take effect when there are
	// more than ~48K clients querying 5x servers or at lower server
	// values when there is a partition.
	//
	// For example, in a 100K consul cluster with 5x servers, it will
	// take ~5min for all servers to rebalance their connections.  If
	// 99,995 agents are in the minority talking to only one server, it
	// will take ~26min for all servers to rebalance.  A 10K cluster in
	// the same scenario will take ~2.6min to rebalance.
	newRebalanceConnsPerSecPerServer = 64

	// maxConsulServerManagerEvents is the size of the consulServersCh
	// buffer.
	maxConsulServerManagerEvents = 16

	// defaultClusterSize is the assumed cluster size if no serf cluster
	// is available.
	defaultClusterSize = 1024
)

type ConsulClusterInfo interface {
	NumNodes() int
}

// serverCfg is the thread-safe configuration structure that is used to
// maintain the list of consul servers in Client.
//
// NOTE(sean@): We are explicitly relying on the fact that this is copied.
// Please keep this structure light.
type serverConfig struct {
	// servers tracks the locally known servers
	servers []*server_details.ServerDetails
}

type ServerManager struct {
	// serverConfig provides the necessary load/store semantics to
	// serverConfig
	serverConfigValue atomic.Value
	serverConfigLock  sync.Mutex

	// consulServersCh is used to receive events related to the
	// maintenance of the list of consulServers
	consulServersCh chan consulServerEventTypes

	// refreshRebalanceDurationCh is used to signal that a refresh should
	// occur
	refreshRebalanceDurationCh chan bool

	// shutdownCh is a copy of the channel in consul.Client
	shutdownCh chan struct{}

	// logger uses the provided LogOutput
	logger *log.Logger

	// serf is used to estimate the approximate number of nodes in a
	// cluster and limit the rate at which it rebalances server
	// connections
	clusterInfo ConsulClusterInfo

	// notifyFailedServersBarrier is acts as a barrier to prevent
	// queueing behind serverConfigLog and acts as a TryLock().
	notifyFailedBarrier int32
}

// AddServer takes out an internal write lock and adds a new server.  If the
// server is not known, it adds the new server and schedules a rebalance.  If
// it is known, we merge the new server details.
func (sm *ServerManager) AddServer(server *server_details.ServerDetails) {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	// Check if this server is known
	found := false
	for idx, existing := range serverCfg.servers {
		if existing.Name == server.Name {
			newServers := make([]*server_details.ServerDetails, len(serverCfg.servers))
			copy(newServers, serverCfg.servers)

			// Overwrite the existing server details in order to
			// possibly update metadata (e.g. server version)
			newServers[idx] = server

			serverCfg.servers = newServers
			found = true
			break
		}
	}

	// Add to the list if not known
	if !found {
		newServers := make([]*server_details.ServerDetails, len(serverCfg.servers), len(serverCfg.servers)+1)
		copy(newServers, serverCfg.servers)
		newServers = append(newServers, server)
		serverCfg.servers = newServers
	}

	sm.saveServerConfig(serverCfg)
}

// cycleServers returns a new list of servers that has dequeued the first
// server and enqueued it at the end of the list.  cycleServers assumes the
// caller is holding the serverConfigLock.
func (sc *serverConfig) cycleServer() (servers []*server_details.ServerDetails) {
	numServers := len(sc.servers)
	if numServers < 2 {
		// No action required
		return servers
	}

	newServers := make([]*server_details.ServerDetails, 0, numServers)
	newServers = append(newServers, sc.servers[1:]...)
	newServers = append(newServers, sc.servers[0])
	return newServers
}

// FindHealthyServer takes out an internal "read lock" and searches through
// the list of servers to find a healthy server.
func (sm *ServerManager) FindHealthyServer() *server_details.ServerDetails {
	serverCfg := sm.getServerConfig()
	numServers := len(serverCfg.servers)
	if numServers == 0 {
		sm.logger.Printf("[ERR] consul: No servers found in the server config")
		return nil
	} else {
		// Return whatever is at the front of the list
		return serverCfg.servers[0]
	}
}

// GetNumServers takes out an internal "read lock" and returns the number of
// servers.  numServers includes both healthy and unhealthy servers.
func (sm *ServerManager) GetNumServers() (numServers int) {
	serverCfg := sm.getServerConfig()
	numServers = len(serverCfg.servers)
	return numServers
}

// getServerConfig is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (sm *ServerManager) getServerConfig() serverConfig {
	return sm.serverConfigValue.Load().(serverConfig)
}

// NewServerManager is the only way to safely create a new ServerManager
// struct.
func NewServerManager(logger *log.Logger, shutdownCh chan struct{}, cci ConsulClusterInfo) (sm *ServerManager) {
	// NOTE(sean@): Can't pass *consul.Client due to an import cycle
	sm = new(ServerManager)
	sm.logger = logger
	sm.clusterInfo = cci
	sm.consulServersCh = make(chan consulServerEventTypes, maxConsulServerManagerEvents)
	sm.shutdownCh = shutdownCh

	sm.refreshRebalanceDurationCh = make(chan bool, maxConsulServerManagerEvents)

	sc := serverConfig{}
	sc.servers = make([]*server_details.ServerDetails, 0)
	sm.serverConfigValue.Store(sc)
	return sm
}

// NotifyFailedServer is an exported convenience function that allows callers
// to pass in a server that has failed an RPC request and mark it as failed.
// If the server being failed is not the first server on the list, this is a
// noop.  If, however, the server is failed and first on the list, acquire
// the lock, retest, and take the penalty of moving the server to the end of
// the list.
func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {
	serverCfg := sm.getServerConfig()

	// Use atomic.CAS to emulate a TryLock().
	if len(serverCfg.servers) > 0 && serverCfg.servers[0] == server &&
		atomic.CompareAndSwapInt32(&sm.notifyFailedBarrier, 0, 1) {
		defer atomic.StoreInt32(&sm.notifyFailedBarrier, 0)

		// Grab a lock, retest, and take the hit of cycling the first
		// server to the end.
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()
		serverCfg = sm.getServerConfig()

		if len(serverCfg.servers) > 0 && serverCfg.servers[0] == server {
			serverCfg.cycleServer()
			sm.saveServerConfig(serverCfg)
		}
	}
}

// RebalanceServers takes out an internal write lock and shuffles the list of
// servers on this agent.  This allows for a redistribution of work across
// consul servers and provides a guarantee that the order list of
// ServerDetails isn't actually ordered, therefore we can sequentially walk
// the array to pick a server without all agents in the cluster dog piling on
// a single node.
func (sm *ServerManager) RebalanceServers() {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	newServers := make([]*server_details.ServerDetails, len(serverCfg.servers))
	copy(newServers, serverCfg.servers)

	// Shuffle the server list on server join.  Servers are selected from
	// the head of the list and are moved to the end of the list on
	// failure.
	for i := len(serverCfg.servers) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		newServers[i], newServers[j] = newServers[j], newServers[i]
	}
	serverCfg.servers = newServers

	sm.saveServerConfig(serverCfg)
}

// RemoveServer takes out an internal write lock and removes a server from
// the server list.  No rebalancing happens as a result of the removed server
// because we do not want a network partition which separated a server from
// this agent to cause an increase in work.  Instead we rely on the internal
// already existing semantics to handle failure detection after a server has
// been removed.
func (sm *ServerManager) RemoveServer(server *server_details.ServerDetails) {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	// Remove the server if known
	n := len(serverCfg.servers)
	for i := 0; i < n; i++ {
		if serverCfg.servers[i].Name == server.Name {
			newServers := make([]*server_details.ServerDetails, len(serverCfg.servers)-1)
			copy(newServers, serverCfg.servers)

			newServers[i], newServers[n-1] = newServers[n-1], nil
			newServers = newServers[:n-1]
			serverCfg.servers = newServers

			sm.saveServerConfig(serverCfg)
			return
		}
	}
}

// requestRefreshRebalanceDuration sends a message to which causes a background
// thread to recalc the duration
func (sm *ServerManager) requestRefreshRebalanceDuration() {
	sm.refreshRebalanceDurationCh <- true
}

// refreshServerRebalanceTimer is called
func (sm *ServerManager) refreshServerRebalanceTimer(timer *time.Timer) {
	serverCfg := sm.getServerConfig()
	numConsulServers := len(serverCfg.servers)
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numConsulServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)

	numLANMembers := sm.clusterInfo.NumNodes()
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)
	sm.logger.Printf("[DEBUG] consul: connection will be rebalanced in %v", connRebalanceTimeout)

	timer.Reset(connRebalanceTimeout)
}

// saveServerConfig is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (sm *ServerManager) saveServerConfig(sc serverConfig) {
	sm.serverConfigValue.Store(sc)
}

// Start is used to start and manage the task of automatically shuffling and
// rebalance the list of consul servers.  This maintenance happens either
// when a new server is added or when a duration has been exceed.
func (sm *ServerManager) Start() {
	var rebalanceTimer *time.Timer = time.NewTimer(time.Duration(initialRebalanceTimeoutHours * time.Hour))
	var rebalanceTaskDispatched int32

	func() {
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()

		serverCfgPtr := sm.serverConfigValue.Load()
		if serverCfgPtr == nil {
			panic("server config has not been initialized")
		}
		var serverCfg serverConfig
		serverCfg = serverCfgPtr.(serverConfig)
		sm.saveServerConfig(serverCfg)
	}()

	for {
		select {
		case <-rebalanceTimer.C:
			sm.logger.Printf("[INFO] server manager: server rebalance timeout")
			sm.RebalanceServers()

			// Only run one rebalance task at a time, but do
			// allow for the channel to be drained
			if atomic.CompareAndSwapInt32(&rebalanceTaskDispatched, 0, 1) {
				sm.logger.Printf("[INFO] server manager: Launching rebalance duration task")
				go func() {
					defer atomic.StoreInt32(&rebalanceTaskDispatched, 0)
					sm.refreshServerRebalanceTimer(rebalanceTimer)
				}()
			}

		case <-sm.shutdownCh:
			sm.logger.Printf("[INFO] server manager: shutting down")
			return
		}
	}
}
