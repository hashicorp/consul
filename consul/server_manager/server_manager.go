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

// Pinger is an interface wrapping client.ConnPool to prevent a
// cyclic import dependency
type Pinger interface {
	PingConsulServer(server *server_details.ServerDetails) (bool, error)
}

// serverConfig is the thread-safe configuration struct used to maintain the
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

	// rebalanceTimer controls the duration of the rebalance interval
	rebalanceTimer *time.Timer

	// shutdownCh is a copy of the channel in consul.Client
	shutdownCh chan struct{}

	logger *log.Logger

	// clusterInfo is used to estimate the approximate number of nodes in
	// a cluster and limit the rate at which it rebalances server
	// connections.  ConsulClusterInfo is an interface that wraps serf.
	clusterInfo ConsulClusterInfo

	// connPoolPinger is used to test the health of a server in the
	// connection pool.  Pinger is an interface that wraps
	// client.ConnPool.
	connPoolPinger Pinger

	// notifyFailedBarrier is acts as a barrier to prevent queuing behind
	// serverConfigLog and acts as a TryLock().
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
	sc := sm.getServerConfig()

	// Check if this server is known
	found := false
	for idx, existing := range sc.servers {
		if existing.Name == server.Name {
			newServers := make([]*server_details.ServerDetails, len(sc.servers))
			copy(newServers, sc.servers)

			// Overwrite the existing server details in order to
			// possibly update metadata (e.g. server version)
			newServers[idx] = server

			sc.servers = newServers
			found = true
			break
		}
	}

	// Add to the list if not known
	if !found {
		newServers := make([]*server_details.ServerDetails, len(sc.servers), len(sc.servers)+1)
		copy(newServers, sc.servers)
		newServers = append(newServers, server)
		sc.servers = newServers
	}

	sm.saveServerConfig(sc)
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

	return newServers
}

// removeServerByKey performs an inline removal of the first matching server
func (sc *serverConfig) removeServerByKey(targetKey *server_details.Key) {
	for i, s := range sc.servers {
		if targetKey.Equal(s.Key()) {
			// Delete the target server
			copy(sc.servers[i:], sc.servers[i+1:])
			sc.servers[len(sc.servers)-1] = nil
			sc.servers = sc.servers[:len(sc.servers)-1]
			return
		}
	}
}

// shuffleServers shuffles the server list in place
func (sc *serverConfig) shuffleServers() {
	newServers := make([]*server_details.ServerDetails, len(sc.servers))
	copy(newServers, sc.servers)

	// Shuffle server list
	for i := len(sc.servers) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		newServers[i], newServers[j] = newServers[j], newServers[i]
	}
	sc.servers = newServers
}

