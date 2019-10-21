package consul

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	connlimit "github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"
)

// These are the protocol versions that Consul can _understand_. These are
// Consul-level protocol versions, that are used to configure the Serf
// protocol versions.
const (
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

	// serverRPCCache controls how long we keep an idle connection
	// open to a server
	serverRPCCache = 2 * time.Minute

	// serverMaxStreams controls how many idle streams we keep
	// open to a server
	serverMaxStreams = 64

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
)

const (
	legacyACLReplicationRoutineName       = "legacy ACL replication"
	aclPolicyReplicationRoutineName       = "ACL policy replication"
	aclRoleReplicationRoutineName         = "ACL role replication"
	aclTokenReplicationRoutineName        = "ACL token replication"
	aclTokenReapingRoutineName            = "acl token reaping"
	aclUpgradeRoutineName                 = "legacy ACL token upgrade"
	caRootPruningRoutineName              = "CA root pruning"
	configReplicationRoutineName          = "config entry replication"
	federationStateReplicationRoutineName = "federation state replication"
	federationStateAntiEntropyRoutineName = "federation state anti-entropy"
	federationStatePruningRoutineName     = "federation state pruning"
	intentionReplicationRoutineName       = "intention replication"
	secondaryCARootWatchRoutineName       = "secondary CA roots watch"
	secondaryCertRenewWatchRoutineName    = "secondary cert renew watch"
)

var (
	ErrWANFederationDisabled = fmt.Errorf("WAN Federation is disabled")
)

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
	acls *ACLResolver

	aclAuthMethodValidators authmethod.Cache

	// DEPRECATED (ACL-Legacy-Compat) - only needed while we support both
	// useNewACLs is used to determine whether we can use new ACLs or not
	useNewACLs int32

	// autopilot is the Autopilot instance for this server.
	autopilot *autopilot.Autopilot

	// autopilotWaitGroup is used to block until Autopilot shuts down.
	autopilotWaitGroup sync.WaitGroup

	// caProviderReconfigurationLock guards the provider reconfiguration.
	caProviderReconfigurationLock sync.Mutex
	// caProvider is the current CA provider in use for Connect. This is
	// only non-nil when we are the leader.
	caProvider ca.Provider
	// caProviderRoot is the CARoot that was stored along with the ca.Provider
	// active. It's only updated in lock-step with the caProvider. This prevents
	// races between state updates to active roots and the fetch of the provider
	// instance.
	caProviderRoot *structs.CARoot
	caProviderLock sync.RWMutex

	// Consul configuration
	config *Config

	// configReplicator is used to manage the leaders replication routines for
	// centralized config
	configReplicator *Replicator

	// federationStateReplicator is used to manage the leaders replication routines for
	// federation states
	federationStateReplicator *Replicator

	// tokens holds ACL tokens initially from the configuration, but can
	// be updated at runtime, so should always be used instead of going to
	// the configuration directly.
	tokens *token.Store

	// Connection pool to other consul servers
	connPool *pool.ConnPool

	// eventChLAN is used to receive events from the
	// serf cluster in the datacenter
	eventChLAN chan serf.Event

	// eventChWAN is used to receive events from the
	// serf cluster that spans datacenters
	eventChWAN chan serf.Event

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
	raftStore     *raftboltdb.BoltStore
	raftTransport *raft.NetworkTransport
	raftInmem     *raft.InmemStore

	// raftNotifyCh is set up by setupRaft() and ensures that we get reliable leader
	// transition notifications from the Raft layer.
	raftNotifyCh <-chan bool

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

	// router is used to map out Consul servers in the WAN and in Consul
	// Enterprise user-defined areas.
	router *router.Router

	// rpcLimiter is used to rate limit the total number of RPCs initiated
	// from an agent.
	rpcLimiter atomic.Value

	// rpcConnLimiter limits the number of RPC connections from a single source IP
	rpcConnLimiter connlimit.Limiter

	// Listener is used to listen for incoming connections
	Listener  net.Listener
	rpcServer *rpc.Server

	// insecureRPCServer is a RPC server that is configure with
	// IncomingInsecureRPCConfig to allow clients to call AutoEncrypt.Sign
	// to request client certificates. At this point a client doesn't have
	// a client cert and thus cannot present it. This is the only RPC
	// Endpoint that is available at the time of writing.
	insecureRPCServer *rpc.Server

	// tlsConfigurator holds the agent configuration relevant to TLS and
	// configures everything related to it.
	tlsConfigurator *tlsutil.Configurator

	// serfLAN is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serfLAN *serf.Serf

	// segmentLAN maps segment names to their Serf cluster
	segmentLAN map[string]*serf.Serf

	// serfWAN is the Serf cluster maintained between DC's
	// which SHOULD only consist of Consul servers
	serfWAN                *serf.Serf
	memberlistTransportWAN memberlist.IngestionAwareTransport
	gatewayLocator         *GatewayLocator

	// serverLookup tracks server consuls in the local datacenter.
	// Used to do leader forwarding and provide fast lookup by server id and address
	serverLookup *ServerLookup

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

	// State for whether this datacenter is acting as a secondary CA.
	actingSecondaryCA   bool
	actingSecondaryLock sync.RWMutex

	// Manager to handle starting/stopping go routines when establishing/revoking raft leadership
	leaderRoutineManager *LeaderRoutineManager

	// embedded struct to hold all the enterprise specific data
	EnterpriseServer
}

