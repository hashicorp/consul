// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	raftwal "github.com/hashicorp/raft-wal"
	walmetrics "github.com/hashicorp/raft-wal/metrics"
	"github.com/hashicorp/raft-wal/verifier"
	"github.com/hashicorp/serf/serf"
	"go.etcd.io/bbolt"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/blockingquery"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/consul/authmethod/ssoauth"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/multilimiter"
	rpcRate "github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/consul/reporting"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/consul/usagemetrics"
	"github.com/hashicorp/consul/agent/consul/wanfed"
	"github.com/hashicorp/consul/agent/consul/xdscapacity"
	aclgrpc "github.com/hashicorp/consul/agent/grpc-external/services/acl"
	"github.com/hashicorp/consul/agent/grpc-external/services/connectca"
	"github.com/hashicorp/consul/agent/grpc-external/services/dataplane"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	resourcegrpc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/services/serverdiscovery"
	agentgrpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/services/subscribe"
	"github.com/hashicorp/consul/agent/hcp"
	logdrop "github.com/hashicorp/consul/agent/log-drop"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/rpc/operator"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/resource/reaper"
	raftstorage "github.com/hashicorp/consul/internal/storage/raft"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/routine"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	cslversion "github.com/hashicorp/consul/version"
)

// NOTE The "consul.client.rpc" and "consul.client.rpc.exceeded" counters are defined in consul/client.go

// These are the protocol versions that Consul can _understand_. These are
// Consul-level protocol versions, that are used to configure the Serf
// protocol versions.
const (
	DefaultRPCProtocol = 2

	ProtocolVersionMin uint8 = 2

	// Version 3 added support for network coordinates but we kept the
	// default protocol version at 2 to ease the transition to this new
	// feature. A Consul agent speaking version 2 of the protocol will
	// attempt to send its coordinates to a server who understands version
	// 3 or greater.
	ProtocolVersion2Compatible = 2

	ProtocolVersionMax = 3
)

const (
	serfLANSnapshot   = "serf/local.snapshot"
	serfWANSnapshot   = "serf/remote.snapshot"
	raftState         = "raft/"
	snapshotsRetained = 2

	// raftLogCacheSize is the maximum number of logs to cache in-memory.
	// This is used to reduce disk I/O for the recently committed entries.
	raftLogCacheSize = 512

	// raftRemoveGracePeriod is how long we wait to allow a RemovePeer
	// to replicate to gracefully leave the cluster.
	raftRemoveGracePeriod = 5 * time.Second

	// serfEventChSize is the size of the buffered channel to get Serf
	// events. If this is exhausted we will block Serf and Memberlist.
	serfEventChSize = 2048

	// reconcileChSize is the size of the buffered channel reconcile updates
	// from Serf with the Catalog. If this is exhausted we will drop updates,
	// and wait for a periodic reconcile.
	reconcileChSize = 256

	LeaderTransferMinVersion = "1.6.0"
)

const (
	aclPolicyReplicationRoutineName       = "ACL policy replication"
	aclRoleReplicationRoutineName         = "ACL role replication"
	aclTokenReplicationRoutineName        = "ACL token replication"
	aclTokenReapingRoutineName            = "acl token reaping"
	caRootPruningRoutineName              = "CA root pruning"
	caRootMetricRoutineName               = "CA root expiration metric"
	caSigningMetricRoutineName            = "CA signing expiration metric"
	configEntryControllersRoutineName     = "config entry controllers"
	configReplicationRoutineName          = "config entry replication"
	federationStateReplicationRoutineName = "federation state replication"
	federationStateAntiEntropyRoutineName = "federation state anti-entropy"
	federationStatePruningRoutineName     = "federation state pruning"
	intentionMigrationRoutineName         = "intention config entry migration"
	secondaryCARootWatchRoutineName       = "secondary CA roots watch"
	intermediateCertRenewWatchRoutineName = "intermediate cert renew watch"
	backgroundCAInitializationRoutineName = "CA initialization"
	virtualIPCheckRoutineName             = "virtual IP version check"
	peeringStreamsRoutineName             = "streaming peering resources"
	peeringDeletionRoutineName            = "peering deferred deletion"
	peeringStreamsMetricsRoutineName      = "metrics for streaming peering resources"
	raftLogVerifierRoutineName            = "raft log verifier"
)

var (
	ErrWANFederationDisabled = fmt.Errorf("WAN Federation is disabled")
)

const (
	PoolKindPartition = "partition"
	PoolKindSegment   = "segment"
)

// raftStore combines LogStore and io.Closer since we need both but have
// multiple LogStore implementations that need closing too.
type raftStore interface {
	raft.LogStore
	io.Closer
}

const requestLimitsBurstMultiplier = 10

var _ blockingquery.FSMServer = (*Server)(nil)