// FindServer takes out an internal "read lock" and searches through the list
// of servers to find a "healthy" server.  If the server is actually
// unhealthy, we rely on Serf to detect this and remove the node from the
// server list.  If the server at the front of the list has failed or fails
// during an RPC call, it is rotated to the end of the list.  If there are no
// servers available, return nil.
func (sm *ServerManager) FindServer() *server_details.ServerDetails {
	sc := sm.getServerConfig()
	numServers := len(sc.servers)
	if numServers == 0 {
		sm.logger.Printf("[WARN] server manager: No servers available")
		return nil
	} else {
		// Return whatever is at the front of the list because it is
		// assumed to be the oldest in the server list (unless -
		// hypothetically - the server list was rotated right after a
		// server was added).
		return sc.servers[0]
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
func New(logger *log.Logger, shutdownCh chan struct{}, clusterInfo ConsulClusterInfo, connPoolPinger Pinger) (sm *ServerManager) {
	sm = new(ServerManager)
	sm.logger = logger
	sm.clusterInfo = clusterInfo       // can't pass *consul.Client: import cycle
	sm.connPoolPinger = connPoolPinger // can't pass *consul.ConnPool: import cycle
	sm.rebalanceTimer = time.NewTimer(clientRPCMinReuseDuration)
	sm.shutdownCh = shutdownCh

	sc := serverConfig{}
	sc.servers = make([]*server_details.ServerDetails, 0)
	sm.saveServerConfig(sc)
	return sm
}

// NotifyFailedServer marks the passed in server as "failed" by rotating it
// to the end of the server list.
func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {
	sc := sm.getServerConfig()

	// If the server being failed is not the first server on the list,
	// this is a noop.  If, however, the server is failed and first on
	// the list, acquire the lock, retest, and take the penalty of moving
	// the server to the end of the list.

	// Only rotate the server list when there is more than one server
	if len(sc.servers) > 1 && sc.servers[0] == server &&
		// Use atomic.CAS to emulate a TryLock().
		atomic.CompareAndSwapInt32(&sm.notifyFailedBarrier, 0, 1) {
		defer atomic.StoreInt32(&sm.notifyFailedBarrier, 0)

		// Grab a lock, retest, and take the hit of cycling the first
		// server to the end.
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()
		sc = sm.getServerConfig()

		if len(sc.servers) > 1 && sc.servers[0] == server {
			sc.servers = sc.cycleServer()
			// FIXME(sean@): Is it worth it to fire off a go routine and
			// PingConsulServer?
			sm.saveServerConfig(sc)
		}
	}
}

// NumServers takes out an internal "read lock" and returns the number of
// servers.  numServers includes both healthy and unhealthy servers.
func (sm *ServerManager) NumServers() (numServers int) {
	sc := sm.getServerConfig()
	numServers = len(sc.servers)
	return numServers
}

// RebalanceServers shuffles the list of servers on this agent.  The server
// at the front of the list is selected for the next RPC.  RPC calls that
// fail for a particular server are rotated to the end of the list.  This
// method reshuffles the list periodically in order to redistribute work
// across all known consul servers (i.e. guarantee that the order of servers
// in the server list isn't positively correlated with the age of a server in
// the consul cluster).  Periodically shuffling the server list prevents
// long-lived clients from fixating on long-lived servers.
//
// Unhealthy servers are removed when serf notices the server has been
// deregistered.  Before the newly shuffled server list is saved, the new
// remote endpoint is tested to ensure its responsive.
func (sm *ServerManager) RebalanceServers() {
FAILED_SERVER_DURING_REBALANCE:
	// Obtain a copy of the server config
	sc := sm.getServerConfig()

	// Early abort if there is no value to shuffling
	if len(sc.servers) < 2 {
		// sm.logger.Printf("[INFO] server manager: can't rebalance with only %d servers", len(sc.servers))
		return
	}

	sc.shuffleServers()

	// Iterate through the shuffled server list to find a healthy server.
	// Don't iterate on the list directly, this loop mutates server the
	// list.
	var foundHealthyServer bool
	for n := len(sc.servers); n > 0; n-- {
		// Always test the first server.  Failed servers are cycled
		// while Serf detects the node has failed.
		selectedServer := sc.servers[0]

		// sm.logger.Printf("[INFO] server manager: Preemptively testing server %s before rebalance", selectedServer.String())
		ok, err := sm.connPoolPinger.PingConsulServer(selectedServer)
		if ok {
			foundHealthyServer = true
			break
		}
		sm.logger.Printf("[DEBUG] server manager: pinging server %s failed: %s", selectedServer.String(), err)

		sc.cycleServer()
	}

	// If no healthy servers were found, sleep and wait for Serf to make
	// the world a happy place again.
	if !foundHealthyServer {
		const backoffDuration = 1 * time.Second
		// sm.logger.Printf("[INFO] server manager: No servers available, sleeping for %v", backoffDuration)

		// Sleep with no locks
		time.Sleep(backoffDuration)
		goto FAILED_SERVER_DURING_REBALANCE
	}

	// Verify that all servers are present. Use an anonymous func to
	// ensure lock is released when exiting the critical section.
	reconcileServerLists := func() bool {
		sm.serverConfigLock.Lock()
		defer sm.serverConfigLock.Unlock()
		tmpServerCfg := sm.getServerConfig()

		type targetServer struct {
			server *server_details.ServerDetails

			//   'b' == both
			//   'o' == original
			//   'n' == new
			state byte
		}
		mergedList := make(map[server_details.Key]*targetServer)
		for _, s := range sc.servers {
			mergedList[*s.Key()] = &targetServer{server: s, state: 'o'}
		}
		for _, s := range tmpServerCfg.servers {
			k := s.Key()
			_, found := mergedList[*k]
			if found {
				mergedList[*k].state = 'b'
			} else {
				mergedList[*k] = &targetServer{server: s, state: 'n'}
			}
		}

		// Ensure the selected server has not been removed by Serf
		selectedServerKey := sc.servers[0].Key()
		if v, found := mergedList[*selectedServerKey]; found && v.state == 'o' {
			return false
		}

		// Add any new servers and remove any old servers
		for k, v := range mergedList {
			switch v.state {
			case 'b':
				// Do nothing, server exists in both
			case 'o':
				// Server has been removed
				sc.removeServerByKey(&k)
			case 'n':
				// Server added
				sc.servers = append(sc.servers, v.server)
			default:
				panic("not implemented")
			}
		}

		sm.saveServerConfig(sc)
		return true
	}

	if !reconcileServerLists() {
		goto FAILED_SERVER_DURING_REBALANCE
	}

	sm.logger.Printf("[DEBUG] server manager: Rebalanced %d servers, next active server is %s", len(sc.servers), sc.servers[0].String())
	return
}

// RemoveServer takes out an internal write lock and removes a server from
// the server list.
func (sm *ServerManager) RemoveServer(server *server_details.ServerDetails) {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	sc := sm.getServerConfig()

	// Remove the server if known
	for i, _ := range sc.servers {
		if sc.servers[i].Name == server.Name {
			newServers := make([]*server_details.ServerDetails, 0, len(sc.servers)-1)
			newServers = append(newServers, sc.servers[:i]...)
			newServers = append(newServers, sc.servers[i+1:]...)
			sc.servers = newServers

			sm.saveServerConfig(sc)
			return
		}
	}
}

// refreshServerRebalanceTimer is only called once sm.rebalanceTimer expires.
func (sm *ServerManager) refreshServerRebalanceTimer() time.Duration {
	sc := sm.getServerConfig()
	numConsulServers := len(sc.servers)
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numConsulServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers := sm.clusterInfo.NumNodes()
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)

	sm.rebalanceTimer.Reset(connRebalanceTimeout)
	return connRebalanceTimeout
}

// ResetRebalanceTimer resets the rebalance timer.  This method primarily
// exists for testing and should not be used directly.
func (sm *ServerManager) ResetRebalanceTimer() {
	sm.serverConfigLock.Lock()
	defer sm.serverConfigLock.Unlock()
	sm.rebalanceTimer.Reset(clientRPCMinReuseDuration)
}

// Start is used to start and manage the task of automatically shuffling and
// rebalancing the list of consul servers.  This maintenance only happens
// periodically based on the expiration of the timer.  Failed servers are
// automatically cycled to the end of the list.  New servers are appended to
// the list.  The order of the server list must be shuffled periodically to
// distribute load across all known and available consul servers.
func (sm *ServerManager) Start() {
	for {
		select {
		case <-sm.rebalanceTimer.C:
			//sm.logger.Printf("[INFO] server manager: Rebalancing server connections")
			sm.RebalanceServers()
			sm.refreshServerRebalanceTimer()

		case <-sm.shutdownCh:
			sm.logger.Printf("[INFO] server manager: shutting down")
			return
		}
	}
}