// NewServer is only used to help setting up a server for testing. Normal code
// exercises NewServerLogger.
func NewServer(config *Config) (*Server, error) {
	c, err := tlsutil.NewConfigurator(config.ToTLSUtilConfig(), nil)
	if err != nil {
		return nil, err
	}
	return NewServerLogger(config, nil, new(token.Store), c)
}

// NewServerLogger is used to construct a new Consul server from the
// configuration, potentially returning an error
func NewServerLogger(config *Config, logger hclog.InterceptLogger, tokens *token.Store, tlsConfigurator *tlsutil.Configurator) (*Server, error) {
	// Check the protocol version.
	if err := config.CheckProtocolVersion(); err != nil {
		return nil, err
	}

	// Check for a data directory.
	if config.DataDir == "" && !config.DevMode {
		return nil, fmt.Errorf("Config must provide a DataDir")
	}

	// Sanity check the ACLs.
	if err := config.CheckACL(); err != nil {
		return nil, err
	}

	// Ensure we have a log output and create a logger.
	if config.LogOutput == nil {
		config.LogOutput = os.Stderr
	}

	if logger == nil {
		logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{
			Level:  hclog.Debug,
			Output: config.LogOutput,
		})
	}

	// Check if TLS is enabled
	if config.CAFile != "" || config.CAPath != "" {
		config.UseTLS = true
	}

	// Set the primary DC if it wasn't set.
	if config.PrimaryDatacenter == "" {
		if config.ACLDatacenter != "" {
			config.PrimaryDatacenter = config.ACLDatacenter
		} else {
			config.PrimaryDatacenter = config.Datacenter
		}
	}

	if config.PrimaryDatacenter != "" {
		config.ACLDatacenter = config.PrimaryDatacenter
	}

	// Create the tombstone GC.
	gc, err := state.NewTombstoneGC(config.TombstoneTTL, config.TombstoneTTLGranularity)
	if err != nil {
		return nil, err
	}

	// Create the shutdown channel - this is closed but never written to.
	shutdownCh := make(chan struct{})

	connPool := &pool.ConnPool{
		SrcAddr:         config.RPCSrcAddr,
		LogOutput:       config.LogOutput,
		MaxTime:         serverRPCCache,
		MaxStreams:      serverMaxStreams,
		TLSConfigurator: tlsConfigurator,
		ForceTLS:        config.VerifyOutgoing,
		Datacenter:      config.Datacenter,
	}

	serverLogger := logger.NamedIntercept(logging.ConsulServer)
	loggers := newLoggerStore(serverLogger)
	// Create server.
	s := &Server{
		config:                  config,
		tokens:                  tokens,
		connPool:                connPool,
		eventChLAN:              make(chan serf.Event, serfEventChSize),
		eventChWAN:              make(chan serf.Event, serfEventChSize),
		logger:                  serverLogger,
		loggers:                 loggers,
		leaveCh:                 make(chan struct{}),
		reconcileCh:             make(chan serf.Member, reconcileChSize),
		router:                  router.NewRouter(serverLogger, config.Datacenter),
		rpcServer:               rpc.NewServer(),
		insecureRPCServer:       rpc.NewServer(),
		tlsConfigurator:         tlsConfigurator,
		reassertLeaderCh:        make(chan chan error),
		segmentLAN:              make(map[string]*serf.Serf, len(config.Segments)),
		sessionTimers:           NewSessionTimers(),
		tombstoneGC:             gc,
		serverLookup:            NewServerLookup(),
		shutdownCh:              shutdownCh,
		leaderRoutineManager:    NewLeaderRoutineManager(logger),
		aclAuthMethodValidators: authmethod.NewCache(),
	}

	if s.config.ConnectMeshGatewayWANFederationEnabled {
		s.gatewayLocator = NewGatewayLocator(
			s.logger,
			s,
			s.config.Datacenter,
			s.config.PrimaryDatacenter,
		)
		s.connPool.GatewayResolver = s.gatewayLocator.PickGateway
	}

	// Initialize enterprise specific server functionality
	if err := s.initEnterprise(); err != nil {
		s.Shutdown()
		return nil, err
	}

	s.rpcLimiter.Store(rate.NewLimiter(config.RPCRate, config.RPCMaxBurst))

	configReplicatorConfig := ReplicatorConfig{
		Name:     logging.ConfigEntry,
		Delegate: &FunctionReplicator{ReplicateFn: s.replicateConfig},
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
			Delegate: &FederationStateReplicator{srv: s},
			Logger:   s.logger,
		},
		Rate:   s.config.FederationStateReplicationRate,
		Burst:  s.config.FederationStateReplicationBurst,
		Logger: logger,
	}
	s.federationStateReplicator, err = NewReplicator(&federationStateReplicatorConfig)
	if err != nil {
		s.Shutdown()
		return nil, err
	}

	// Initialize the stats fetcher that autopilot will use.
	s.statsFetcher = NewStatsFetcher(logger, s.connPool, s.config.Datacenter)

	s.aclConfig = newACLConfig(logger)
	s.useNewACLs = 0
	aclConfig := ACLResolverConfig{
		Config:      config,
		Delegate:    s,
		CacheConfig: serverACLCacheConfig,
		AutoDisable: false,
		Logger:      logger,
		ACLConfig:   s.aclConfig,
	}
	// Initialize the ACL resolver.
	if s.acls, err = NewACLResolver(&aclConfig); err != nil {
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

	if s.config.ConnectEnabled && s.config.AutoEncryptAllowTLS {
		go s.trackAutoEncryptCARoots()
	}

	if s.gatewayLocator != nil {
		go s.gatewayLocator.Run(s.shutdownCh)
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
	serfBindPortWAN := -1
	if config.SerfWANConfig != nil {
		serfBindPortWAN = config.SerfWANConfig.MemberlistConfig.BindPort
		s.serfWAN, err = s.setupSerf(config.SerfWANConfig, s.eventChWAN, serfWANSnapshot, true, serfBindPortWAN, "", s.Listener)
		if err != nil {
			s.Shutdown()
			return nil, fmt.Errorf("Failed to start WAN Serf: %v", err)
		}

		// This is always a *memberlist.NetTransport or something which wraps
		// it which satisfies this interface.
		s.memberlistTransportWAN = config.SerfWANConfig.MemberlistConfig.Transport.(memberlist.IngestionAwareTransport)

		// See big comment above why we are doing this.
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
	if err := s.setupSegments(config, serfBindPortWAN, segmentListeners); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to setup network segments: %v", err)
	}

	// Initialize the LAN Serf for the default network segment.
	s.serfLAN, err = s.setupSerf(config.SerfLANConfig, s.eventChLAN, serfLANSnapshot, false, serfBindPortWAN, "", s.Listener)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start LAN Serf: %v", err)
	}
	go s.lanEventHandler()

	// Start the flooders after the LAN event handler is wired up.
	s.floodSegments(config)

	// Add a "static route" to the WAN Serf and hook it up to Serf events.
	if s.serfWAN != nil {
		if err := s.router.AddArea(types.AreaWAN, s.serfWAN, s.connPool, s.config.VerifyOutgoing); err != nil {
			s.Shutdown()
			return nil, fmt.Errorf("Failed to add WAN serf route: %v", err)
		}
		go router.HandleSerfEvents(s.logger, s.router, types.AreaWAN, s.serfWAN.ShutdownCh(), s.eventChWAN)

		// Fire up the LAN <-> WAN join flooder.
		portFn := func(s *metadata.Server) (int, bool) {
			if s.WanJoinPort > 0 {
				return s.WanJoinPort, true
			}
			return 0, false
		}
		go s.Flood(nil, portFn, s.serfWAN)
	}

	// Start enterprise specific functionality
	if err := s.startEnterprise(); err != nil {
		s.Shutdown()
		return nil, err
	}

	// Initialize Autopilot. This must happen before starting leadership monitoring
	// as establishing leadership could attempt to use autopilot and cause a panic.
	s.initAutopilot(config)

	// Start monitoring leadership. This must happen after Serf is set up
	// since it can fire events when leadership is obtained.
	go s.monitorLeadership()

	// Start listening for RPC requests.
	go s.listen(s.Listener)

	// Start listeners for any segments with separate RPC listeners.
	for _, listener := range segmentListeners {
		go s.listen(listener)
	}

	// Start the metrics handlers.
	go s.updateMetrics()

	return s, nil
}