// Server is Consul server which manages the service discovery,
// health checking, DC forwarding, Raft, and multiple Serf pools.
type Server struct {
	// queriesBlocking is a counter that we incr and decr atomically in
	// rpc calls to provide telemetry on how many blocking queries are running.
	// We interact with queriesBlocking atomically, do not move without ensuring it is
	// correctly 64-byte aligned in the struct layout
	queriesBlocking uint64

	// aclConfig is the configuration for the ACL system
	aclConfig *acl.Config

	// acls is used to resolve tokens to effective policies
	*ACLResolver

	aclAuthMethodValidators authmethod.Cache

	// autopilot is the Autopilot instance for this server.
	autopilot *autopilot.Autopilot

	// caManager is used to synchronize CA operations across the leader and RPC functions.
	caManager *CAManager

	// rate limiter to use when signing leaf certificates
	caLeafLimiter connectSignRateLimiter

	// Consul configuration
	config *Config

	// configReplicator is used to manage the leaders replication routines for
	// centralized config
	configReplicator *Replicator

	// federationStateReplicator is used to manage the leaders replication routines for
	// federation states
	federationStateReplicator *Replicator

	// dcSupportsFederationStates is used to determine whether we can
	// replicate federation states or not. All servers in the local
	// DC must be on a version of Consul supporting federation states
	// before this will get enabled.
	dcSupportsFederationStates int32

	// tokens holds ACL tokens initially from the configuration, but can
	// be updated at runtime, so should always be used instead of going to
	// the configuration directly.
	tokens *token.Store

	// Connection pool to other consul servers
	connPool *pool.ConnPool

	// Connection pool to other consul servers using gRPC
	grpcConnPool GRPCClientConner

	// eventChLAN is used to receive events from the
	// serf cluster in the datacenter
	eventChLAN chan serf.Event

	// eventChWAN is used to receive events from the
	// serf cluster that spans datacenters
	eventChWAN chan serf.Event

	// wanMembershipNotifyCh is used to receive notifications that the the
	// serfWAN wan pool may have changed.
	//
	// If this is nil, notification is skipped.
	wanMembershipNotifyCh chan struct{}

	// fsm is the state machine used with Raft to provide
	// strong consistency.
	fsm *fsm.FSM

	// Logger uses the provided LogOutput
	logger  hclog.InterceptLogger
	loggers *loggerStore

	// The raft instance is used among Consul nodes within the DC to protect
	// operations that require strong consistency.
	// the state directly.
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftStore     raftStore
	raftTransport *raft.NetworkTransport
	raftInmem     *raft.InmemStore

	// raftNotifyCh is set up by setupRaft() and ensures that we get reliable leader
	// transition notifications from the Raft layer.
	raftNotifyCh <-chan bool

	// raftStorageBackend is the Raft-backed storage backend for resources.
	raftStorageBackend *raftstorage.Backend

	// reconcileCh is used to pass events from the serf handler
	// into the leader manager, so that the strong state can be
	// updated
	reconcileCh chan serf.Member

	// readyForConsistentReads is used to track when the leader server is
	// ready to serve consistent reads, after it has applied its initial
	// barrier. This is updated atomically.
	readyForConsistentReads int32

	// leaveCh is used to signal that the server is leaving the cluster
	// and trying to shed its RPC traffic onto other Consul servers. This
	// is only ever closed.
	leaveCh chan struct{}

	// externalACLServer serves the ACL service exposed on the external gRPC port.
	// It is also exposed on the internal multiplexed "server" port to enable
	// RPC forwarding.
	externalACLServer *aclgrpc.Server

	// externalConnectCAServer serves the Connect CA service exposed on the external
	// gRPC port. It is also exposed on the internal multiplexed "server" port to
	// enable RPC forwarding.
	externalConnectCAServer *connectca.Server

	// externalGRPCServer has a gRPC server exposed on the dedicated gRPC ports, as
	// opposed to the multiplexed "server" port which is served by grpcHandler.
	externalGRPCServer *grpc.Server

	// router is used to map out Consul servers in the WAN and in Consul
	// Enterprise user-defined areas.
	router *router.Router

	// rpcLimiter is used to rate limit the total number of RPCs initiated
	// from an agent.
	rpcLimiter atomic.Value

	// rpcConnLimiter limits the number of RPC connections from a single source IP
	rpcConnLimiter connlimit.Limiter

	// Listener is used to listen for incoming connections
	Listener    net.Listener
	grpcHandler connHandler
	rpcServer   *rpc.Server

	// incomingRPCLimiter rate-limits incoming net/rpc and gRPC calls.
	incomingRPCLimiter rpcRate.RequestLimitsHandler

	// insecureRPCServer is a RPC server that is configure with
	// IncomingInsecureRPCConfig to allow clients to call AutoEncrypt.Sign
	// to request client certificates. At this point a client doesn't have
	// a client cert and thus cannot present it. This is the only RPC
	// Endpoint that is available at the time of writing.
	insecureRPCServer *rpc.Server

	// rpcRecorder is a middleware component that can emit RPC request metrics.
	rpcRecorder *middleware.RequestRecorder

	// tlsConfigurator holds the agent configuration relevant to TLS and
	// configures everything related to it.
	tlsConfigurator *tlsutil.Configurator

	// serfLAN is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	//
	// - If Network Segments are active, this only contains members in the
	//   default segment.
	//
	// - If Admin Partitions are active, this only contains members in the
	//   default partition.
	//
	serfLAN *serf.Serf

	// serfWAN is the Serf cluster maintained between DC's
	// which SHOULD only consist of Consul servers
	serfWAN                *serf.Serf
	serfWANConfig          *serf.Config
	memberlistTransportWAN wanfed.IngestionAwareTransport
	gatewayLocator         *GatewayLocator

	// serverLookup tracks server consuls in the local datacenter.
	// Used to do leader forwarding and provide fast lookup by server id and address
	serverLookup *ServerLookup

	// grpcLeaderForwarder is notified on leader change in order to keep the grpc
	// resolver up to date.
	grpcLeaderForwarder LeaderForwarder

	// floodLock controls access to floodCh.
	floodLock sync.RWMutex
	floodCh   []chan struct{}

	// sessionTimers track the expiration time of each Session that has
	// a TTL. On expiration, a SessionDestroy event will occur, and
	// destroy the session via standard session destroy processing
	sessionTimers *SessionTimers

	// statsFetcher is used by autopilot to check the status of the other
	// Consul router.
	statsFetcher *StatsFetcher

	// overviewManager is used to periodically update the cluster overview
	// and emit node/service/check health metrics.
	overviewManager *OverviewManager

	// reassertLeaderCh is used to signal the leader loop should re-run
	// leadership actions after a snapshot restore.
	reassertLeaderCh chan chan error

	// tombstoneGC is used to track the pending GC invocations
	// for the KV tombstones
	tombstoneGC *state.TombstoneGC

	// aclReplicationStatus (and its associated lock) provide information
	// about the health of the ACL replication goroutine.
	aclReplicationStatus     structs.ACLReplicationStatus
	aclReplicationStatusLock sync.RWMutex

	// shutdown and the associated members here are used in orchestrating
	// a clean shutdown. The shutdownCh is never written to, only closed to
	// indicate a shutdown has been initiated.
	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// dcSupportsIntentionsAsConfigEntries is used to determine whether we can
	// migrate old intentions into service-intentions config entries. All
	// servers in the local DC must be on a version of Consul supporting
	// service-intentions before this will get enabled.
	dcSupportsIntentionsAsConfigEntries int32

	// Manager to handle starting/stopping go routines when establishing/revoking raft leadership
	leaderRoutineManager *routine.Manager

	// publisher is the EventPublisher to be shared amongst various server components. Events from
	// modifications to the FSM, autopilot and others will flow through here. If in the future we
	// need Events generated outside of the Server and all its components, then we could move
	// this into the Deps struct and created it much earlier on.
	publisher *stream.EventPublisher

	// peeringBackend is shared between the external and internal gRPC services for peering
	peeringBackend *PeeringBackend

	// operatorBackend is shared between the external and internal gRPC services for peering
	operatorBackend *OperatorBackend

	// peerStreamServer is a server used to handle peering streams from external clusters.
	peerStreamServer *peerstream.Server

	// peeringServer handles peering RPC requests internal to this cluster, like generating peering tokens.
	peeringServer *peering.Server

	// xdsCapacityController controls the number of concurrent xDS streams the
	// server is able to handle.
	xdsCapacityController *xdscapacity.Controller

	// hcpManager handles pushing server status updates to the HashiCorp Cloud Platform when enabled
	hcpManager *hcp.Manager

	// embedded struct to hold all the enterprise specific data
	EnterpriseServer
	operatorServer *operator.Server

	// routineManager is responsible for managing longer running go routines
	// run by the Server
	routineManager *routine.Manager

	// typeRegistry contains Consul's registered resource types.
	typeRegistry resource.Registry

	// internalResourceServiceClient is a client that can be used to communicate
	// with the Resource Service in-process (i.e. not via the network) without auth.
	// It should only be used for purely-internal workloads, such as controllers.
	internalResourceServiceClient pbresource.ResourceServiceClient

	// controllerManager schedules the execution of controllers.
	controllerManager *controller.Manager

	// handles metrics reporting to HashiCorp
	reportingManager *reporting.ReportingManager
}

func (s *Server) DecrementBlockingQueries() uint64 {
	return atomic.AddUint64(&s.queriesBlocking, ^uint64(0))
}

func (s *Server) GetShutdownChannel() chan struct{} {
	return s.shutdownCh
}

func (s *Server) IncrementBlockingQueries() uint64 {
	return atomic.AddUint64(&s.queriesBlocking, 1)
}

type connHandler interface {
	Run() error
	Handle(conn net.Conn)
	Shutdown() error
}

