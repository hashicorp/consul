package consul

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
)

const (
	// clientRPCMinReuseDuration controls the minimum amount of time RPC
	// queries are sent over an established connection to a single server
	clientRPCMinReuseDuration = 120 * time.Second

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

	// clientRPCConnMaxIdle controls how long we keep an idle connection
	// open to a server.  127s was chosen as the first prime above 120s
	// (arbitrarily chose to use a prime) with the intent of reusing
	// connections who are used by once-a-minute cron(8) jobs *and* who
	// use a 60s jitter window (e.g. in vixie cron job execution can
	// drift by up to 59s per job, or 119s for a once-a-minute cron job).
	clientRPCConnMaxIdle = 127 * time.Second

	// clientMaxStreams controls how many idle streams we keep
	// open to a server
	clientMaxStreams = 32
)

// Interface is used to provide either a Client or Server,
// both of which can be used to perform certain common
// Consul methods
type Interface interface {
	RPC(method string, args interface{}, reply interface{}) error
	LANMembers() []serf.Member
	LocalMember() serf.Member
}

// Client is Consul client which uses RPC to communicate with the
// services for service discovery, health checking, and DC forwarding.
type Client struct {
	config *Config

	// Connection pool to consul servers
	connPool *ConnPool

	// consuls tracks the locally known servers
	consuls    []*serverParts
	consulLock sync.RWMutex

	// eventCh is used to receive events from the
	// serf cluster in the datacenter
	eventCh chan serf.Event

	// lastServer is the last server we made an RPC call to,
	// this is used to re-use the last connection
	lastServer  *serverParts
	lastRPCTime time.Time

	// connRebalanceTime is the time at which we should change the server
	// we query for RPC requests.
	connRebalanceTime time.Time

	// Logger uses the provided LogOutput
	logger *log.Logger

	// serf is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serf *serf.Serf

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewClient is used to construct a new Consul client from the
// configuration, potentially returning an error
func NewClient(config *Config) (*Client, error) {
	// Check the protocol version
	if err := config.CheckVersion(); err != nil {
		return nil, err
	}

	// Check for a data directory!
	if config.DataDir == "" {
		return nil, fmt.Errorf("Config must provide a DataDir")
	}

	// Sanity check the ACLs
	if err := config.CheckACL(); err != nil {
		return nil, err
	}

	// Ensure we have a log output
	if config.LogOutput == nil {
		config.LogOutput = os.Stderr
	}

	// Create the tls Wrapper
	tlsWrap, err := config.tlsConfig().OutgoingTLSWrapper()
	if err != nil {
		return nil, err
	}

	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create server
	c := &Client{
		config:     config,
		connPool:   NewPool(config.LogOutput, clientRPCConnMaxIdle, clientMaxStreams, tlsWrap),
		eventCh:    make(chan serf.Event, 256),
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}

	// Start the Serf listeners to prevent a deadlock
	go c.lanEventHandler()

	// Initialize the lan Serf
	c.serf, err = c.setupSerf(config.SerfLANConfig,
		c.eventCh, serfLANSnapshot)
	if err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}
	return c, nil
}

// setupSerf is used to setup and initialize a Serf
func (c *Client) setupSerf(conf *serf.Config, ch chan serf.Event, path string) (*serf.Serf, error) {
	conf.Init()
	conf.NodeName = c.config.NodeName
	conf.Tags["role"] = "node"
	conf.Tags["dc"] = c.config.Datacenter
	conf.Tags["vsn"] = fmt.Sprintf("%d", c.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["build"] = c.config.Build
	conf.MemberlistConfig.LogOutput = c.config.LogOutput
	conf.LogOutput = c.config.LogOutput
	conf.EventCh = ch
	conf.SnapshotPath = filepath.Join(c.config.DataDir, path)
	conf.ProtocolVersion = protocolVersionMap[c.config.ProtocolVersion]
	conf.RejoinAfterLeave = c.config.RejoinAfterLeave
	conf.Merge = &lanMergeDelegate{dc: c.config.Datacenter}
	conf.DisableCoordinates = c.config.DisableCoordinates
	if wanAddr := c.config.SerfWANConfig.MemberlistConfig.AdvertiseAddr; wanAddr != "" {
		conf.Tags["wan_addr"] = wanAddr
	}
	if err := ensurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}
	return serf.Create(conf)
}

// Shutdown is used to shutdown the client
func (c *Client) Shutdown() error {
	c.logger.Printf("[INFO] consul: shutting down client")
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		return nil
	}

	c.shutdown = true
	close(c.shutdownCh)

	if c.serf != nil {
		c.serf.Shutdown()
	}

	// Close the connection pool
	c.connPool.Shutdown()
	return nil
}

// Leave is used to prepare for a graceful shutdown
func (c *Client) Leave() error {
	c.logger.Printf("[INFO] consul: client starting leave")

	// Leave the LAN pool
	if c.serf != nil {
		if err := c.serf.Leave(); err != nil {
			c.logger.Printf("[ERR] consul: Failed to leave LAN Serf cluster: %v", err)
		}
	}
	return nil
}

// JoinLAN is used to have Consul client join the inner-DC pool
// The target address should be another node inside the DC
// listening on the Serf LAN address
func (c *Client) JoinLAN(addrs []string) (int, error) {
	return c.serf.Join(addrs, true)
}

// LocalMember is used to return the local node
func (c *Client) LocalMember() serf.Member {
	return c.serf.LocalMember()
}

