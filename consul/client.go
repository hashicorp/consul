package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Interface is used to provide either a Client or Server,
// both of which can be used to perform certain common
// Consul methods
type Interface interface {
	RPC(method string, args interface{}, reply interface{}) error
	LANMembers() []serf.Member
}

// Client is Consul client which uses RPC to communicate with the
// services for service discovery, health checking, and DC forwarding.
type Client struct {
	config *Config

	// Connection pool to consul servers
	connPool *ConnPool

	// consuls tracks the locally known servers
	consuls    []net.Addr
	consulLock sync.RWMutex

	// eventCh is used to receive events from the
	// serf cluster in the datacenter
	eventCh chan serf.Event

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
	// Check for a data directory!
	if config.DataDir == "" {
		return nil, fmt.Errorf("Config must provide a DataDir")
	}

	// Ensure we have a log output
	if config.LogOutput == nil {
		config.LogOutput = os.Stderr
	}

	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create server
	c := &Client{
		config:     config,
		connPool:   NewPool(8, 30*time.Second),
		eventCh:    make(chan serf.Event, 256),
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}

	// Start the Serf listeners to prevent a deadlock
	go c.lanEventHandler()

	// Initialize the lan Serf
	var err error
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
	conf.MemberlistConfig.LogOutput = c.config.LogOutput
	conf.LogOutput = c.config.LogOutput
	conf.EventCh = ch
	conf.SnapshotPath = filepath.Join(c.config.DataDir, path)
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
	return c.serf.Join(addrs, false)
}

// LANMembers is used to return the members of the LAN cluster
func (c *Client) LANMembers() []serf.Member {
	return c.serf.Members()
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (c *Client) RemoveFailedNode(node string) error {
	return c.serf.RemoveFailedNode(node)
}

// lanEventHandler is used to handle events from the lan Serf cluster
func (c *Client) lanEventHandler() {
	for {
		select {
		case e := <-c.eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				c.nodeJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave:
				fallthrough
			case serf.EventMemberFailed:
				c.nodeFail(e.(serf.MemberEvent))
			case serf.EventUser:
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

		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: parts.Port}
		c.logger.Printf("[INFO] consul: adding server for datacenter: %s, addr: %s", parts.Datacenter, addr)

		// Check if this server is known
		found := false
		c.consulLock.Lock()
		for _, c := range c.consuls {
			if c.String() == addr.String() {
				found = true
				break
			}
		}

		// Add to the list if not known
		if !found {
			c.consuls = append(c.consuls, addr)
		}
		c.consulLock.Unlock()
	}
}

// nodeFail is used to handle fail events on the serf cluster
func (c *Client) nodeFail(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isConsulServer(m)
		if !ok {
			continue
		}
		var addr net.Addr = &net.TCPAddr{IP: m.Addr, Port: parts.Port}
		c.logger.Printf("[INFO] consul: removing server for datacenter: %s, addr: %s", parts.Datacenter, addr)

		// Remove the server if known
		c.consulLock.Lock()
		n := len(c.consuls)
		for i := 0; i < n; i++ {
			if c.consuls[i].String() == addr.String() {
				c.consuls[i], c.consuls[n-1] = c.consuls[n-1], nil
				c.consuls = c.consuls[:n-1]
				break
			}
		}
		c.consulLock.Unlock()
	}
}

// RPC is used to forward an RPC call to a consul server, or fail if no servers
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Bail if we can't find any servers
	c.consulLock.RLock()
	if len(c.consuls) == 0 {
		c.consulLock.RUnlock()
		return structs.ErrNoServers
	}

	// Select a random addr
	offset := rand.Int31() % int32(len(c.consuls))
	server := c.consuls[offset]
	c.consulLock.RUnlock()

	// Forward to remote Consul
	return c.connPool.RPC(server, method, args, reply)
}