// NewServer is used to construct a new Consul server from the configuration
// and extra options, potentially returning an error.
func NewServer(config *Config, flat Deps, externalGRPCServer *grpc.Server, incomingRPCLimiter rpcRate.RequestLimitsHandler, serverLogger hclog.InterceptLogger) (*Server, error) {
	logger := flat.Logger
	if err := config.CheckProtocolVersion(); err != nil {
		return nil, err
	}
	if config.DataDir == "" && !config.DevMode {
		return nil, fmt.Errorf("Config must provide a DataDir")
	}
	if err := config.CheckACL(); err != nil {
		return nil, err
	}

	// Create the tombstone GC.
	gc, err := state.NewTombstoneGC(config.TombstoneTTL, config.TombstoneTTLGranularity)
	if err != nil {
		return nil, err
	}

	// Create the shutdown channel - this is closed but never written to.
	shutdownCh := make(chan struct{})

	loggers := newLoggerStore(serverLogger)

	if incomingRPCLimiter == nil {
		incomingRPCLimiter = rpcRate.NullRequestLimitsHandler()
	}

	// Create server.
	s := &Server{
		config:                  config,
		tokens:                  flat.Tokens,
		connPool:                flat.ConnPool,
		grpcConnPool:            flat.GRPCConnPool,
		eventChLAN:              make(chan serf.Event, serfEventChSize),
		eventChWAN:              make(chan serf.Event, serfEventChSize),
		logger:                  serverLogger,
		loggers:                 loggers,
		leaveCh:                 make(chan struct{}),
		reconcileCh:             make(chan serf.Member, reconcileChSize),
		router:                  flat.Router,
		tlsConfigurator:         flat.TLSConfigurator,
		externalGRPCServer:      externalGRPCServer,
		reassertLeaderCh:        make(chan chan error),
		sessionTimers:           NewSessionTimers(),
		tombstoneGC:             gc,
		serverLookup:            NewServerLookup(),
		shutdownCh:              shutdownCh,
		leaderRoutineManager:    routine.NewManager(logger.Named(logging.Leader)),
		aclAuthMethodValidators: authmethod.NewCache(),
		publisher:               flat.EventPublisher,
		incomingRPCLimiter:      incomingRPCLimiter,
		routineManager:          routine.NewManager(logger.Named(logging.ConsulServer)),
		typeRegistry:            resource.NewRegistry(),
	}
	incomingRPCLimiter.Register(s)

	s.raftStorageBackend, err = raftstorage.NewBackend(&raftHandle{s}, logger.Named("raft-storage-backend"))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage backend: %w", err)
	}
	go s.raftStorageBackend.Run(&lib.StopChannelContext{StopCh: shutdownCh})

	s.fsm = fsm.NewFromDeps(fsm.Deps{
		Logger: flat.Logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(gc, flat.EventPublisher)
		},
		Publisher:      flat.EventPublisher,
		StorageBackend: s.raftStorageBackend,
	})

	s.hcpManager = hcp.NewManager(hcp.ManagerConfig{
		Client:   flat.HCP.Client,
		StatusFn: s.hcpServerStatus(flat),
		Logger:   logger.Named("hcp_manager"),
	})

	var recorder *middleware.RequestRecorder
	if flat.NewRequestRecorderFunc != nil {
		recorder = flat.NewRequestRecorderFunc(serverLogger, s.IsLeader, s.config.Datacenter)
	} else {
		return nil, fmt.Errorf("cannot initialize server without an RPC request recorder provider")
	}
	if recorder == nil {
		return nil, fmt.Errorf("cannot initialize server with a nil RPC request recorder")
	}

	rpcServerOpts := []func(*rpc.Server){
		rpc.WithPreBodyInterceptor(middleware.GetNetRPCRateLimitingInterceptor(s.incomingRPCLimiter, middleware.NewPanicHandler(s.logger))),
	}

	if flat.GetNetRPCInterceptorFunc != nil {
		rpcServerOpts = append(rpcServerOpts, rpc.WithServerServiceCallInterceptor(flat.GetNetRPCInterceptorFunc(recorder)))
	}

	s.rpcServer = rpc.NewServerWithOpts(rpcServerOpts...)
	s.insecureRPCServer = rpc.NewServerWithOpts(rpcServerOpts...)

	s.rpcRecorder = recorder
	s.incomingRPCLimiter.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	go s.publisher.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	if s.config.ConnectMeshGatewayWANFederationEnabled {
		s.gatewayLocator = NewGatewayLocator(
			s.logger,
			s,
			s.config.Datacenter,
			s.config.PrimaryDatacenter,
		)
		s.connPool.GatewayResolver = s.gatewayLocator.PickGateway
		s.grpcConnPool.SetGatewayResolver(s.gatewayLocator.PickGateway)
	}

	// Initialize enterprise specific server functionality
	if err := s.initEnterprise(flat); err != nil {
		s.Shutdown()
		return nil, err
	}

	initLeaderMetrics()

	s.rpcLimiter.Store(rate.NewLimiter(config.RPCRateLimit, config.RPCMaxBurst))

	configReplicatorConfig := ReplicatorConfig{
		Name:     logging.ConfigEntry,
		Delegate: &FunctionReplicator{ReplicateFn: s.replicateConfig, Name: "config-entries"},
		Rate:     s.config.ConfigReplicationRate,
		Burst:    s.config.ConfigReplicationBurst,
		Logger:   s.logger,
	}
	s.configReplicator, err = NewReplicator(&configReplicatorConfig)
	if err != nil {
		s.Shutdown()
		return nil, err
	}

	federationStateReplicatorConfig := ReplicatorConfig{
		Name: logging.FederationState,
		Delegate: &IndexReplicator{
			Delegate: &FederationStateReplicator{
				srv:            s,
				gatewayLocator: s.gatewayLocator,
			},
			Logger: s.loggers.Named(logging.Replication).Named(logging.FederationState),
		},
		Rate:             s.config.FederationStateReplicationRate,
		Burst:            s.config.FederationStateReplicationBurst,
		Logger:           s.logger,
		SuppressErrorLog: isErrFederationStatesNotSupported,
	}
	s.federationStateReplicator, err = NewReplicator(&federationStateReplicatorConfig)
	if err != nil {
		s.Shutdown()
		return nil, err
	}

	// Initialize the stats fetcher that autopilot will use.
	s.statsFetcher = NewStatsFetcher(logger, s.connPool, s.config.Datacenter)

	partitionInfo := serverPartitionInfo(s)
	s.aclConfig = newACLConfig(partitionInfo, logger)
	aclConfig := ACLResolverConfig{
		Config:      config.ACLResolverSettings,
		Backend:     &serverACLResolverBackend{Server: s},
		CacheConfig: serverACLCacheConfig,
		Logger:      logger,
		ACLConfig:   s.aclConfig,
		Tokens:      flat.Tokens,
	}
	// Initialize the ACL resolver.
	if s.ACLResolver, err = NewACLResolver(&aclConfig); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL resolver: %v", err)
	}

	// Initialize the RPC layer.
	if err := s.setupRPC(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize any extra RPC listeners for segments.
	segmentListeners, err := s.setupSegmentRPC()
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start segment RPC layer: %v", err)
	}

	// Initialize the Raft server.
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	s.caManager = NewCAManager(&caDelegateWithState{Server: s}, s.leaderRoutineManager, s.logger.ResetNamed("connect.ca"), s.config)
	if s.config.ConnectEnabled && (s.config.AutoEncryptAllowTLS || s.config.AutoConfigAuthzEnabled) {
		go s.connectCARootsMonitor(&lib.StopChannelContext{StopCh: s.shutdownCh})
	}

	if s.gatewayLocator != nil {
		go s.gatewayLocator.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})
	}

	// Serf and dynamic bind ports
	//
	// The LAN serf cluster announces the port of the WAN serf cluster
	// which creates a race when the WAN cluster is supposed to bind to
	// a dynamic port (port 0). The current memberlist implementation will
	// update the bind port in the configuration after the memberlist is
	// created, so we can pull it out from there reliably, even though it's
	// a little gross to be reading the updated config.

	// Initialize the WAN Serf if enabled
	if config.SerfWANConfig != nil {
		s.serfWAN, s.serfWANConfig, err = s.setupSerf(setupSerfOptions{
			Config:       config.SerfWANConfig,
			EventCh:      s.eventChWAN,
			SnapshotPath: serfWANSnapshot,
			WAN:          true,
			Listener:     s.Listener,
		})
		if err != nil {
			s.Shutdown()
			return nil, fmt.Errorf("Failed to start WAN Serf: %v", err)
		}

		// This is always a *memberlist.NetTransport or something which wraps
		// it which satisfies this interface.
		s.memberlistTransportWAN = config.SerfWANConfig.MemberlistConfig.Transport.(wanfed.IngestionAwareTransport)

		// See big comment above why we are doing this.
		serfBindPortWAN := config.SerfWANConfig.MemberlistConfig.BindPort
		if serfBindPortWAN == 0 {
			serfBindPortWAN = config.SerfWANConfig.MemberlistConfig.BindPort
			if serfBindPortWAN == 0 {
				return nil, fmt.Errorf("Failed to get dynamic bind port for WAN Serf")
			}
			s.logger.Info("Serf WAN TCP bound", "port", serfBindPortWAN)
		}
	}

	// Initialize the LAN segments before the default LAN Serf so we have
	// updated port information to publish there.
	if err := s.setupSegments(config, segmentListeners); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to setup network segments: %v", err)
	}

	// Initialize the LAN Serf for the default network segment.
	if err := s.setupSerfLAN(config); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start LAN Serf: %v", err)
	}

	if err := s.router.AddArea(types.AreaLAN, s.serfLAN, s.connPool); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to add LAN serf route: %w", err)
	}
	go s.lanEventHandler()

	// Start the flooders after the LAN event handler is wired up.
	s.floodSegments(config)

	// Add a "static route" to the WAN Serf and hook it up to Serf events.
	if s.serfWAN != nil {
		if err := s.router.AddArea(types.AreaWAN, s.serfWAN, s.connPool); err != nil {
			s.Shutdown()
			return nil, fmt.Errorf("Failed to add WAN serf route: %v", err)
		}
		go router.HandleSerfEvents(s.logger, s.router, types.AreaWAN, s.serfWAN.ShutdownCh(), s.eventChWAN, s.wanMembershipNotifyCh)

		// Fire up the LAN <-> WAN join flooder.
		addrFn := func(s *metadata.Server) (string, error) {
			if s.WanJoinPort == 0 {
				return "", fmt.Errorf("no wan join  port for server: %s", s.Addr.String())
			}
			addr, _, err := net.SplitHostPort(s.Addr.String())
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s:%d", addr, s.WanJoinPort), nil
		}
		go s.Flood(addrFn, s.serfWAN)
	}

	// Start enterprise specific functionality
	if err := s.startEnterprise(); err != nil {
		s.Shutdown()
		return nil, err
	}

	reporter, err := usagemetrics.NewUsageMetricsReporter(
		new(usagemetrics.Config).
			WithStateProvider(s.fsm).
			WithLogger(s.logger).
			WithDatacenter(s.config.Datacenter).
			WithReportingInterval(s.config.MetricsReportingInterval).
			WithGetMembersFunc(func() []serf.Member {
				members, err := s.lanPoolAllMembers()
				if err != nil {
					return []serf.Member{}
				}

				return members
			}),
	)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start usage metrics reporter: %v", err)
	}
	go reporter.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	s.overviewManager = NewOverviewManager(s.logger, s.fsm, s.config.MetricsReportingInterval)
	go s.overviewManager.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	s.reportingManager = reporting.NewReportingManager(s.logger, getEnterpriseReportingDeps(flat), s, s.fsm.State())
	go s.reportingManager.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	// Initialize external gRPC server
	s.setupExternalGRPC(config, logger)

	// Initialize internal gRPC server.
	//
	// Note: some "external" gRPC services are also exposed on the internal gRPC server
	// to enable RPC forwarding.
	s.grpcHandler = newGRPCHandlerFromConfig(flat, config, s)
	s.grpcLeaderForwarder = flat.LeaderForwarder

	if err := s.setupInternalResourceService(logger); err != nil {
		return nil, err
	}
	s.controllerManager = controller.NewManager(
		s.internalResourceServiceClient,
		logger.Named(logging.ControllerRuntime),
	)
	s.registerResources()
	go s.controllerManager.Run(&lib.StopChannelContext{StopCh: shutdownCh})

	go s.trackLeaderChanges()

	s.xdsCapacityController = xdscapacity.NewController(xdscapacity.Config{
		Logger:         s.logger.Named(logging.XDSCapacityController),
		GetStore:       func() xdscapacity.Store { return s.fsm.State() },
		SessionLimiter: flat.XDSStreamLimiter,
	})
	go s.xdsCapacityController.Run(&lib.StopChannelContext{StopCh: s.shutdownCh})

	// Initialize Autopilot. This must happen before starting leadership monitoring
	// as establishing leadership could attempt to use autopilot and cause a panic.
	s.initAutopilot(config)

	// Start monitoring leadership. This must happen after Serf is set up
	// since it can fire events when leadership is obtained.
	go s.monitorLeadership()

	// Start listening for RPC requests.
	go func() {
		if err := s.grpcHandler.Run(); err != nil {
			s.logger.Error("gRPC server failed", "error", err)
		}
	}()
	go s.listen(s.Listener)

	// Start listeners for any segments with separate RPC listeners.
	for _, listener := range segmentListeners {
		go s.listen(listener)
	}

	// start autopilot - this must happen after the RPC listeners get setup
	// or else it may block
	s.autopilot.Start(&lib.StopChannelContext{StopCh: s.shutdownCh})

	// Start the metrics handlers.
	go s.updateMetrics()

	// Now we are setup, configure the HCP manager
	go s.hcpManager.Run(&lib.StopChannelContext{StopCh: shutdownCh})

	err = s.runEnterpriseRateLimiterConfigEntryController()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) registerResources() {
	catalog.RegisterTypes(s.typeRegistry)
	catalog.RegisterControllers(s.controllerManager, catalog.DefaultControllerDependencies())

	mesh.RegisterTypes(s.typeRegistry)
	reaper.RegisterControllers(s.controllerManager)

	if s.config.DevMode {
		demo.RegisterTypes(s.typeRegistry)
		demo.RegisterControllers(s.controllerManager)
	}
}

