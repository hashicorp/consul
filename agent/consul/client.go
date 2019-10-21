package consul

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"
)

const (
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

	// serfEventBacklog is the maximum number of unprocessed Serf Events
	// that will be held in queue before new serf events block.  A
	// blocking serf event queue is a bad thing.
	serfEventBacklog = 256

	// serfEventBacklogWarning is the threshold at which point log
	// warnings will be emitted indicating a problem when processing serf
	// events.
	serfEventBacklogWarning = 200
)

// Client is Consul client which uses RPC to communicate with the
// services for service discovery, health checking, and DC forwarding.
type Client struct {
	config *Config

	// acls is used to resolve tokens to effective policies
	acls *ACLResolver

	// DEPRECATED (ACL-Legacy-Compat) - Only needed while we support both
	// useNewACLs is a flag to indicate whether we are using the new ACL system
	useNewACLs int32

	// Connection pool to consul servers
	connPool *pool.ConnPool

	// routers is responsible for the selection and maintenance of
	// Consul servers this agent uses for RPC requests
	routers *router.Manager

	// rpcLimiter is used to rate limit the total number of RPCs initiated
	// from an agent.
	rpcLimiter atomic.Value

	// eventCh is used to receive events from the
	// serf cluster in the datacenter
	eventCh chan serf.Event

	// Logger uses the provided LogOutput
	logger hclog.InterceptLogger

	// serf is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serf *serf.Serf

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// embedded struct to hold all the enterprise specific data
	EnterpriseClient

	tlsConfigurator *tlsutil.Configurator
}

// NewClient is used to construct a new Consul client from the configuration,
// potentially returning an error.
// NewClient only used to help setting up a client for testing. Normal code
// exercises NewClientLogger.
func NewClient(config *Config) (*Client, error) {
	c, err := tlsutil.NewConfigurator(config.ToTLSUtilConfig(), nil)
	if err != nil {
		return nil, err
	}
	return NewClientLogger(config, nil, c)
}

func NewClientLogger(config *Config, logger hclog.InterceptLogger, tlsConfigurator *tlsutil.Configurator) (*Client, error) {
	// Check the protocol version
	if err := config.CheckProtocolVersion(); err != nil {
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

	// Create a logger
	if logger == nil {
		logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{
			Level:  hclog.Debug,
			Output: config.LogOutput,
		})
	}

	connPool := &pool.ConnPool{
		SrcAddr:         config.RPCSrcAddr,
		LogOutput:       config.LogOutput,
		MaxTime:         clientRPCConnMaxIdle,
		MaxStreams:      clientMaxStreams,
		TLSConfigurator: tlsConfigurator,
		ForceTLS:        config.VerifyOutgoing,
		Datacenter:      config.Datacenter,
	}

	// Create client
	c := &Client{
		config:          config,
		connPool:        connPool,
		eventCh:         make(chan serf.Event, serfEventBacklog),
		logger:          logger.NamedIntercept(logging.ConsulClient),
		shutdownCh:      make(chan struct{}),
		tlsConfigurator: tlsConfigurator,
	}

	c.rpcLimiter.Store(rate.NewLimiter(config.RPCRate, config.RPCMaxBurst))

	if err := c.initEnterprise(); err != nil {
		c.Shutdown()
		return nil, err
	}

	c.useNewACLs = 0
	aclConfig := ACLResolverConfig{
		Config:      config,
		Delegate:    c,
		Logger:      c.logger,
		AutoDisable: true,
		CacheConfig: clientACLCacheConfig,
		ACLConfig:   newACLConfig(c.logger),
	}
	var err error
	if c.acls, err = NewACLResolver(&aclConfig); err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL resolver: %v", err)
	}

	// Initialize the LAN Serf
	c.serf, err = c.setupSerf(config.SerfLANConfig,
		c.eventCh, serfLANSnapshot)
	if err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}

	if c.acls.ACLsEnabled() {
		go c.monitorACLMode()
	}

	// Start maintenance task for servers
	c.routers = router.New(c.logger, c.shutdownCh, c.serf, c.connPool)
	go c.routers.Start()

	// Start LAN event handlers after the router is complete since the event
	// handlers depend on the router and the router depends on Serf.
	go c.lanEventHandler()

	if err := c.startEnterprise(); err != nil {
		c.Shutdown()
		return nil, err
	}

	return c, nil
}

// Shutdown is used to shutdown the client
func (c *Client) Shutdown() error {
	c.logger.Info("shutting down client")
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

	c.acls.Close()

	return nil
}