func (s *Server) trackAutoEncryptCARoots() {
	for {
		select {
		case <-s.shutdownCh:
			s.logger.Debug("shutting down trackAutoEncryptCARoots because shutdown")
			return
		default:
		}
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
		if err := s.tlsConfigurator.UpdateAutoEncryptCA(caPems); err != nil {
			s.logger.Error("Failed to update AutoEncrypt CAPems", "error", err)
		}
		ws.Watch(nil)
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

	// Create the FSM.
	var err error
	s.fsm, err = fsm.New(s.tombstoneGC, s.logger)
	if err != nil {
		return err
	}

	var serverAddressProvider raft.ServerAddressProvider = nil
	if s.config.RaftConfig.ProtocolVersion >= 3 { //ServerAddressProvider needs server ids to work correctly, which is only supported in protocol version 3 or higher
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

		// Create the backend raft store for logs and stable storage.
		store, err := raftboltdb.NewBoltStore(filepath.Join(path, "raft.db"))
		if err != nil {
			return err
		}
		s.raftStore = store
		stable = store

		// Wrap the store in a LogCache to improve performance.
		cacheStore, err := raft.NewLogCache(raftLogCacheSize, store)
		if err != nil {
			return err
		}
		log = cacheStore

		// Create the snapshot store.
		snapshots, err := raft.NewFileSnapshotStore(path, snapshotsRetained, s.config.LogOutput)
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
			if err := ioutil.WriteFile(peersInfoFile, []byte(peersInfoContent), 0755); err != nil {
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

			tmpFsm, err := fsm.New(s.tombstoneGC, s.logger)
			if err != nil {
				return fmt.Errorf("recovery failed to make temp FSM: %v", err)
			}
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
	if s.config.Bootstrap || s.config.DevMode {
		hasState, err := raft.HasExistingState(log, stable, snap)
		if err != nil {
			return err
		}
		if !hasState {
			configuration := raft.Configuration{
				Servers: []raft.Server{
					raft.Server{
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
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm.ChunkingFSM(), log, stable, snap, trans)
	if err != nil {
		return err
	}
	return nil
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

	// Provide a DC specific wrapper. Raft replication is only
	// ever done in the same datacenter, so we can provide it as a constant.
	wrapper := tlsutil.SpecificDC(s.config.Datacenter, s.tlsConfigurator.OutgoingRPCWrapper())

	// Define a callback for determining whether to wrap a connection with TLS
	tlsFunc := func(address raft.ServerAddress) bool {
		if s.config.VerifyOutgoing {
			return true
		}

		server := s.serverLookup.Server(address)

		if server == nil {
			return false
		}

		return server.UseTLS
	}
	s.raftLayer = NewRaftLayer(s.config.RPCSrcAddr, s.config.RPCAdvertise, wrapper, tlsFunc)
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

	if s.serfLAN != nil {
		s.serfLAN.Shutdown()
	}

	if s.serfWAN != nil {
		s.serfWAN.Shutdown()
		if err := s.router.RemoveArea(types.AreaWAN); err != nil {
			s.logger.Warn("error removing WAN area", "error", err)
		}
	}
	s.router.Shutdown()

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

	// Close the connection pool
	s.connPool.Shutdown()

	s.acls.Close()

	if s.config.NotifyShutdown != nil {
		s.config.NotifyShutdown()
	}

	return nil
}

// Leave is used to prepare for a graceful shutdown of the server
func (s *Server) Leave() error {
	s.logger.Info("server starting leave")

	// Check the number of known peers
	numPeers, err := s.numPeers()
	if err != nil {
		s.logger.Error("failed to check raft peers", "error", err)
		return err
	}

	addr := s.raftTransport.LocalAddr()

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemoveServer/RemovePeer to safely reduce the quorum size.
	// If we are not the leader, then we should issue our leave intention and wait to be
	// removed for some sane period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 1 {
		minRaftProtocol, err := s.autopilot.MinRaftProtocol()
		if err != nil {
			return err
		}

		if minRaftProtocol >= 2 && s.config.RaftConfig.ProtocolVersion >= 3 {
			future := s.raft.RemoveServer(raft.ServerID(s.config.NodeID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove ourself as raft peer", "error", err)
			}
		} else {
			future := s.raft.RemovePeer(addr)
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

// numPeers is used to check on the number of known peers, including potentially
// the local node. We count only voters, since others can't actually become
// leader, so aren't considered peers.
func (s *Server) numPeers() (int, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return 0, err
	}

	return autopilot.NumPeers(future.Configuration()), nil
}

// JoinLAN is used to have Consul join the inner-DC pool
// The target address should be another node inside the DC
// listening on the Serf LAN address
func (s *Server) JoinLAN(addrs []string) (int, error) {
	return s.serfLAN.Join(addrs, true)
}

// JoinWAN is used to have Consul join the cross-WAN Consul ring
// The target address should be another node listening on the
// Serf WAN address
func (s *Server) JoinWAN(addrs []string) (int, error) {
	if s.serfWAN == nil {
		return 0, ErrWANFederationDisabled
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
func (s *Server) RefreshPrimaryGatewayFallbackAddresses(addrs []string) (int, error) {
	if s.gatewayLocator == nil {
		return 0, nil
	}
	return s.gatewayLocator.RefreshPrimaryGatewayFallbackAddresses(addrs)
}

// PrimaryGatewayFallbackAddresses returns the current set of discovered
// fallback addresses for the mesh gateways in the primary datacenter.
func (s *Server) PrimaryGatewayFallbackAddresses() []string {
	if s.gatewayLocator == nil {
		return nil
	}
	return s.gatewayLocator.PrimaryGatewayFallbackAddresses()
}

// LocalMember is used to return the local node
func (s *Server) LocalMember() serf.Member {
	return s.serfLAN.LocalMember()
}

// LANMembers is used to return the members of the LAN cluster
func (s *Server) LANMembers() []serf.Member {
	return s.serfLAN.Members()
}

// WANMembers is used to return the members of the LAN cluster
func (s *Server) WANMembers() []serf.Member {
	if s.serfWAN == nil {
		return nil
	}
	return s.serfWAN.Members()
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (s *Server) RemoveFailedNode(node string, prune bool) error {
	var removeFn func(*serf.Serf, string) error
	if prune {
		removeFn = (*serf.Serf).RemoveFailedNodePrune
	} else {
		removeFn = (*serf.Serf).RemoveFailedNode
	}

	if err := removeFn(s.serfLAN, node); err != nil {
		return err
	}
	// The Serf WAN pool stores members as node.datacenter
	// so the dc is appended if not present
	if !strings.HasSuffix(node, "."+s.config.Datacenter) {
		node = node + "." + s.config.Datacenter
	}
	if s.serfWAN != nil {
		if err := removeFn(s.serfWAN, node); err != nil {
			return err
		}
	}
	return nil
}

// IsLeader checks if this server is the cluster leader
func (s *Server) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// KeyManagerLAN returns the LAN Serf keyring manager
func (s *Server) KeyManagerLAN() *serf.KeyManager {
	return s.serfLAN.KeyManager()
}

// KeyManagerWAN returns the WAN Serf keyring manager
func (s *Server) KeyManagerWAN() *serf.KeyManager {
	return s.serfWAN.KeyManager()
}

// Encrypted determines if gossip is encrypted
func (s *Server) Encrypted() bool {
	LANEncrypted := s.serfLAN.EncryptionEnabled()
	if s.serfWAN == nil {
		return LANEncrypted
	}
	return LANEncrypted && s.serfWAN.EncryptionEnabled()
}

// LANSegments returns a map of LAN segments by name
func (s *Server) LANSegments() map[string]*serf.Serf {
	segments := make(map[string]*serf.Serf, len(s.segmentLAN)+1)
	segments[""] = s.serfLAN
	for name, segment := range s.segmentLAN {
		segments[name] = segment
	}

	return segments
}

// inmemCodec is used to do an RPC call without going over a network
type inmemCodec struct {
	method string
	args   interface{}
	reply  interface{}
	err    error
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

func (i *inmemCodec) Close() error {
	return nil
}

// RPC is used to make a local RPC call
func (s *Server) RPC(method string, args interface{}, reply interface{}) error {
	codec := &inmemCodec{
		method: method,
		args:   args,
		reply:  reply,
	}

	// Enforce the RPC limit.
	//
	// "client" metric path because the internal client API is calling to the
	// internal server API. It's odd that the same request directed to a server is
	// recorded differently. On the other hand this possibly masks the different
	// between regular client requests that traverse the network and these which
	// don't (unless forwarded). This still seems most sane.
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
	// don't (unless forwarded). This still seems most sane.
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

// RegisterEndpoint is used to substitute an endpoint for testing.
func (s *Server) RegisterEndpoint(name string, handler interface{}) error {
	s.logger.Warn("endpoint injected; this should only be used for testing")
	return s.rpcServer.RegisterName(name, handler)
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (s *Server) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	numKnownDCs := len(s.router.GetDatacenters())
	stats := map[string]map[string]string{
		"consul": map[string]string{
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

	if s.ACLsEnabled() {
		if s.UseLegacyACLs() {
			stats["consul"]["acl"] = "legacy"
		} else {
			stats["consul"]["acl"] = "enabled"
		}
	} else {
		stats["consul"]["acl"] = "disabled"
	}

	if s.serfWAN != nil {
		stats["serf_wan"] = s.serfWAN.Stats()
	}

	for outerKey, outerValue := range s.enterpriseStats() {
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

// GetLANCoordinate returns the coordinate of the server in the LAN gossip pool.
func (s *Server) GetLANCoordinate() (lib.CoordinateSet, error) {
	lan, err := s.serfLAN.GetCoordinate()
	if err != nil {
		return nil, err
	}

	cs := lib.CoordinateSet{"": lan}
	for name, segment := range s.segmentLAN {
		c, err := segment.GetCoordinate()
		if err != nil {
			return nil, err
		}
		cs[name] = c
	}
	return cs, nil
}

// ReloadConfig is used to have the Server do an online reload of
// relevant configuration information
func (s *Server) ReloadConfig(config *Config) error {
	s.rpcLimiter.Store(rate.NewLimiter(config.RPCRate, config.RPCMaxBurst))
	s.rpcConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: config.RPCMaxConnsPerClient,
	})

	if s.IsLeader() {
		// only bootstrap the config entries if we are the leader
		// this will error if we lose leadership while bootstrapping here.
		return s.bootstrapConfigEntries(config.ConfigEntryBootstrap)
	}
	return nil
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

func (s *Server) intentionReplicationEnabled() bool {
	return s.config.ConnectEnabled && s.config.Datacenter != s.config.PrimaryDatacenter
}

// peersInfoContent is used to help operators understand what happened to the
// peers.json file. This is written to a file called peers.info in the same
// location.
const peersInfoContent = `
As of Consul 0.7.0, the peers.json file is only used for recovery
after an outage. The format of this file depends on what the server has
configured for its Raft protocol version. Please see the agent configuration
page at https://www.consul.io/docs/agent/options.html#_raft_protocol for more
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