func newGRPCHandlerFromConfig(deps Deps, config *Config, s *Server) connHandler {
	if s.peeringBackend == nil {
		panic("peeringBackend is required during construction")
	}

	p := peering.NewServer(peering.Config{
		Backend: s.peeringBackend,
		Tracker: s.peerStreamServer.Tracker,
		Logger:  deps.Logger.Named("grpc-api.peering"),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to generate peering tokens cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		Datacenter:     config.Datacenter,
		ConnectEnabled: config.ConnectEnabled,
		PeeringEnabled: config.PeeringEnabled,
		Locality:       config.Locality,
		FSMServer:      s,
	})
	s.peeringServer = p
	o := operator.NewServer(operator.Config{
		Backend: s.operatorBackend,
		Logger:  deps.Logger.Named("grpc-api.operator"),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to transfer leader cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		Datacenter: config.Datacenter,
	})
	s.operatorServer = o

	register := func(srv *grpc.Server) {
		if config.RPCConfig.EnableStreaming {
			pbsubscribe.RegisterStateChangeSubscriptionServer(srv, subscribe.NewServer(
				&subscribeBackend{srv: s, connPool: deps.GRPCConnPool},
				deps.Logger.Named("grpc-api.subscription")))
		}
		s.peeringServer.Register(srv)
		s.operatorServer.Register(srv)
		s.registerEnterpriseGRPCServices(deps, srv)

		// Note: these external gRPC services are also exposed on the internal server to
		// enable RPC forwarding.
		s.peerStreamServer.Register(srv)
		s.externalACLServer.Register(srv)
		s.externalConnectCAServer.Register(srv)
	}

	return agentgrpc.NewHandler(deps.Logger, config.RPCAddr, register, nil, s.incomingRPCLimiter)
}