// LANMembers is used to return the members of the LAN cluster
func (c *Client) LANMembers() []serf.Member {
	return c.serf.Members()
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (c *Client) RemoveFailedNode(node string) error {
	return c.serf.RemoveFailedNode(node)
}

// KeyManagerLAN returns the LAN Serf keyring manager
func (c *Client) KeyManagerLAN() *serf.KeyManager {
	return c.serf.KeyManager()
}

// Encrypted determines if gossip is encrypted
func (c *Client) Encrypted() bool {
	return c.serf.EncryptionEnabled()
}

// lanEventHandler is used to handle events from the lan Serf cluster
func (c *Client) lanEventHandler() {
	for {
		select {
		case e := <-c.eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				c.nodeJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave, serf.EventMemberFailed:
				c.nodeFail(e.(serf.MemberEvent))
			case serf.EventUser:
				c.localEvent(e.(serf.UserEvent))
			case serf.EventMemberUpdate: // Ignore
			case serf.EventMemberReap: // Ignore
			case serf.EventQuery: // Ignore
			default:
				c.logger.Printf("[WARN] consul: unhandled LAN Serf Event: %#v", e)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// nodeJoin is used to handle join events on the serf cluster
func (c *Client) nodeJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			continue
		}
		if parts.Datacenter != c.config.Datacenter {
			c.logger.Printf("[WARN] consul: server %s for datacenter %s has joined wrong cluster",
				m.Name, parts.Datacenter)
			continue
		}
		c.logger.Printf("[INFO] consul: adding server %s", parts)

		// Check if this server is known
		found := false
		c.consulLock.Lock()
		for idx, existing := range c.consuls {
			if existing.Name == parts.Name {
				c.consuls[idx] = parts
				found = true
				break
			}
		}

		// Add to the list if not known
		if !found {
			c.consuls = append(c.consuls, parts)
		}
		c.consulLock.Unlock()

		// Trigger the callback
		if c.config.ServerUp != nil {
			c.config.ServerUp()
		}
	}
}

// nodeFail is used to handle fail events on the serf cluster
func (c *Client) nodeFail(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			continue
		}
		c.logger.Printf("[INFO] consul: removing server %s", parts)

		// Remove the server if known
		c.consulLock.Lock()
		n := len(c.consuls)
		for i := 0; i < n; i++ {
			if c.consuls[i].Name == parts.Name {
				c.consuls[i], c.consuls[n-1] = c.consuls[n-1], nil
				c.consuls = c.consuls[:n-1]
				break
			}
		}
		c.consulLock.Unlock()
	}
}

// localEvent is called when we receive an event on the local Serf
func (c *Client) localEvent(event serf.UserEvent) {
	// Handle only consul events
	if !strings.HasPrefix(event.Name, "consul:") {
		return
	}

	switch name := event.Name; {
	case name == newLeaderEvent:
		c.logger.Printf("[INFO] consul: New leader elected: %s", event.Payload)

		// Trigger the callback
		if c.config.ServerUp != nil {
			c.config.ServerUp()
		}
	case isUserEvent(name):
		event.Name = rawUserEventName(name)
		c.logger.Printf("[DEBUG] consul: user event: %s", event.Name)

		// Trigger the callback
		if c.config.UserEventHandler != nil {
			c.config.UserEventHandler(event)
		}
	default:
		c.logger.Printf("[WARN] consul: Unhandled local event: %v", event)
	}
}

// RPC is used to forward an RPC call to a consul server, or fail if no servers
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Check to make sure we haven't spent too much time querying a
	// single server
	now := time.Now()
	if !c.connRebalanceTime.IsZero() && now.After(c.connRebalanceTime) {
		c.logger.Printf("[DEBUG] consul: connection time to server %s exceeded, rotating server connection", c.lastServer.Addr)
		c.lastServer = nil
	}

	// Allocate these vars on the stack before the goto
	var numConsulServers int
	var clusterWideRebalanceConnsPerSec float64
	var connReuseLowWaterMark time.Duration
	var numLANMembers int

	// Check the last RPC time, continue to reuse cached connection for
	// up to clientRPCMinReuseDuration unless exceeded
	// clientRPCConnMaxIdle
	lastRPCTime := now.Sub(c.lastRPCTime)
	var server *serverParts
	if c.lastServer != nil && lastRPCTime < clientRPCConnMaxIdle {
		server = c.lastServer
		goto TRY_RPC
	}

	// Bail if we can't find any servers
	c.consulLock.RLock()
	numConsulServers = len(c.consuls)
	if numConsulServers == 0 {
		c.consulLock.RUnlock()
		return structs.ErrNoServers
	}

	// Select a random addr
	server = c.consuls[rand.Int31n(int32(numConsulServers))]
	c.consulLock.RUnlock()

	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWaterMark, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec = float64(numConsulServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWaterMark = clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers = len(c.LANMembers())
	c.connRebalanceTime = now.Add(lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWaterMark, numLANMembers))
	c.logger.Printf("[DEBUG] consul: connection to server %s will expire at %v", server.Addr, c.connRebalanceTime)

	// Forward to remote Consul
TRY_RPC:
	if err := c.connPool.RPC(c.config.Datacenter, server.Addr, server.Version, method, args, reply); err != nil {
		c.connRebalanceTime = time.Time{}
		c.lastRPCTime = time.Time{}
		c.lastServer = nil
		return err
	}

	// Cache the last server
	c.lastServer = server
	c.lastRPCTime = now
	return nil
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"consul": map[string]string{
			"server":        "false",
			"known_servers": toString(uint64(len(c.consuls))),
		},
		"serf_lan": c.serf.Stats(),
		"runtime":  runtimeStats(),
	}
	return stats
}

// GetCoordinate returns the network coordinate of the current node, as
// maintained by Serf.
func (c *Client) GetCoordinate() (*coordinate.Coordinate, error) {
	return c.serf.GetCoordinate()
}
