package server_manager

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/consul/server_details"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/serf/serf"
)

type consulServerEventTypes int

const (
	// consulServersNodeJoin is used to notify of a new consulServer.
	// The primary effect of this is a reshuffling of consulServers and
	// finding a new preferredServer.
	consulServersNodeJoin = iota

	// consulServersRebalance is used to signal we should rebalance our
	// connection load across servers
	consulServersRebalance

	// consulServersRefreshRebalanceDuration is used to signal when we
	// should reset the rebalance duration because the server list has
	// changed and we don't need to proactively change our connection
	consulServersRefreshRebalanceDuration

	// consulServersRPCError is used to signal when a server has either
	// timed out or returned an error and we would like to have the
	// server manager find a new preferredServer.
	consulServersRPCError
)

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
	serf *serf.Serf
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

		// Notify the server maintenance task of a new server
		sm.consulServersCh <- consulServersNodeJoin
	}

	sm.saveServerConfig(serverCfg)
}

// CycleFailedServers takes out an internal write lock and dequeues all
// failed servers and re-enqueues them.  This method does not reshuffle the
// server list, instead it requests the rebalance duration be refreshed/reset
// further into the future.
func (sm *ServerManager) CycleFailedServers() {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	for i := range serverCfg.servers {
		failCount := atomic.LoadUint64(&(serverCfg.servers[i].Disabled))
		if failCount == 0 {
			break
		} else if failCount > 0 {
			serverCfg.servers = serverCfg.cycleServer()
		}
	}

	sm.saveServerConfig(serverCfg)
	sm.requestRefreshRebalanceDuration()
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
func (sm *ServerManager) FindHealthyServer() (server *server_details.ServerDetails) {
	serverCfg := sm.getServerConfig()
	numServers := len(serverCfg.servers)
	if numServers == 0 {
		sm.logger.Printf("[ERR] consul: No servers found in the server config")
		return nil
	}

	// Find the first non-failing server in the server list.  If this is
	// not the first server a prior RPC call marked the first server as
	// failed and we're waiting for the server management task to reorder
	// a working server to the front of the list.
	for i := range serverCfg.servers {
		failCount := atomic.LoadUint64(&(serverCfg.servers[i].Disabled))
		if failCount == 0 {
			server = serverCfg.servers[i]
			break
		}
	}

	return server
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
func NewServerManager(logger *log.Logger, shutdownCh chan struct{}, serf *serf.Serf) (sm *ServerManager) {
	// NOTE(sean@): Can't pass *consul.Client due to an import cycle
	sm = new(ServerManager)
	sm.logger = logger
	sm.serf = serf
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
// This will initiate a background task that will optimize the failed server
// to the end of the serer list.  No locks are required here because we are
// bypassing the serverConfig and sending a message to ServerManager's
// channel.
func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {
	atomic.AddUint64(&server.Disabled, 1)
	sm.consulServersCh <- consulServersRPCError
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
	sm.requestRefreshRebalanceDuration()
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

// requestServerRebalance sends a message to which causes a background thread
// to reshuffle the list of servers
func (sm *ServerManager) requestServerRebalance() {
	sm.consulServersCh <- consulServersRebalance
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

	// Assume a moderate sized cluster unless we have an actual serf
	// instance we can query.
	numLANMembers := defaultClusterSize
	if sm.serf != nil {
		numLANMembers = sm.serf.NumNodes()
	}
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)
	sm.logger.Printf("[DEBUG] consul: connection will be rebalanced in %v", connRebalanceTimeout)

	timer.Reset(connRebalanceTimeout)
}

// saveServerConfig is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (sm *ServerManager) saveServerConfig(sc serverConfig) {
	sm.serverConfigValue.Store(sc)
}

// StartServerManager is used to start and manage the task of automatically
// shuffling and rebalance the list of consul servers.  This maintenance
// happens either when a new server is added or when a duration has been
// exceed.
func (sm *ServerManager) StartServerManager() {
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
		case e := <-sm.consulServersCh:
			switch e {
			case consulServersNodeJoin:
				sm.logger.Printf("[INFO] server manager: new node joined cluster")
				// rebalance on new server
				sm.requestServerRebalance()
			case consulServersRebalance:
				sm.logger.Printf("[INFO] server manager: rebalancing servers by request")
				sm.RebalanceServers()
			case consulServersRPCError:
				sm.logger.Printf("[INFO] server manager: need to find a new server to talk with")
				sm.CycleFailedServers()
				// FIXME(sean@): wtb preemptive Status.Ping
				// of servers, ideally parallel fan-out of N
				// nodes, then settle on the first node which
				// responds successfully.
				//
				// Is there a distinction between slow and
				// offline?  Do we run the Status.Ping with a
				// fixed timeout (say 30s) that way we can
				// alert administrators that they've set
				// their RPC time too low even though the
				// Ping did return successfully?
			default:
				sm.logger.Printf("[WARN] server manager: unhandled LAN Serf Event: %#v", e)
			}
		case <-sm.refreshRebalanceDurationCh:
			chanLen := len(sm.refreshRebalanceDurationCh)
			// Drain all messages from the rebalance channel
			for i := 0; i < chanLen; i++ {
				<-sm.refreshRebalanceDurationCh
			}
			// Only run one rebalance task at a time, but do
			// allow for the channel to be drained
			if atomic.CompareAndSwapInt32(&rebalanceTaskDispatched, 0, 1) {
				go func() {
					defer atomic.StoreInt32(&rebalanceTaskDispatched, 0)
					sm.refreshServerRebalanceTimer(rebalanceTimer)
				}()
			}
		case <-rebalanceTimer.C:
			sm.logger.Printf("[INFO] consul: server rebalance timeout")
			sm.RebalanceServers()

		case <-sm.shutdownCh:
			return
		}
	}
}