func (s *Server) connectCARootsMonitor(ctx context.Context) {
	for {
		ws := memdb.NewWatchSet()
		state := s.fsm.State()
		ws.Add(state.AbandonCh())
		_, cas, err := state.CARoots(ws)
		if err != nil {
			s.logger.Error("Failed to watch AutoEncrypt CARoot", "error", err)
			return
		}
		caPems := []string{}
		for _, ca := range cas {
			caPems = append(caPems, ca.RootCert)
		}
		if err := s.tlsConfigurator.UpdateAutoTLSCA(caPems); err != nil {
			s.logger.Error("Failed to update AutoEncrypt CAPems", "error", err)
		}

		if err := ws.WatchCtx(ctx); err == context.Canceled {
			s.logger.Info("shutting down Connect CA roots monitor")
			return
		}
	}
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we have an unclean exit then attempt to close the Raft store.
	defer func() {
		if s.raft == nil && s.raftStore != nil {
			if err := s.raftStore.Close(); err != nil {
				s.logger.Error("failed to close Raft store", "error", err)
			}
		}
	}()

	var serverAddressProvider raft.ServerAddressProvider = nil
	if s.config.RaftConfig.ProtocolVersion >= 3 { // ServerAddressProvider needs server ids to work correctly, which is only supported in protocol version 3 or higher
		serverAddressProvider = s.serverLookup
	}

	// Create a transport layer.
	transConfig := &raft.NetworkTransportConfig{
		Stream:                s.raftLayer,
		MaxPool:               3,
		Timeout:               10 * time.Second,
		ServerAddressProvider: serverAddressProvider,
		Logger:                s.loggers.Named(logging.Raft),
	}

	trans := raft.NewNetworkTransportWithConfig(transConfig)
	s.raftTransport = trans
	s.config.RaftConfig.Logger = s.loggers.Named(logging.Raft)

	// Versions of the Raft protocol below 3 require the LocalID to match the network
	// address of the transport.
	s.config.RaftConfig.LocalID = raft.ServerID(trans.LocalAddr())
	if s.config.RaftConfig.ProtocolVersion >= 3 {
		s.config.RaftConfig.LocalID = raft.ServerID(s.config.NodeID)
	}

	// Build an all in-memory setup for dev mode, otherwise prepare a full
	// disk-based setup.
	var log raft.LogStore
	var stable raft.StableStore
	var snap raft.SnapshotStore
	if s.config.DevMode {
		store := raft.NewInmemStore()
		s.raftInmem = store
		stable = store
		log = store
		snap = raft.NewInmemSnapshotStore()
	} else {
		// Create the base raft path.
		path := filepath.Join(s.config.DataDir, raftState)
		if err := lib.EnsurePath(path, true); err != nil {
			return err
		}

		boltDBFile := filepath.Join(path, "raft.db")
		boltFileExists, err := fileExists(boltDBFile)
		if err != nil {
			return fmt.Errorf("failed trying to see if raft.db exists not sure how to continue: %w", err)
		}

		// Only use WAL if there is no existing raft.db, even if it's enabled.
		if s.config.LogStoreConfig.Backend == LogStoreBackendWAL && !boltFileExists {
			walDir := filepath.Join(path, "wal")
			if err := os.MkdirAll(walDir, 0755); err != nil {
				return err
			}

			mc := walmetrics.NewGoMetricsCollector([]string{"raft", "wal"}, nil, nil)

			wal, err := raftwal.Open(walDir,
				raftwal.WithSegmentSize(s.config.LogStoreConfig.WAL.SegmentSize),
				raftwal.WithMetricsCollector(mc),
			)
			if err != nil {
				return fmt.Errorf("fail to open write-ahead-log: %w", err)
			}

			s.raftStore = wal
			log = wal
			stable = wal
		} else {
			if s.config.LogStoreConfig.Backend == LogStoreBackendWAL {
				// User configured the new storage, but still has old raft.db. Warn
				// them!
				s.logger.Warn("BoltDB file raft.db found, IGNORING raft_logstore.backend which is set to 'wal'")
			}

			// Create the backend raft store for logs and stable storage.
			store, err := raftboltdb.New(raftboltdb.Options{
				BoltOptions: &bbolt.Options{
					NoFreelistSync: s.config.LogStoreConfig.BoltDB.NoFreelistSync,
				},
				Path: boltDBFile,
			})
			if err != nil {
				return err
			}
			s.raftStore = store
			log = store
			stable = store

			// start publishing boltdb metrics
			go store.RunMetrics(&lib.StopChannelContext{StopCh: s.shutdownCh}, 0)
		}

		// See if log verification is enabled
		if s.config.LogStoreConfig.Verification.Enabled {
			mc := walmetrics.NewGoMetricsCollector([]string{"raft", "logstore", "verifier"}, nil, nil)
			reportFn := makeLogVerifyReportFn(s.logger.Named("raft.logstore.verifier"))
			verifier := verifier.NewLogStore(log, isLogVerifyCheckpoint, reportFn, mc)
			s.raftStore = verifier
			log = verifier
		}

		// Wrap the store in a LogCache to improve performance.
		cacheStore, err := raft.NewLogCache(raftLogCacheSize, log)
		if err != nil {
			return err
		}
		log = cacheStore

		// Create the snapshot store.
		snapshots, err := raft.NewFileSnapshotStoreWithLogger(path, snapshotsRetained, s.logger.Named("raft.snapshot"))
		if err != nil {
			return err
		}
		snap = snapshots

		// For an existing cluster being upgraded to the new version of
		// Raft, we almost never want to run recovery based on the old
		// peers.json file. We create a peers.info file with a helpful
		// note about where peers.json went, and use that as a sentinel
		// to avoid ingesting the old one that first time (if we have to
		// create the peers.info file because it's not there, we also
		// blow away any existing peers.json file).
		peersFile := filepath.Join(path, "peers.json")
		peersInfoFile := filepath.Join(path, "peers.info")
		if _, err := os.Stat(peersInfoFile); os.IsNotExist(err) {
			if err := os.WriteFile(peersInfoFile, []byte(peersInfoContent), 0755); err != nil {
				return fmt.Errorf("failed to write peers.info file: %v", err)
			}

			// Blow away the peers.json file if present, since the
			// peers.info sentinel wasn't there.
			if _, err := os.Stat(peersFile); err == nil {
				if err := os.Remove(peersFile); err != nil {
					return fmt.Errorf("failed to delete peers.json, please delete manually (see peers.info for details): %v", err)
				}
				s.logger.Info("deleted peers.json file (see peers.info for details)")
			}
		} else if _, err := os.Stat(peersFile); err == nil {
			s.logger.Info("found peers.json file, recovering Raft configuration...")

			var configuration raft.Configuration
			if s.config.RaftConfig.ProtocolVersion < 3 {
				configuration, err = raft.ReadPeersJSON(peersFile)
			} else {
				configuration, err = raft.ReadConfigJSON(peersFile)
			}
			if err != nil {
				return fmt.Errorf("recovery failed to parse peers.json: %v", err)
			}

			// It's safe to pass nil as the handle argument here because we won't call
			// the backend's data access methods (only Apply, Snapshot, and Restore).
			backend, err := raftstorage.NewBackend(nil, hclog.NewNullLogger())
			if err != nil {
				return fmt.Errorf("recovery failed: %w", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go backend.Run(ctx)

			tmpFsm := fsm.NewFromDeps(fsm.Deps{
				Logger: s.logger,
				NewStateStore: func() *state.Store {
					return state.NewStateStore(s.tombstoneGC)
				},
				StorageBackend: backend,
			})
			if err := raft.RecoverCluster(s.config.RaftConfig, tmpFsm,
				log, stable, snap, trans, configuration); err != nil {
				return fmt.Errorf("recovery failed: %v", err)
			}

			if err := os.Remove(peersFile); err != nil {
				return fmt.Errorf("recovery failed to delete peers.json, please delete manually (see peers.info for details): %v", err)
			}
			s.logger.Info("deleted peers.json file after successful recovery")
		}
	}

	// If we are in bootstrap or dev mode and the state is clean then we can
	// bootstrap now.
	if (s.config.Bootstrap || s.config.DevMode) && !s.config.ReadReplica {
		hasState, err := raft.HasExistingState(log, stable, snap)
		if err != nil {
			return err
		}
		if !hasState {
			configuration := raft.Configuration{
				Servers: []raft.Server{
					{
						ID:      s.config.RaftConfig.LocalID,
						Address: trans.LocalAddr(),
					},
				},
			}
			if err := raft.BootstrapCluster(s.config.RaftConfig,
				log, stable, snap, trans, configuration); err != nil {
				return err
			}
		}
	}

	// Set up a channel for reliable leader notifications.
	raftNotifyCh := make(chan bool, 10)
	s.config.RaftConfig.NotifyCh = raftNotifyCh
	s.raftNotifyCh = raftNotifyCh

	// Setup the Raft store.
	var err error
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm.ChunkingFSM(), log, stable, snap, trans)
	return err
}

// endpointFactory is a function that returns an RPC endpoint bound to the given
// server.
type factory func(s *Server) interface{}

// endpoints is a list of registered RPC endpoint factories.
var endpoints []factory

// registerEndpoint registers a new RPC endpoint factory.
func registerEndpoint(fn factory) {
	endpoints = append(endpoints, fn)
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC() error {
	s.rpcConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: s.config.RPCMaxConnsPerClient,
	})

	for _, fn := range endpoints {
		s.rpcServer.Register(fn(s))
	}

	// Only register AutoEncrypt on the insecure RPC server. Insecure only
	// means that verify incoming is turned off even though it might have
	// been configured.
	s.insecureRPCServer.Register(&AutoEncrypt{srv: s})

	// Setup the AutoConfig JWT Authorizer
	var authz AutoConfigAuthorizer
	if s.config.AutoConfigAuthzEnabled {
		// create the auto config authorizer from the JWT authmethod
		validator, err := ssoauth.NewValidator(s.logger, &s.config.AutoConfigAuthzAuthMethod)
		if err != nil {
			return fmt.Errorf("Failed to initialize JWT Auto Config Authorizer: %w", err)
		}

		authz = &jwtAuthorizer{
			validator:       validator,
			allowReuse:      s.config.AutoConfigAuthzAllowReuse,
			claimAssertions: s.config.AutoConfigAuthzClaimAssertions,
		}
	} else {
		// This authorizer always returns that the endpoint is disabled
		authz = &disabledAuthorizer{}
	}
	// now register with the insecure RPC server
	s.insecureRPCServer.Register(NewAutoConfig(s.config, s.tlsConfigurator, autoConfigBackend{Server: s}, authz))

	ln, err := net.ListenTCP("tcp", s.config.RPCAddr)
	if err != nil {
		return err
	}
	s.Listener = ln

	if s.config.NotifyListen != nil {
		s.config.NotifyListen()
	}
	// todo(fs): we should probably guard this
	if s.config.RPCAdvertise == nil {
		s.config.RPCAdvertise = ln.Addr().(*net.TCPAddr)
	}

	// Verify that we have a usable advertise address
	if s.config.RPCAdvertise.IP.IsUnspecified() {
		ln.Close()
		return fmt.Errorf("RPC advertise address is not advertisable: %v", s.config.RPCAdvertise)
	}

	// TODO (hans) switch NewRaftLayer to tlsConfigurator

	// Provide a DC specific wrapper. Raft replication is only
	// ever done in the same datacenter, so we can provide it as a constant.
	wrapper := tlsutil.SpecificDC(s.config.Datacenter, s.tlsConfigurator.OutgoingRPCWrapper())

	// Define a callback for determining whether to wrap a connection with TLS
	tlsFunc := func(address raft.ServerAddress) bool {
		// raft only talks to its own datacenter
		return s.tlsConfigurator.UseTLS(s.config.Datacenter)
	}
	s.raftLayer = NewRaftLayer(s.config.RPCSrcAddr, s.config.RPCAdvertise, wrapper, tlsFunc)
	return nil
}

// Initialize and register services on external gRPC server.
func (s *Server) setupExternalGRPC(config *Config, logger hclog.Logger) {
	s.externalACLServer = aclgrpc.NewServer(aclgrpc.Config{
		ACLsEnabled: s.config.ACLsEnabled,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		InPrimaryDatacenter: s.InPrimaryDatacenter(),
		LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, aclgrpc.Validator, error) {
			return s.loadAuthMethod(methodName, entMeta)
		},
		LocalTokensEnabled:        s.LocalTokensEnabled,
		Logger:                    logger.Named("grpc-api.acl"),
		NewLogin:                  func() aclgrpc.Login { return s.aclLogin() },
		NewTokenWriter:            func() aclgrpc.TokenWriter { return s.aclTokenWriter() },
		PrimaryDatacenter:         s.config.PrimaryDatacenter,
		ValidateEnterpriseRequest: s.validateEnterpriseRequest,
	})
	s.externalACLServer.Register(s.externalGRPCServer)

	s.externalConnectCAServer = connectca.NewServer(connectca.Config{
		Publisher:   s.publisher,
		GetStore:    func() connectca.StateStore { return s.FSM().State() },
		Logger:      logger.Named("grpc-api.connect-ca"),
		ACLResolver: s.ACLResolver,
		CAManager:   s.caManager,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		ConnectEnabled: s.config.ConnectEnabled,
	})
	s.externalConnectCAServer.Register(s.externalGRPCServer)

	dataplane.NewServer(dataplane.Config{
		GetStore:    func() dataplane.StateStore { return s.FSM().State() },
		Logger:      logger.Named("grpc-api.dataplane"),
		ACLResolver: s.ACLResolver,
		Datacenter:  s.config.Datacenter,
	}).Register(s.externalGRPCServer)

	serverdiscovery.NewServer(serverdiscovery.Config{
		Publisher:   s.publisher,
		ACLResolver: s.ACLResolver,
		Logger:      logger.Named("grpc-api.server-discovery"),
	}).Register(s.externalGRPCServer)

	s.peeringBackend = NewPeeringBackend(s)
	s.operatorBackend = NewOperatorBackend(s)

	s.peerStreamServer = peerstream.NewServer(peerstream.Config{
		Backend:        s.peeringBackend,
		GetStore:       func() peerstream.StateStore { return s.FSM().State() },
		Logger:         logger.Named("grpc-api.peerstream"),
		ACLResolver:    s.ACLResolver,
		Datacenter:     s.config.Datacenter,
		ConnectEnabled: s.config.ConnectEnabled,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to generate peering tokens cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
	})
	s.peerStreamServer.Register(s.externalGRPCServer)

	resourcegrpc.NewServer(resourcegrpc.Config{
		Registry:    s.typeRegistry,
		Backend:     s.raftStorageBackend,
		ACLResolver: s.ACLResolver,
		Logger:      logger.Named("grpc-api.resource"),
	}).Register(s.externalGRPCServer)
}

