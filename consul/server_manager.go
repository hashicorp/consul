package consul

import (
	"math/rand"
	"sync/atomic"
	"time"

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

// serverCfg is the thread-safe configuration structure that is used to
// maintain the list of consul servers in Client.
//
// NOTE(sean@): We are explicitly relying on the fact that this is copied.
// Please keep this structure light.
type serverConfig struct {
	// servers tracks the locally known servers
	servers []*serverParts

	// Timer used to control rebalancing of servers
	rebalanceTimer *time.Timer
}

// consulServersManager is used to automatically shuffle and rebalance the
// list of consulServers.  This maintenance happens either when a new server
// is added or when a duration has been exceed.
func (c *Client) consulServersManager() {
	defaultTimeout := 5 * time.Second // FIXME(sean@): This is a bullshit value
	var rebalanceTimer *time.Timer
	func(c *Client) {
		c.serverConfigMtx.Lock()
		defer c.serverConfigMtx.Unlock()

		serverCfgPtr := c.serverConfigValue.Load()
		if serverCfgPtr == nil {
			panic("server config has not been initialized")
		}
		var serverCfg serverConfig
		serverCfg = serverCfgPtr.(serverConfig)
		rebalanceTimer = time.NewTimer(defaultTimeout)
		serverCfg.rebalanceTimer = rebalanceTimer
	}(c)

	for {
		select {
		case e := <-c.consulServersCh:
			switch e {
			case consulServersNodeJoin:
				c.logger.Printf("[INFO] consul: new node joined cluster")
				c.RebalanceServers()
			case consulServersRebalance:
				c.logger.Printf("[INFO] consul: rebalancing servers by request")
				c.RebalanceServers()
			case consulServersRPCError:
				c.logger.Printf("[INFO] consul: need to find a new server to talk with")
				c.CycleFailedServers()
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
				c.logger.Printf("[WARN] consul: unhandled LAN Serf Event: %#v", e)
			}
		case <-rebalanceTimer.C:
			c.logger.Printf("[INFO] consul: server rebalance timeout")
			c.RebalanceServers()

		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Client) AddServer(server *serverParts) {
	c.serverConfigMtx.Lock()
	defer c.serverConfigMtx.Unlock()
	serverCfg := c.serverConfigValue.Load().(serverConfig)

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
		serverCfg.servers = append(serverCfg.servers, server)

		// Notify the server maintenance task of a new server
		c.consulServersCh <- consulServersNodeJoin
	}

	c.serverConfigValue.Store(serverCfg)

}

func (c *Client) CycleFailedServers() {
	c.serverConfigMtx.Lock()
	defer c.serverConfigMtx.Unlock()
	serverCfg := c.serverConfigValue.Load().(serverConfig)

	for i := range serverCfg.servers {
		failCount := atomic.LoadUint64(&(serverCfg.servers[i].Disabled))
		if failCount == 0 {
			break
		} else if failCount > 0 {
			serverCfg.servers = serverCfg.cycleServer()
		}
	}

	serverCfg.resetRebalanceTimer(c)
	c.serverConfigValue.Store(serverCfg)
}

func (sc *serverConfig) cycleServer() (servers []*serverParts) {
	numServers := len(servers)
	if numServers < 2 {
		// No action required for zero or one server situations
		return servers
	}

	var failedNode *serverParts
	failedNode, servers = servers[0], servers[1:]
	servers = append(servers, failedNode)
	return servers
}

func (c *Client) RebalanceServers() {
	c.serverConfigMtx.Lock()
	defer c.serverConfigMtx.Unlock()
	serverCfg := c.serverConfigValue.Load().(serverConfig)

	// Shuffle the server list on server join.  Servers are selected from
	// the head of the list and are moved to the end of the list on
	// failure.
	for i := len(serverCfg.servers) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		serverCfg.servers[i], serverCfg.servers[j] = serverCfg.servers[j], serverCfg.servers[i]
	}

	serverCfg.resetRebalanceTimer(c)
	c.serverConfigValue.Store(serverCfg)
}

func (c *Client) RemoveServer(server *serverParts) {
	c.serverConfigMtx.Lock()
	defer c.serverConfigMtx.Unlock()
	serverCfg := c.serverConfigValue.Load().(serverConfig)

	// Remove the server if known
	n := len(serverCfg.servers)
	for i := 0; i < n; i++ {
		if serverCfg.servers[i].Name == server.Name {
			serverCfg.servers[i], serverCfg.servers[n-1] = serverCfg.servers[n-1], nil
			serverCfg.servers = serverCfg.servers[:n-1]
			break
		}
	}

	c.serverConfigValue.Store(serverCfg)

}

// resetRebalanceTimer assumes:
//
// 1) the serverConfigMtx is already held by the caller.
// 2) the caller will call serverConfigValue.Store()
func (sc *serverConfig) resetRebalanceTimer(c *Client) {
	numConsulServers := len(sc.servers)
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numConsulServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers := len(c.LANMembers())
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)
	c.logger.Printf("[DEBUG] consul: connection will be rebalanced in %v", connRebalanceTimeout)

	sc.rebalanceTimer.Reset(connRebalanceTimeout)
}