// Leave is used to prepare for a graceful shutdown
func (c *Client) Leave() error {
	c.logger.Info("client starting leave")

	// Leave the LAN pool
	if c.serf != nil {
		if err := c.serf.Leave(); err != nil {
			c.logger.Error("Failed to leave LAN Serf cluster", "error", err)
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

// LANMembersAllSegments returns members from all segments.
func (c *Client) LANMembersAllSegments() ([]serf.Member, error) {
	return c.serf.Members(), nil
}

// LANSegmentMembers only returns our own segment's members, because clients
// can't be in multiple segments.
func (c *Client) LANSegmentMembers(segment string) ([]serf.Member, error) {
	if segment == c.config.Segment {
		return c.LANMembers(), nil
	}

	return nil, fmt.Errorf("segment %q not found", segment)
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (c *Client) RemoveFailedNode(node string, prune bool) error {
	if prune {
		return c.serf.RemoveFailedNodePrune(node)
	}
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

// RPC is used to forward an RPC call to a consul server, or fail if no servers
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// This is subtle but we start measuring the time on the client side
	// right at the time of the first request, vs. on the first retry as
	// is done on the server side inside forward(). This is because the
	// servers may already be applying the RPCHoldTimeout up there, so by
	// starting the timer here we won't potentially double up the delay.
	// TODO (slackpad) Plumb a deadline here with a context.
	firstCheck := time.Now()

TRY:
	server := c.routers.FindServer()
	if server == nil {
		return structs.ErrNoServers
	}

	// Enforce the RPC limit.
	metrics.IncrCounter([]string{"client", "rpc"}, 1)
	if !c.rpcLimiter.Load().(*rate.Limiter).Allow() {
		metrics.IncrCounter([]string{"client", "rpc", "exceeded"}, 1)
		return structs.ErrRPCRateExceeded
	}

	// Make the request.
	rpcErr := c.connPool.RPC(c.config.Datacenter, server.ShortName, server.Addr, server.Version, method, server.UseTLS, args, reply)
	if rpcErr == nil {
		return nil
	}

	// Move off to another server, and see if we can retry.
	c.logger.Error("RPC failed to server",
		"method", method,
		"server", server.Addr,
		"error", rpcErr,
	)
	metrics.IncrCounterWithLabels([]string{"client", "rpc", "failed"}, 1, []metrics.Label{{Name: "server", Value: server.Name}})
	c.routers.NotifyFailedServer(server)
	if retry := canRetry(args, rpcErr); !retry {
		return rpcErr
	}

	// We can wait a bit and retry!
	if time.Since(firstCheck) < c.config.RPCHoldTimeout {
		jitter := lib.RandomStagger(c.config.RPCHoldTimeout / jitterFraction)
		select {
		case <-time.After(jitter):
			goto TRY
		case <-c.shutdownCh:
		}
	}
	return rpcErr
}

// SnapshotRPC sends the snapshot request to one of the servers, reading from
// the streaming input and writing to the streaming output depending on the
// operation.
func (c *Client) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer,
	replyFn structs.SnapshotReplyFn) error {
	server := c.routers.FindServer()
	if server == nil {
		return structs.ErrNoServers
	}

	// Enforce the RPC limit.
	metrics.IncrCounter([]string{"client", "rpc"}, 1)
	if !c.rpcLimiter.Load().(*rate.Limiter).Allow() {
		metrics.IncrCounter([]string{"client", "rpc", "exceeded"}, 1)
		return structs.ErrRPCRateExceeded
	}

	// Request the operation.
	var reply structs.SnapshotResponse
	snap, err := SnapshotRPC(c.connPool, c.config.Datacenter, server.ShortName, server.Addr, server.UseTLS, args, in, &reply)
	if err != nil {
		return err
	}
	defer func() {
		if err := snap.Close(); err != nil {
			c.logger.Error("Failed closing snapshot stream", "error", err)
		}
	}()

	// Let the caller peek at the reply.
	if replyFn != nil {
		if err := replyFn(&reply); err != nil {
			return nil
		}
	}

	// Stream the snapshot.
	if out != nil {
		if _, err := io.Copy(out, snap); err != nil {
			return fmt.Errorf("failed to stream snapshot: %v", err)
		}
	}

	return nil
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	numServers := c.routers.NumServers()

	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"consul": map[string]string{
			"server":        "false",
			"known_servers": toString(uint64(numServers)),
		},
		"serf_lan": c.serf.Stats(),
		"runtime":  runtimeStats(),
	}

	if c.ACLsEnabled() {
		if c.UseLegacyACLs() {
			stats["consul"]["acl"] = "legacy"
		} else {
			stats["consul"]["acl"] = "enabled"
		}
	} else {
		stats["consul"]["acl"] = "disabled"
	}

	for outerKey, outerValue := range c.enterpriseStats() {
		if _, ok := stats[outerKey]; ok {
			for innerKey, innerValue := range outerValue {
				stats[outerKey][innerKey] = innerValue
			}
		} else {
			stats[outerKey] = outerValue
		}
	}

	return stats
}

// GetLANCoordinate returns the network coordinate of the current node, as
// maintained by Serf.
func (c *Client) GetLANCoordinate() (lib.CoordinateSet, error) {
	lan, err := c.serf.GetCoordinate()
	if err != nil {
		return nil, err
	}

	cs := lib.CoordinateSet{c.config.Segment: lan}
	return cs, nil
}

// ReloadConfig is used to have the Client do an online reload of
// relevant configuration information
func (c *Client) ReloadConfig(config *Config) error {
	c.rpcLimiter.Store(rate.NewLimiter(config.RPCRate, config.RPCMaxBurst))
	return nil
}