func (s *Server) setupInternalResourceService(logger hclog.Logger) error {
	server := grpc.NewServer()

	resourcegrpc.NewServer(resourcegrpc.Config{
		Registry:    s.typeRegistry,
		Backend:     s.raftStorageBackend,
		ACLResolver: resolver.DANGER_NO_AUTH{},
		Logger:      logger.Named("grpc-api.resource"),
	}).Register(server)

	pipe := agentgrpc.NewPipeListener()
	go server.Serve(pipe)

	go func() {
		<-s.shutdownCh
		server.Stop()
	}()

	conn, err := grpc.Dial("",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(pipe.DialContext),
		grpc.WithBlock(),
	)
	if err != nil {
		server.Stop()
		return err
	}
	go func() {
		<-s.shutdownCh
		conn.Close()
	}()
	s.internalResourceServiceClient = pbresource.NewResourceServiceClient(conn)

	return nil
}

// Shutdown is used to shutdown the server
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down server")
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()

	if s.shutdown {
		return nil
	}

	s.shutdown = true
	close(s.shutdownCh)

	// ensure that any leader routines still running get canceled
	if s.leaderRoutineManager != nil {
		s.leaderRoutineManager.StopAll()
	}

	s.shutdownSerfLAN()

	if s.serfWAN != nil {
		s.serfWAN.Shutdown()
		if err := s.router.RemoveArea(types.AreaWAN); err != nil {
			s.logger.Warn("error removing WAN area", "error", err)
		}
	}
	s.router.Shutdown()

	// TODO: actually shutdown areas?

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Warn("error shutting down raft", "error", err)
		}
		if s.raftStore != nil {
			s.raftStore.Close()
		}
	}

	if s.Listener != nil {
		s.Listener.Close()
	}

	if s.grpcHandler != nil {
		if err := s.grpcHandler.Shutdown(); err != nil {
			s.logger.Warn("failed to stop gRPC server", "error", err)
		}
	}

	// Close the connection pool
	if s.connPool != nil {
		s.connPool.Shutdown()
	}

	if s.ACLResolver != nil {
		s.ACLResolver.Close()
	}

	if s.fsm != nil {
		s.fsm.State().Abandon()
	}

	return nil
}

func (s *Server) attemptLeadershipTransfer(id raft.ServerID) (err error) {
	var addr raft.ServerAddress
	if id != "" {
		addr, err = s.serverLookup.ServerAddr(id)
		if err != nil {
			return err
		}
		future := s.raft.LeadershipTransferToServer(id, addr)
		if err := future.Error(); err != nil {
			return err
		}
	} else {
		future := s.raft.LeadershipTransfer()
		if err := future.Error(); err != nil {
			return err
		}
	}

	return nil
}

// Leave is used to prepare for a graceful shutdown.
func (s *Server) Leave() error {
	s.logger.Info("server starting leave")

	// Check the number of known peers
	numPeers, err := s.autopilot.NumVoters()
	if err != nil {
		s.logger.Error("failed to check raft peers", "error", err)
		return err
	}

	addr := s.raftTransport.LocalAddr()

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemoveServer/RemovePeer to safely reduce the quorum size.
	// If we are not the leader, then we should issue our leave intention and wait to be
	// removed for some reasonable period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 1 {
		if err := s.attemptLeadershipTransfer(""); err == nil {
			isLeader = false
		} else {
			future := s.raft.RemoveServer(raft.ServerID(s.config.NodeID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove ourself as raft peer", "error", err)
			}
		}
	}

	// Leave the WAN pool
	if s.serfWAN != nil {
		if err := s.serfWAN.Leave(); err != nil {
			s.logger.Error("failed to leave WAN Serf cluster", "error", err)
		}
	}

	// Leave the LAN pool
	if s.serfLAN != nil {
		if err := s.serfLAN.Leave(); err != nil {
			s.logger.Error("failed to leave LAN Serf cluster", "error", err)
		}
	}

	// Leave everything enterprise related as well
	s.handleEnterpriseLeave()

	// Start refusing RPCs now that we've left the LAN pool. It's important
	// to do this *after* we've left the LAN pool so that clients will know
	// to shift onto another server if they perform a retry. We also wake up
	// all queries in the RPC retry state.
	s.logger.Info("Waiting to drain RPC traffic", "drain_time", s.config.LeaveDrainTime)
	close(s.leaveCh)
	time.Sleep(s.config.LeaveDrainTime)

	// If we were not leader, wait to be safely removed from the cluster. We
	// must wait to allow the raft replication to take place, otherwise an
	// immediate shutdown could cause a loss of quorum.
	if !isLeader {
		left := false
		limit := time.Now().Add(raftRemoveGracePeriod)
		for !left && time.Now().Before(limit) {
			// Sleep a while before we check.
			time.Sleep(50 * time.Millisecond)

			// Get the latest configuration.
			future := s.raft.GetConfiguration()
			if err := future.Error(); err != nil {
				s.logger.Error("failed to get raft configuration", "error", err)
				break
			}

			// See if we are no longer included.
			left = true
			for _, server := range future.Configuration().Servers {
				if server.Address == addr {
					left = false
					break
				}
			}
		}

		// TODO (slackpad) With the old Raft library we used to force the
		// peers set to empty when a graceful leave occurred. This would
		// keep voting spam down if the server was restarted, but it was
		// dangerous because the peers was inconsistent with the logs and
		// snapshots, so it wasn't really safe in all cases for the server
		// to become leader. This is now safe, but the log spam is noisy.
		// The next new version of the library will have a "you are not a
		// peer stop it" behavior that should address this. We will have
		// to evaluate during the RC period if this interim situation is
		// not too confusing for operators.

		// TODO (slackpad) When we take a later new version of the Raft
		// library it won't try to complete replication, so this peer
		// may not realize that it has been removed. Need to revisit this
		// and the warning here.
		if !left {
			s.logger.Warn("failed to leave raft configuration gracefully, timeout")
		}
	}

	return nil
}

// JoinWAN is used to have Consul join the cross-WAN Consul ring
// The target address should be another node listening on the
// Serf WAN address
func (s *Server) JoinWAN(addrs []string) (int, error) {
	if s.serfWAN == nil {
		return 0, ErrWANFederationDisabled
	}

	if err := s.enterpriseValidateJoinWAN(); err != nil {
		return 0, err
	}

	return s.serfWAN.Join(addrs, true)
}

// PrimaryMeshGatewayAddressesReadyCh returns a channel that will be closed
// when federation state replication ships back at least one primary mesh
// gateway (not via fallback config).
func (s *Server) PrimaryMeshGatewayAddressesReadyCh() <-chan struct{} {
	if s.gatewayLocator == nil {
		return nil
	}
	return s.gatewayLocator.PrimaryMeshGatewayAddressesReadyCh()
}

// PickRandomMeshGatewaySuitableForDialing is a convenience function used for writing tests.
func (s *Server) PickRandomMeshGatewaySuitableForDialing(dc string) string {
	if s.gatewayLocator == nil {
		return ""
	}
	return s.gatewayLocator.PickGateway(dc)
}

// RefreshPrimaryGatewayFallbackAddresses is used to update the list of current
// fallback addresses for locating mesh gateways in the primary datacenter.
func (s *Server) RefreshPrimaryGatewayFallbackAddresses(addrs []string) {
	if s.gatewayLocator != nil {
		s.gatewayLocator.RefreshPrimaryGatewayFallbackAddresses(addrs)
	}
}

// PrimaryGatewayFallbackAddresses returns the current set of discovered
// fallback addresses for the mesh gateways in the primary datacenter.
func (s *Server) PrimaryGatewayFallbackAddresses() []string {
	if s.gatewayLocator == nil {
		return nil
	}
	return s.gatewayLocator.PrimaryGatewayFallbackAddresses()
}

// AgentLocalMember is used to retrieve the LAN member for the local node.
func (s *Server) AgentLocalMember() serf.Member {
	return s.serfLAN.LocalMember()
}

// LANMembersInAgentPartition returns the LAN members for this agent's
// canonical serf pool. For clients this is the only pool that exists. For
// servers it's the pool in the default segment and the default partition.
func (s *Server) LANMembersInAgentPartition() []serf.Member {
	return s.serfLAN.Members()
}

// WANMembers is used to return the members of the WAN cluster
func (s *Server) WANMembers() []serf.Member {
	if s.serfWAN == nil {
		return nil
	}
	return s.serfWAN.Members()
}

// GetPeeringBackend is a test helper.
func (s *Server) GetPeeringBackend() peering.Backend {
	return s.peeringBackend
}

