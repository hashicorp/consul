// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
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

	"github.com/hashicorp/consul/acl"
	rpcRate "github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

var ClientCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"client", "rpc"},
		Help: "Increments whenever a Consul agent makes an RPC request to a Consul server.",
	},
	{
		Name: []string{"client", "rpc", "exceeded"},
		Help: "Increments whenever a Consul agent makes an RPC request to a Consul server gets rate limited by that agent's limits configuration.",
	},
	{
		Name: []string{"client", "rpc", "failed"},
		Help: "Increments whenever a Consul agent makes an RPC request to a Consul server and fails.",
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
	*ACLResolver

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

	// resourceServiceClient is a client for the gRPC Resource Service.
	resourceServiceClient pbresource.ResourceServiceClient
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

	aclConfig := ACLResolverConfig{
		Config:          config.ACLResolverSettings,
		Backend:         &clientACLResolverBackend{Client: c},
		Logger:          c.logger,
		DisableDuration: aclClientDisabledTTL,
		CacheConfig:     clientACLCacheConfig,
		ACLConfig:       newACLConfig(&partitionInfoNoop{}, c.logger),
		Tokens:          deps.Tokens,
	}
	var err error
	if c.ACLResolver, err = NewACLResolver(&aclConfig); err != nil {
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

	conn, err := deps.GRPCConnPool.ClientConn(deps.ConnPool.Datacenter)
	if err != nil {
		c.Shutdown()
		return nil, fmt.Errorf("Failed to get gRPC client connection: %w", err)
	}
	c.resourceServiceClient = pbresource.NewResourceServiceClient(conn)

	// Start LAN event handlers after the router is complete since the event
	// handlers depend on the router and the router depends on Serf.
	go c.lanEventHandler()

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

	c.ACLResolver.Close()

	return nil
}

// Leave is used to prepare for a graceful shutdown.
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

// JoinLAN is used to have Consul join the inner-DC pool The target address
// should be another node inside the DC listening on the Serf LAN address
func (c *Client) JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (int, error) {
	// Partitions definitely have to match.
	if c.config.AgentEnterpriseMeta().PartitionOrDefault() != entMeta.PartitionOrDefault() {
		return 0, fmt.Errorf("target partition %q must match client agent partition %q",
			entMeta.PartitionOrDefault(),
			c.config.AgentEnterpriseMeta().PartitionOrDefault(),
		)
	}
	return c.serf.Join(addrs, true)
}

// AgentLocalMember is used to retrieve the LAN member for the local node.
func (c *Client) AgentLocalMember() serf.Member {
	return c.serf.LocalMember()
}

// LANMembersInAgentPartition returns the LAN members for this agent's
// canonical serf pool. For clients this is the only pool that exists. For
// servers it's the pool in the default segment and the default partition.
func (c *Client) LANMembersInAgentPartition() []serf.Member {
	return c.serf.Members()
}

// LANMembers returns the LAN members for one of:
//
// - the requested partition
// - the requested segment
// - all segments
//
// This is limited to segments and partitions that the node is a member of.
func (c *Client) LANMembers(filter LANMemberFilter) ([]serf.Member, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	// Partitions definitely have to match.
	if c.config.AgentEnterpriseMeta().PartitionOrDefault() != filter.PartitionOrDefault() {
		return nil, fmt.Errorf("partition %q not found", filter.PartitionOrDefault())
	}

	if !filter.AllSegments && filter.Segment != c.config.Segment {
		return nil, fmt.Errorf("segment %q not found", filter.Segment)
	}

	return c.serf.Members(), nil
}

// RemoveFailedNode is used to remove a failed node from the cluster.
func (c *Client) RemoveFailedNode(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	// Partitions definitely have to match.
	if c.config.AgentEnterpriseMeta().PartitionOrDefault() != entMeta.PartitionOrDefault() {
		return fmt.Errorf("client agent in partition %q cannot remove node in different partition %q",
			c.config.AgentEnterpriseMeta().PartitionOrDefault(), entMeta.PartitionOrDefault())
	}
	if !isSerfMember(c.serf, node) {
		return fmt.Errorf("agent: No node found with name '%s'", node)
	}
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
func (c *Client) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	// This is subtle but we start measuring the time on the client side
	// right at the time of the first request, vs. on the first retry as
	// is done on the server side inside forward(). This is because the
	// servers may already be applying the RPCHoldTimeout up there, so by
	// starting the timer here we won't potentially double up the delay.
	// TODO (slackpad) Plumb a deadline here with a context.
	firstCheck := time.Now()
	retryCount := 0
	previousJitter := time.Duration(0)
TRY:
	retryCount++
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
	manager.NotifyFailedServer(server)

	// Use the zero value for RPCInfo if the request doesn't implement RPCInfo
	info, _ := args.(structs.RPCInfo)
	retryableMessages := []error{
		// If we are chunking and it doesn't seem to have completed, try again.
		ErrChunkingResubmit,

		// These rate limit errors are returned before the handler is called, so are
		// safe to retry.
		rpcRate.ErrRetryElsewhere,
	}

	if retry := canRetry(info, rpcErr, firstCheck, c.config, retryableMessages); !retry {
		c.logger.Error("RPC failed to server",
			"method", method,
			"server", server.Addr,
			"error", rpcErr,
		)
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "failed"}, 1, []metrics.Label{{Name: "server", Value: server.Name}})
		return rpcErr
	}

	c.logger.Warn("Retrying RPC to server",
		"method", method,
		"server", server.Addr,
		"error", rpcErr,
	)

	// We can wait a bit and retry!
	jitter := lib.RandomStaggerWithRange(previousJitter, getWaitTime(c.config.RPCHoldTimeout, retryCount))
	previousJitter = jitter

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
			return err
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
		stats["consul"]["acl"] = "enabled"
	} else {
		stats["consul"]["acl"] = "disabled"
	}

	return stats
}

// GetLANCoordinate returns the coordinate of the node in the LAN gossip
// pool.
//
//   - Clients return a single coordinate for the single gossip pool they are
//     in (default, segment, or partition).
//
//   - Servers return one coordinate for their canonical gossip pool (i.e.
//     default partition/segment) and one per segment they are also ancillary
//     members of.
//
// NOTE: servers do not emit coordinates for partitioned gossip pools they
// are ancillary members of.
//
// NOTE: This assumes coordinates are enabled, so check that before calling.
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
	c.connPool.SetRPCClientTimeout(config.RPCClientTimeout)
	return nil
}

func (c *Client) AgentEnterpriseMeta() *acl.EnterpriseMeta {
	return c.config.AgentEnterpriseMeta()
}

func (c *Client) agentSegmentName() string {
	return c.config.Segment
}

func (c *Client) ResourceServiceClient() pbresource.ResourceServiceClient {
	return c.resourceServiceClient
}
