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
)

// ConsulClusterInfo is an interface wrapper around serf and prevents a
// cyclic import dependency
type ConsulClusterInfo interface {
	NumNodes() int
}

// serverCfg is the thread-safe configuration struct used to maintain the
// list of Consul servers in ServerManager.
//
// NOTE(sean@): We are explicitly relying on the fact that serverConfig will
// be copied onto the stack.  Please keep this structure light.
type serverConfig struct {
	// servers tracks the locally known servers.  List membership is
	// maintained by Serf.
	servers []*server_details.ServerDetails
}

type ServerManager struct {
	// serverConfig provides the necessary load/store semantics for the
	// server list.
	serverConfigValue atomic.Value
	serverConfigLock  sync.Mutex

	// shutdownCh is a copy of the channel in consul.Client
	shutdownCh chan struct{}

	logger *log.Logger

	// clusterInfo is used to estimate the approximate number of nodes in
	// a cluster and limit the rate at which it rebalances server
	// connections.  ConsulClusterInfo is an interface that wraps serf.
	clusterInfo ConsulClusterInfo

	// notifyFailedServersBarrier is acts as a barrier to prevent
	// queueing behind serverConfigLog and acts as a TryLock().
	notifyFailedBarrier int32
}

// AddServer takes out an internal write lock and adds a new server.  If the
// server is not known, appends the server to the list.  The new server will
// begin seeing use after the rebalance timer fires or enough servers fail
// organically.  If the server is already known, merge the new server
// details.
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
// caller is holding the serverConfigLock.  cycleServer does not test or ping
// the next server inline.  cycleServer may be called when the environment
// has just entered an unhealthy situation and blocking on a server test is
// less desirable than just returning the next server in the firing line.  If
// the next server fails, it will fail fast enough and cycleServer will be
// called again.
func (sc *serverConfig) cycleServer() (servers []*server_details.ServerDetails) {
	numServers := len(sc.servers)
	if numServers < 2 {
		return servers // No action required
	}

	newServers := make([]*server_details.ServerDetails, 0, numServers)
	newServers = append(newServers, sc.servers[1:]...)
	newServers = append(newServers, sc.servers[0])

	// FIXME(sean@): Is it worth it to fire off a go routine and
	// TestConsulServer?
	return newServers
}

// FindServer takes out an internal "read lock" and searches through the list
// of servers to find a "healthy" server.  If the server is actually
// unhealthy, we rely on Serf to detect this and remove the node from the
// server list.  If the server at the front of the list has failed or fails
// during an RPC call, it is rotated to the end of the list.  If there are no
// servers available, return nil.
func (sm *ServerManager) FindServer() *server_details.ServerDetails {
	serverCfg := sm.getServerConfig()
	numServers := len(serverCfg.servers)
	if numServers == 0 {
		sm.logger.Printf("[WARN] consul: No servers available")
		return nil
	} else {
		// Return whatever is at the front of the list because it is
		// assumed to be the oldest in the server list (unless -
		// hypothetically - the server list was rotated right after a
		// server was added).
		return serverCfg.servers[0]
	}
}

// getServerConfig is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (sm *ServerManager) getServerConfig() serverConfig {
	return sm.serverConfigValue.Load().(serverConfig)
}

// saveServerConfig is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (sm *ServerManager) saveServerConfig(sc serverConfig) {
	sm.serverConfigValue.Store(sc)
}

// New is the only way to safely create a new ServerManager struct.
func New(logger *log.Logger, shutdownCh chan struct{}, clusterInfo ConsulClusterInfo) (sm *ServerManager) {
	// NOTE(sean@): Can't pass *consul.Client due to an import cycle
	sm = new(ServerManager)
	sm.logger = logger
	sm.clusterInfo = clusterInfo
	sm.shutdownCh = shutdownCh

	sc := serverConfig{}
	sc.servers = make([]*server_details.ServerDetails, 0)
	sm.saveServerConfig(sc)
	return sm
}

