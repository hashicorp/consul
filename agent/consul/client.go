package consul

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

var ClientCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"client", "rpc"},
		Help: "Increments whenever a Consul agent in client mode makes an RPC request to a Consul server.",
	},
	{
		Name: []string{"client", "rpc", "exceeded"},
		Help: "Increments whenever a Consul agent in client mode makes an RPC request to a Consul server gets rate limited by that agent's limits configuration.",
	},
	{
		Name: []string{"client", "rpc", "failed"},
		Help: "Increments whenever a Consul agent in client mode makes an RPC request to a Consul server and fails.",
	},
}

const (
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

	// router is responsible for the selection and maintenance of
	// Consul servers this agent uses for RPC requests
	router *router.Router

	// rpcLimiter is used to rate limit the total number of RPCs initiated
	// from an agent.
	rpcLimiter atomic.Value

	// eventCh is used to receive events from the serf cluster in the datacenter
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

// NewClient creates and returns a Client
func NewClient(config *Config, deps Deps) (*Client, error) {
	if err := config.CheckProtocolVersion(); err != nil {
		return nil, err
	}
	if config.DataDir == "" {
		return nil, fmt.Errorf("Config must provide a DataDir")
	}
	if err := config.CheckACL(); err != nil {
		return nil, err
	}

	c := &Client{
		config:          config,
		connPool:        deps.ConnPool,
		eventCh:         make(chan serf.Event, serfEventBacklog),
		logger:          deps.Logger.NamedIntercept(logging.ConsulClient),
		shutdownCh:      make(chan struct{}),
		tlsConfigurator: deps.TLSConfigurator,
	}

	c.rpcLimiter.Store(rate.NewLimiter(config.RPCRateLimit, config.RPCMaxBurst))

	if err := c.initEnterprise(deps); err != nil {
		c.Shutdown()
		return nil, err
	}

	c.useNewACLs = 0
	aclConfig := ACLResolverConfig{
		Config:          config.ACLResolverSettings,
		Delegate:        c,
		Logger:          c.logger,
		DisableDuration: aclClientDisabledTTL,
		CacheConfig:     clientACLCacheConfig,
		ACLConfig:       newACLConfig(c.logger),
		Tokens:          deps.Tokens,
	}
	var err error
	if c.acls, err = NewACLResolver(&aclConfig); err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL resolver: %v", err)
	}

	// Initialize the LAN Serf
	c.serf, err = c.setupSerf(config.SerfLANConfig, c.eventCh, serfLANSnapshot)
	if err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}

	if err := deps.Router.AddArea(types.AreaLAN, c.serf, c.connPool); err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to add LAN area to the RPC router: %w", err)
	}
	c.router = deps.Router

	// Start LAN event handlers after the router is complete since the event
	// handlers depend on the router and the router depends on Serf.
	go c.lanEventHandler()

	// This needs to happen after initializing c.router to prevent a race
	// condition where the router manager is used when the pointer is nil
	if c.acls.ACLsEnabled() {
		go c.monitorACLMode()
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
	manager, server := c.router.FindLANRoute()
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
	rpcErr := c.connPool.RPC(c.config.Datacenter, server.ShortName, server.Addr, method, args, reply)
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
	manager.NotifyFailedServer(server)

	// Use the zero value for RPCInfo if the request doesn't implement RPCInfo
	info, _ := args.(structs.RPCInfo)
	if retry := canRetry(info, rpcErr, firstCheck, c.config); !retry {
		return rpcErr
	}

	// We can wait a bit and retry!
	jitter := lib.RandomStagger(c.config.RPCHoldTimeout / structs.JitterFraction)
	select {
	case <-time.After(jitter):
		goto TRY
	case <-c.shutdownCh:
	}
	return rpcErr
}

// SnapshotRPC sends the snapshot request to one of the servers, reading from
// the streaming input and writing to the streaming output depending on the
// operation.
func (c *Client) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer,
	replyFn structs.SnapshotReplyFn) error {
	manager, server := c.router.FindLANRoute()
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
	snap, err := SnapshotRPC(c.connPool, c.config.Datacenter, server.ShortName, server.Addr, args, in, &reply)
	if err != nil {
		manager.NotifyFailedServer(server)
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
	numServers := c.router.GetLANManager().NumServers()

	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"consul": {
			"server":        "false",
			"known_servers": toString(uint64(numServers)),
		},
		"serf_lan": c.serf.Stats(),
		"runtime":  runtimeStats(),
	}

	if c.config.ACLsEnabled {
		if c.UseLegacyACLs() {
			stats["consul"]["acl"] = "legacy"
		} else {
			stats["consul"]["acl"] = "enabled"
		}
	} else {
		stats["consul"]["acl"] = "disabled"
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
func (c *Client) ReloadConfig(config ReloadableConfig) error {
	c.rpcLimiter.Store(rate.NewLimiter(config.RPCRateLimit, config.RPCMaxBurst))
	return nil
}
