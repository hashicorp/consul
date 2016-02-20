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
	// consulServersNodeJoin is used to notify of a new consulServer.
	// The primary effect of this is a reshuffling of consulServers and
	// finding a new preferredServer.
	consulServersNodeJoin = iota

	// consulServersRebalance is used to signal we should rebalance our
	// connection load across servers
	consulServersRebalance

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

// serverCfg is the thread-safe configuration structure that is used to
// maintain the list of consul servers in Client.
//
// NOTE(sean@): We are explicitly relying on the fact that this is copied.
// Please keep this structure light.
type serverConfig struct {
	// servers tracks the locally known servers
	servers []*server_details.ServerDetails

	// Timer used to control rebalancing of servers
	rebalanceTimer *time.Timer
}

type ServerManager struct {
	// serverConfig provides the necessary load/store semantics to
	// serverConfig
	serverConfigValue atomic.Value
	serverConfigLock  sync.Mutex

	// consulServersCh is used to receive events related to the
	// maintenance of the list of consulServers
	consulServersCh chan consulServerEventTypes

	// shutdownCh is a copy of the channel in consul.Client
	shutdownCh chan struct{}

	// Logger uses the provided LogOutput
	logger *log.Logger
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
			// Overwrite the existing server parts in order to
			// possibly update metadata (i.e. server version)
			serverCfg.servers[idx] = server
			found = true
			break
		}
	}

	// Add to the list if not known
	if !found {
		newServers := make([]*server_details.ServerDetails, len(serverCfg.servers)+1)
		copy(newServers, serverCfg.servers)
		serverCfg.servers = newServers

		// Notify the server maintenance task of a new server
		sm.consulServersCh <- consulServersNodeJoin
	}

	sm.serverConfigValue.Store(serverCfg)
}

// CycleFailedServers takes out an internal write lock and dequeues all
// failed servers and re-enqueues them.  This method does not reshuffle the
// server list.  Because this changed the order of servers, we push out the
// time at which a rebalance occurs.
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

	serverCfg.resetRebalanceTimer(sm)
	sm.serverConfigValue.Store(serverCfg)
}

// cycleServers returns a new list of servers that has dequeued the first
// server and enqueued it at the end of the list.  cycleServers assumes the
// caller is holding the serverConfigLock.
func (sc *serverConfig) cycleServer() (servers []*server_details.ServerDetails) {
	numServers := len(servers)
	if numServers < 2 {
		// No action required
		return servers
	}

	var dequeuedServer *server_details.ServerDetails
	newServers := make([]*server_details.ServerDetails, len(servers)+1)
	dequeuedServer, newServers = servers[0], servers[1:]
	servers = append(newServers, dequeuedServer)
	return servers
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

// getServerConfig is a convenience method to hide the locking semantics of
// atomic.Value from the caller.
func (sm *ServerManager) getServerConfig() serverConfig {
	return sm.serverConfigValue.Load().(serverConfig)
}

// NewServerManager is the only way to safely create a new ServerManager
// struct.
//
// NOTE(sean@): We can not pass in *consul.Client due to an import cycle
func NewServerManager(logger *log.Logger, shutdownCh chan struct{}) (sm *ServerManager) {
	sm = new(ServerManager)
	// Create the initial serverConfig
	serverCfg := serverConfig{}
	sm.logger = logger
	sm.shutdownCh = shutdownCh
	sm.serverConfigValue.Store(serverCfg)
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

	// Shuffle the server list on server join.  Servers are selected from
	// the head of the list and are moved to the end of the list on
	// failure.
	for i := len(serverCfg.servers) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		serverCfg.servers[i], serverCfg.servers[j] = serverCfg.servers[j], serverCfg.servers[i]
	}

	serverCfg.resetRebalanceTimer(sm)
	sm.serverConfigValue.Store(serverCfg)
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
			serverCfg.servers[i], serverCfg.servers[n-1] = serverCfg.servers[n-1], nil
			serverCfg.servers = serverCfg.servers[:n-1]
			break
		}
	}

	sm.serverConfigValue.Store(serverCfg)
}

// resetRebalanceTimer assumes:
//
// 1) the serverConfigLock is already held by the caller.
// 2) the caller will call serverConfigValue.Store()
func (sc *serverConfig) resetRebalanceTimer(sm *ServerManager) {
	numConsulServers := len(sc.servers)
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numConsulServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers := 16384 // Assume sufficiently large for now. FIXME: numLanMembers := len(c.LANMembers())
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)
	sm.logger.Printf("[DEBUG] consul: connection will be rebalanced in %v", connRebalanceTimeout)

	if sc.rebalanceTimer == nil {
		sc.rebalanceTimer = time.NewTimer(connRebalanceTimeout)
	} else {
		sc.rebalanceTimer.Reset(connRebalanceTimeout)
	}
}

// StartServerManager is used to start and manage the task of automatically
// shuffling and rebalance the list of consul servers.  This maintenance
// happens either when a new server is added or when a duration has been
// exceed.
func (sm *ServerManager) StartServerManager() {
	var rebalanceTimer *time.Timer
	func() {
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()

		serverCfgPtr := sm.serverConfigValue.Load()
		if serverCfgPtr == nil {
			panic("server config has not been initialized")
		}
		var serverCfg serverConfig
		serverCfg = serverCfgPtr.(serverConfig)
		serverCfg.resetRebalanceTimer(sm)
		rebalanceTimer = serverCfg.rebalanceTimer
		sm.serverConfigValue.Store(serverCfg)
	}()

	for {
		select {
		case e := <-sm.consulServersCh:
			switch e {
			case consulServersNodeJoin:
				sm.logger.Printf("[INFO] consul: new node joined cluster")
				sm.RebalanceServers()
			case consulServersRebalance:
				sm.logger.Printf("[INFO] consul: rebalancing servers by request")
				sm.RebalanceServers()
			case consulServersRPCError:
				sm.logger.Printf("[INFO] consul: need to find a new server to talk with")
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
				sm.logger.Printf("[WARN] consul: unhandled LAN Serf Event: %#v", e)
			}
		case <-rebalanceTimer.C:
			sm.logger.Printf("[INFO] consul: server rebalance timeout")
			sm.RebalanceServers()

		case <-sm.shutdownCh:
			return
		}
	}
}