// RemoveFailedNode is used to remove a failed node from the cluster.
func (s *Server) RemoveFailedNode(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	var removeFn func(*serf.Serf, string) error
	if prune {
		removeFn = (*serf.Serf).RemoveFailedNodePrune
	} else {
		removeFn = (*serf.Serf).RemoveFailedNode
	}

	wanNode := node

	// The Serf WAN pool stores members as node.datacenter
	// so the dc is appended if not present
	if !strings.HasSuffix(node, "."+s.config.Datacenter) {
		wanNode = node + "." + s.config.Datacenter
	}

	return s.removeFailedNode(removeFn, node, wanNode, entMeta)
}

// RemoveFailedNodeWAN is used to remove a failed node from the WAN cluster.
func (s *Server) RemoveFailedNodeWAN(wanNode string, prune bool, entMeta *acl.EnterpriseMeta) error {
	var removeFn func(*serf.Serf, string) error
	if prune {
		removeFn = (*serf.Serf).RemoveFailedNodePrune
	} else {
		removeFn = (*serf.Serf).RemoveFailedNode
	}

	return s.removeFailedNode(removeFn, "", wanNode, entMeta)
}

// IsLeader checks if this server is the cluster leader
func (s *Server) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// LeaderLastContact returns the time of last contact by a leader.
// This only makes sense if we are currently a follower.
func (s *Server) LeaderLastContact() time.Time {
	return s.raft.LastContact()
}

// KeyManagerLAN returns the LAN Serf keyring manager
func (s *Server) KeyManagerLAN() *serf.KeyManager {
	// NOTE: The serfLAN keymanager is shared by all partitions.
	return s.serfLAN.KeyManager()
}

// KeyManagerWAN returns the WAN Serf keyring manager
func (s *Server) KeyManagerWAN() *serf.KeyManager {
	return s.serfWAN.KeyManager()
}

func (s *Server) AgentEnterpriseMeta() *acl.EnterpriseMeta {
	return s.config.AgentEnterpriseMeta()
}

// inmemCodec is used to do an RPC call without going over a network
type inmemCodec struct {
	method     string
	args       interface{}
	reply      interface{}
	err        error
	sourceAddr net.Addr
}

func (i *inmemCodec) ReadRequestHeader(req *rpc.Request) error {
	req.ServiceMethod = i.method
	return nil
}

func (i *inmemCodec) ReadRequestBody(args interface{}) error {
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.args)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(args)))
	dst.Set(sourceValue)
	return nil
}

func (i *inmemCodec) WriteResponse(resp *rpc.Response, reply interface{}) error {
	if resp.Error != "" {
		i.err = errors.New(resp.Error)
		return nil
	}
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(reply)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.reply)))
	dst.Set(sourceValue)
	return nil
}

func (i *inmemCodec) SourceAddr() net.Addr {
	return i.sourceAddr
}

func (i *inmemCodec) Close() error {
	return nil
}

// RPC is used to make a local RPC call
func (s *Server) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	remoteAddr, _ := RemoteAddrFromContext(ctx)
	codec := &inmemCodec{
		method:     method,
		args:       args,
		reply:      reply,
		sourceAddr: remoteAddr,
	}

	// Enforce the RPC limit.
	//
	// "client" metric path because the internal client API is calling to the
	// internal server API. It's odd that the same request directed to a server is
	// recorded differently. On the other hand this possibly masks the different
	// between regular client requests that traverse the network and these which
	// don't (unless forwarded). This still seems most reasonable.
	metrics.IncrCounter([]string{"client", "rpc"}, 1)
	if !s.rpcLimiter.Load().(*rate.Limiter).Allow() {
		metrics.IncrCounter([]string{"client", "rpc", "exceeded"}, 1)
		return structs.ErrRPCRateExceeded
	}
	if err := s.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.err
}