// NotifyFailedServer marks the passed in server as "failed" by rotating it
// to the end of the server list.
func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {
	serverCfg := sm.getServerConfig()

	// If the server being failed is not the first server on the list,
	// this is a noop.  If, however, the server is failed and first on
	// the list, acquire the lock, retest, and take the penalty of moving
	// the server to the end of the list.

	// Only rotate the server list when there is more than one server
	if len(serverCfg.servers) > 1 && serverCfg.servers[0] == server &&
		// Use atomic.CAS to emulate a TryLock().
		atomic.CompareAndSwapInt32(&sm.notifyFailedBarrier, 0, 1) {
		defer atomic.StoreInt32(&sm.notifyFailedBarrier, 0)

		// Grab a lock, retest, and take the hit of cycling the first
		// server to the end.
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()
		serverCfg = sm.getServerConfig()

		if len(serverCfg.servers) > 1 && serverCfg.servers[0] == server {
			serverCfg.servers = serverCfg.cycleServer()
			sm.saveServerConfig(serverCfg)
		}
	}
}

// NumServers takes out an internal "read lock" and returns the number of
// servers.  numServers includes both healthy and unhealthy servers.
func (sm *ServerManager) NumServers() (numServers int) {
	serverCfg := sm.getServerConfig()
	numServers = len(serverCfg.servers)
	return numServers
}

// RebalanceServers takes out an internal write lock and shuffles the list of
// servers on this agent.  This allows for a redistribution of work across
// consul servers and provides a guarantee that the order of the server list
// isn't related to the age at which the node was added to the cluster.
// Elsewhere we rely on the position in the server list as a hint regarding
// the stability of a server relative to its position in the server list.
// Servers at or near the front of the list are more stable than servers near
// the end of the list.  Unhealthy servers are removed when serf notices the
// server has been deregistered.
func (sm *ServerManager) RebalanceServers() {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	newServers := make([]*server_details.ServerDetails, len(serverCfg.servers))
	copy(newServers, serverCfg.servers)

	// Shuffle the server list
	for i := len(serverCfg.servers) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		newServers[i], newServers[j] = newServers[j], newServers[i]
	}
	serverCfg.servers = newServers

	sm.saveServerConfig(serverCfg)
}

// RemoveServer takes out an internal write lock and removes a server from
// the server list.
func (sm *ServerManager) RemoveServer(server *server_details.ServerDetails) {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	serverCfg := sm.getServerConfig()

	// Remove the server if known
	for i, _ := range serverCfg.servers {
		if serverCfg.servers[i].Name == server.Name {
			newServers := make([]*server_details.ServerDetails, 0, len(serverCfg.servers)-1)
			newServers = append(newServers, serverCfg.servers[:i]...)
			newServers = append(newServers, serverCfg.servers[i+1:]...)
			serverCfg.servers = newServers

			sm.saveServerConfig(serverCfg)
			return
		}
	}
}

// refreshServerRebalanceTimer is only called once the rebalanceTimer
// expires.  Historically this was an expensive routine and is intended to be
// run in isolation in a dedicated, non-concurrent task.
func (sm *ServerManager) refreshServerRebalanceTimer(timer *time.Timer) time.Duration {
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

	timer.Reset(connRebalanceTimeout)
	return connRebalanceTimeout
}

// Start is used to start and manage the task of automatically shuffling and
// rebalancing the list of consul servers.  This maintenance only happens
// periodically based on the expiration of the timer.  Failed servers are
// automatically cycled to the end of the list.  New servers are appended to
// the list.  The order of the server list must be shuffled periodically to
// distribute load across all known and available consul servers.
func (sm *ServerManager) Start() {
	var rebalanceTimer *time.Timer = time.NewTimer(clientRPCMinReuseDuration)

	for {
		select {
		case <-rebalanceTimer.C:
			sm.logger.Printf("[INFO] server manager: Rebalancing server connections")
			sm.RebalanceServers()
			sm.refreshServerRebalanceTimer(rebalanceTimer)

		case <-sm.shutdownCh:
			sm.logger.Printf("[INFO] server manager: shutting down")
			return
		}
	}
}