// SnapshotRPC dispatches the given snapshot request, reading from the streaming
// input and writing to the streaming output depending on the operation.
func (s *Server) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer,
	replyFn structs.SnapshotReplyFn) error {

	// Enforce the RPC limit.
	//
	// "client" metric path because the internal client API is calling to the
	// internal server API. It's odd that the same request directed to a server is
	// recorded differently. On the other hand this possibly masks the different
	// between regular client requests that traverse the network and these which
	// don't (unless forwarded). This still seems most reasonable.
	metrics.IncrCounter([]string{"client", "rpc"}, 1)
	if !s.rpcLimiter.Load().(*rate.Limiter).Allow() {
		metrics.IncrCounter([]string{"client", "rpc", "exceeded"}, 1)
		return structs.ErrRPCRateExceeded
	}

	// Perform the operation.
	var reply structs.SnapshotResponse
	snap, err := s.dispatchSnapshotRequest(args, in, &reply)
	if err != nil {
		return err
	}
	defer func() {
		if err := snap.Close(); err != nil {
			s.logger.Error("Failed to close snapshot", "error", err)
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

// RegisterEndpoint is used to substitute an endpoint for testing.
func (s *Server) RegisterEndpoint(name string, handler interface{}) error {
	s.logger.Warn("endpoint injected; this should only be used for testing")
	return s.rpcServer.RegisterName(name, handler)
}

func (s *Server) FSM() *fsm.FSM {
	return s.fsm
}

func (s *Server) GetState() *state.Store {
	if s == nil || s.FSM() == nil {
		return nil
	}
	return s.FSM().State()
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (s *Server) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	numKnownDCs := len(s.router.GetDatacenters())
	stats := map[string]map[string]string{
		"consul": {
			"server":            "true",
			"leader":            fmt.Sprintf("%v", s.IsLeader()),
			"leader_addr":       string(s.raft.Leader()),
			"bootstrap":         fmt.Sprintf("%v", s.config.Bootstrap),
			"known_datacenters": toString(uint64(numKnownDCs)),
		},
		"raft":     s.raft.Stats(),
		"serf_lan": s.serfLAN.Stats(),
		"runtime":  runtimeStats(),
	}

	if s.config.ACLsEnabled {
		stats["consul"]["acl"] = "enabled"
	} else {
		stats["consul"]["acl"] = "disabled"
	}

	if s.serfWAN != nil {
		stats["serf_wan"] = s.serfWAN.Stats()
	}

	s.addEnterpriseStats(stats)

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
func (s *Server) GetLANCoordinate() (lib.CoordinateSet, error) {
	lan, err := s.serfLAN.GetCoordinate()
	if err != nil {
		return nil, err
	}

	cs := lib.CoordinateSet{"": lan}
	if err := s.addEnterpriseLANCoordinates(cs); err != nil {
		return nil, err
	}

	return cs, nil
}

func (s *Server) agentSegmentName() string {
	return s.config.Segment
}

// ReloadConfig is used to have the Server do an online reload of
// relevant configuration information
func (s *Server) ReloadConfig(config ReloadableConfig) error {
	// Reload raft config first before updating any other state since it could
	// error if the new config is invalid.
	raftCfg := computeRaftReloadableConfig(config)
	if err := s.raft.ReloadConfig(raftCfg); err != nil {
		return err
	}

	s.updateReportingConfig(config)

	s.rpcLimiter.Store(rate.NewLimiter(config.RPCRateLimit, config.RPCMaxBurst))

	if config.RequestLimits != nil {
		s.incomingRPCLimiter.UpdateConfig(*convertConsulConfigToRateLimitHandlerConfig(*config.RequestLimits, nil))
	}

	s.rpcConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: config.RPCMaxConnsPerClient,
	})
	s.connPool.SetRPCClientTimeout(config.RPCClientTimeout)

	if s.IsLeader() {
		// only bootstrap the config entries if we are the leader
		// this will error if we lose leadership while bootstrapping here.
		return s.bootstrapConfigEntries(config.ConfigEntryBootstrap)
	}

	return nil
}

// computeRaftReloadableConfig works out the correct reloadable config for raft.
// We reload raft even if nothing has changed since it's cheap and simpler than
// trying to work out if it's different from the current raft config. This
// function is separate to make it cheap to table test thoroughly without a full
// raft instance.
func computeRaftReloadableConfig(config ReloadableConfig) raft.ReloadableConfig {
	// We use the raw defaults _not_ the current values so that you can reload
	// back to a zero value having previously started Consul with a custom value
	// for one of these fields.
	defaultConf := DefaultConfig()
	raftCfg := raft.ReloadableConfig{
		TrailingLogs:      defaultConf.RaftConfig.TrailingLogs,
		SnapshotInterval:  defaultConf.RaftConfig.SnapshotInterval,
		SnapshotThreshold: defaultConf.RaftConfig.SnapshotThreshold,
		ElectionTimeout:   defaultConf.RaftConfig.ElectionTimeout,
		HeartbeatTimeout:  defaultConf.RaftConfig.HeartbeatTimeout,
	}
	if config.RaftSnapshotThreshold != 0 {
		raftCfg.SnapshotThreshold = uint64(config.RaftSnapshotThreshold)
	}
	if config.RaftSnapshotInterval != 0 {
		raftCfg.SnapshotInterval = config.RaftSnapshotInterval
	}
	if config.RaftTrailingLogs != 0 {
		raftCfg.TrailingLogs = uint64(config.RaftTrailingLogs)
	}
	if config.HeartbeatTimeout >= 5*time.Millisecond {
		raftCfg.HeartbeatTimeout = config.HeartbeatTimeout
	}
	if config.ElectionTimeout >= 5*time.Millisecond {
		raftCfg.ElectionTimeout = config.ElectionTimeout
	}
	return raftCfg
}

// Atomically sets a readiness state flag when leadership is obtained, to indicate that server is past its barrier write
func (s *Server) setConsistentReadReady() {
	atomic.StoreInt32(&s.readyForConsistentReads, 1)
}

// Atomically reset readiness state flag on leadership revoke
func (s *Server) resetConsistentReadReady() {
	atomic.StoreInt32(&s.readyForConsistentReads, 0)
}

// Returns true if this server is ready to serve consistent reads
func (s *Server) isReadyForConsistentReads() bool {
	return atomic.LoadInt32(&s.readyForConsistentReads) == 1
}

// trackLeaderChanges registers an Observer with raft in order to receive updates
// about leader changes, in order to keep the grpc resolver up to date for leader forwarding.
func (s *Server) trackLeaderChanges() {
	obsCh := make(chan raft.Observation, 16)
	observer := raft.NewObserver(obsCh, false, func(o *raft.Observation) bool {
		_, ok := o.Data.(raft.LeaderObservation)
		return ok
	})
	s.raft.RegisterObserver(observer)

	for {
		select {
		case obs := <-obsCh:
			leaderObs, ok := obs.Data.(raft.LeaderObservation)
			if !ok {
				s.logger.Debug("got unknown observation type from raft", "type", reflect.TypeOf(obs.Data))
				continue
			}

			s.grpcLeaderForwarder.UpdateLeaderAddr(s.config.Datacenter, string(leaderObs.LeaderAddr))
			s.peeringBackend.SetLeaderAddress(string(leaderObs.LeaderAddr))
			s.raftStorageBackend.LeaderChanged()
			s.controllerManager.SetRaftLeader(s.IsLeader())

			// Trigger sending an update to HCP status
			s.hcpManager.SendUpdate()
		case <-s.shutdownCh:
			s.raft.DeregisterObserver(observer)
			return
		}
	}
}

// hcpServerStatus is the callback used by the HCP manager to emit status updates to the HashiCorp Cloud Platform when
// enabled.
func (s *Server) hcpServerStatus(deps Deps) hcp.StatusCallback {
	return func(ctx context.Context) (status hcp.ServerStatus, err error) {
		status.Name = s.config.NodeName
		status.ID = string(s.config.NodeID)
		status.Version = cslversion.GetHumanVersion()
		status.LanAddress = s.config.RPCAdvertise.IP.String()
		status.GossipPort = s.config.SerfLANConfig.MemberlistConfig.AdvertisePort
		status.RPCPort = s.config.RPCAddr.Port
		status.Datacenter = s.config.Datacenter

		tlsCert := s.tlsConfigurator.Cert()
		if tlsCert != nil {
			status.TLS.Enabled = true
			leaf := tlsCert.Leaf
			if leaf == nil {
				// Parse the leaf cert
				leaf, err = x509.ParseCertificate(tlsCert.Certificate[0])
				if err != nil {
					// Shouldn't be possible
					return
				}
			}
			status.TLS.CertName = leaf.Subject.CommonName
			status.TLS.CertSerial = leaf.SerialNumber.String()
			status.TLS.CertExpiry = leaf.NotAfter
			status.TLS.VerifyIncoming = s.tlsConfigurator.VerifyIncomingRPC()
			status.TLS.VerifyOutgoing = s.tlsConfigurator.Base().InternalRPC.VerifyOutgoing
			status.TLS.VerifyServerHostname = s.tlsConfigurator.VerifyServerHostname()
		}

		status.Raft.IsLeader = s.raft.State() == raft.Leader
		_, leaderID := s.raft.LeaderWithID()
		status.Raft.KnownLeader = leaderID != ""
		status.Raft.AppliedIndex = s.raft.AppliedIndex()
		if !status.Raft.IsLeader {
			status.Raft.TimeSinceLastContact = time.Since(s.raft.LastContact())
		}

		apState := s.autopilot.GetState()
		status.Autopilot.Healthy = apState.Healthy
		status.Autopilot.FailureTolerance = apState.FailureTolerance
		status.Autopilot.NumServers = len(apState.Servers)
		status.Autopilot.NumVoters = len(apState.Voters)
		status.Autopilot.MinQuorum = int(s.getAutopilotConfigOrDefault().MinQuorum)

		status.ScadaStatus = "unknown"
		if deps.HCP.Provider != nil {
			status.ScadaStatus = deps.HCP.Provider.SessionStatus()
		}

		status.ACL.Enabled = s.config.ACLsEnabled

		return status, nil
	}
}

func fileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		// File exists!
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	// We hit some other error trying to stat the file which leaves us in an
	// unknown state so we can't proceed.
	return false, err
}

func ConfiguredIncomingRPCLimiter(ctx context.Context, serverLogger hclog.InterceptLogger, consulCfg *Config) *rpcRate.Handler {
	mlCfg := &multilimiter.Config{ReconcileCheckLimit: 30 * time.Second, ReconcileCheckInterval: time.Second}
	limitsConfig := &RequestLimits{
		Mode:      rpcRate.RequestLimitsModeFromNameWithDefault(consulCfg.RequestLimitsMode),
		ReadRate:  consulCfg.RequestLimitsReadRate,
		WriteRate: consulCfg.RequestLimitsWriteRate,
	}

	sink := logdrop.NewLogDropSink(ctx, 100, serverLogger.Named("rpc-rate-limit"), func(l logdrop.Log) {
		metrics.IncrCounter([]string{"rpc", "rate_limit", "log_dropped"}, 1)
	})
	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{Output: io.Discard})
	logger.RegisterSink(sink)

	rateLimiterConfig := convertConsulConfigToRateLimitHandlerConfig(*limitsConfig, mlCfg)

	return rpcRate.NewHandler(*rateLimiterConfig, logger)
}

func convertConsulConfigToRateLimitHandlerConfig(limitsConfig RequestLimits, multilimiterConfig *multilimiter.Config) *rpcRate.HandlerConfig {
	hc := &rpcRate.HandlerConfig{
		GlobalLimitConfig: rpcRate.GlobalLimitConfig{
			Mode: limitsConfig.Mode,
			ReadWriteConfig: rpcRate.ReadWriteConfig{
				ReadConfig: multilimiter.LimiterConfig{
					Rate:  limitsConfig.ReadRate,
					Burst: int(limitsConfig.ReadRate) * requestLimitsBurstMultiplier,
				},
				WriteConfig: multilimiter.LimiterConfig{
					Rate:  limitsConfig.WriteRate,
					Burst: int(limitsConfig.WriteRate) * requestLimitsBurstMultiplier,
				},
			},
		},
	}
	if multilimiterConfig != nil {
		hc.Config = *multilimiterConfig
	}

	return hc
}

// peersInfoContent is used to help operators understand what happened to the
// peers.json file. This is written to a file called peers.info in the same
// location.
const peersInfoContent = `
As of Consul 0.7.0, the peers.json file is only used for recovery
after an outage. The format of this file depends on what the server has
configured for its Raft protocol version. Please see the agent configuration
page at https://www.consul.io/docs/agent/config/cli-flags#_raft_protocol for more
details about this parameter.

For Raft protocol version 2 and earlier, this should be formatted as a JSON
array containing the address and port of each Consul server in the cluster, like
this:

[
  "10.1.0.1:8300",
  "10.1.0.2:8300",
  "10.1.0.3:8300"
]

For Raft protocol version 3 and later, this should be formatted as a JSON
array containing the node ID, address:port, and suffrage information of each
Consul server in the cluster, like this:

[
  {
    "id": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "address": "10.1.0.1:8300",
    "non_voter": false
  },
  {
    "id": "8b6dda82-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.2:8300",
    "non_voter": false
  },
  {
    "id": "97e17742-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.3:8300",
    "non_voter": false
  }
]

The "id" field is the node ID of the server. This can be found in the logs when
the server starts up, or in the "node-id" file inside the server's data
directory.

The "address" field is the address and port of the server.

The "non_voter" field controls whether the server is a non-voter, which is used
in some advanced Autopilot configurations, please see
https://www.consul.io/docs/guides/autopilot.html for more information. If
"non_voter" is omitted it will default to false, which is typical for most
clusters.

Under normal operation, the peers.json file will not be present.

When Consul starts for the first time, it will create this peers.info file and
delete any existing peers.json file so that recovery doesn't occur on the first
startup.

Once this peers.info file is present, any peers.json file will be ingested at
startup, and will set the Raft peer configuration manually to recover from an
outage. It's crucial that all servers in the cluster are shut down before
creating the peers.json file, and that all servers receive the same
configuration. Once the peers.json file is successfully ingested and applied, it
will be deleted.

Please see https://www.consul.io/docs/guides/outage.html for more information.
`
