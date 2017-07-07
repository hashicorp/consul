package consul

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/agent"
	"github.com/hashicorp/consul/agent/consul/servers"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
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
)

// Server is Consul server which manages the service discovery,
// health checking, DC forwarding, Raft, and multiple Serf pools.
type Server struct {
	// aclAuthCache is the authoritative ACL cache.
	aclAuthCache *acl.Cache

	// aclCache is the non-authoritative ACL cache.
	aclCache *aclCache

	// autopilotPolicy controls the behavior of Autopilot for certain tasks.
	autopilotPolicy AutopilotPolicy

	// autopilotRemoveDeadCh is used to trigger a check for dead server removals.
	autopilotRemoveDeadCh chan struct{}

	// autopilotShutdownCh is used to stop the Autopilot loop.
	autopilotShutdownCh chan struct{}

	// autopilotWaitGroup is used to block until Autopilot shuts down.
	autopilotWaitGroup sync.WaitGroup

	// clusterHealth stores the current view of the cluster's health.
	clusterHealth     structs.OperatorHealthReply
	clusterHealthLock sync.RWMutex

	// Consul configuration
	config *Config

	// Connection pool to other consul servers
	connPool *pool.ConnPool

	// Endpoints holds our RPC endpoints
	endpoints endpoints

	// eventChLAN is used to receive events from the
	// serf cluster in the datacenter
	eventChLAN chan serf.Event

	// eventChWAN is used to receive events from the
	// serf cluster that spans datacenters
	eventChWAN chan serf.Event

	// fsm is the state machine used with Raft to provide
	// strong consistency.
	fsm *consulFSM

	// localConsuls is used to track the known consuls
	// in the local datacenter. Used to do leader forwarding.
	localConsuls map[raft.ServerAddress]*agent.Server
	localLock    sync.RWMutex

	// Logger uses the provided LogOutput
	logger *log.Logger

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

	// used to track when the server is ready to serve consistent reads, updated atomically
	readyForConsistentReads int32

	// router is used to map out Consul servers in the WAN and in Consul
	// Enterprise user-defined areas.
	router *servers.Router

	// Listener is used to listen for incoming connections
	Listener  net.Listener
	rpcServer *rpc.Server

	// rpcTLS is the TLS config for incoming TLS requests
	rpcTLS *tls.Config

	// serfLAN is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serfLAN *serf.Serf

	// serfWAN is the Serf cluster maintained between DC's
	// which SHOULD only consist of Consul servers
	serfWAN *serf.Serf

	// floodLock controls access to floodCh.
	floodLock sync.RWMutex
	floodCh   []chan struct{}

	// sessionTimers track the expiration time of each Session that has
	// a TTL. On expiration, a SessionDestroy event will occur, and
	// destroy the session via standard session destroy processing
	sessionTimers *SessionTimers

	// statsFetcher is used by autopilot to check the status of the other
	// Consul servers.
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
}

// Holds the RPC endpoints
type endpoints struct {
	ACL           *ACL
	Catalog       *Catalog
	Coordinate    *Coordinate
	Health        *Health
	Internal      *Internal
	KVS           *KVS
	Operator      *Operator
	PreparedQuery *PreparedQuery
	Session       *Session
	Status        *Status
	Txn           *Txn
}

func NewServer(config *Config) (*Server, error) {
	return NewServerLogger(config, nil)
}

// NewServer is used to construct a new Consul server from the
// configuration, potentially returning an error
func NewServerLogger(config *Config, logger *log.Logger) (*Server, error) {
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
		logger = log.New(config.LogOutput, "", log.LstdFlags)
	}

	// Check if TLS is enabled
	if config.CAFile != "" || config.CAPath != "" {
		config.UseTLS = true
	}

	// Create the TLS wrapper for outgoing connections.
	tlsConf := config.tlsConfig()
	tlsWrap, err := tlsConf.OutgoingTLSWrapper()
	if err != nil {
		return nil, err
	}

	// Get the incoming TLS config.
	incomingTLS, err := tlsConf.IncomingTLSConfig()
	if err != nil {
		return nil, err
	}

	// Create the tombstone GC.
	gc, err := state.NewTombstoneGC(config.TombstoneTTL, config.TombstoneTTLGranularity)
	if err != nil {
		return nil, err
	}

	// Create the shutdown channel - this is closed but never written to.
	shutdownCh := make(chan struct{})

	connPool := &pool.ConnPool{
		SrcAddr:    config.RPCSrcAddr,
		LogOutput:  config.LogOutput,
		MaxTime:    serverRPCCache,
		MaxStreams: serverMaxStreams,
		TLSWrapper: tlsWrap,
		ForceTLS:   config.VerifyOutgoing,
	}

	// Create server.
	s := &Server{
		autopilotRemoveDeadCh: make(chan struct{}),
		autopilotShutdownCh:   make(chan struct{}),
		config:                config,
		connPool:              connPool,
		eventChLAN:            make(chan serf.Event, 256),
		eventChWAN:            make(chan serf.Event, 256),
		localConsuls:          make(map[raft.ServerAddress]*agent.Server),
		logger:                logger,
		reconcileCh:           make(chan serf.Member, 32),
		router:                servers.NewRouter(logger, config.Datacenter),
		rpcServer:             rpc.NewServer(),
		rpcTLS:                incomingTLS,
		reassertLeaderCh:      make(chan chan error),
		sessionTimers:         NewSessionTimers(),
		tombstoneGC:           gc,
		shutdownCh:            shutdownCh,
	}

	// Set up the autopilot policy
	s.autopilotPolicy = &BasicAutopilot{server: s}

	// Initialize the stats fetcher that autopilot will use.
	s.statsFetcher = NewStatsFetcher(logger, s.connPool, s.config.Datacenter)

	// Initialize the authoritative ACL cache.
	s.aclAuthCache, err = acl.NewCache(aclCacheSize, s.aclLocalFault)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create authoritative ACL cache: %v", err)
	}

	// Set up the non-authoritative ACL cache. A nil local function is given
	// if ACL replication isn't enabled.
	var local acl.FaultFunc
	if s.IsACLReplicationEnabled() {
		local = s.aclLocalFault
	}
	if s.aclCache, err = newACLCache(config, logger, s.RPC, local); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create non-authoritative ACL cache: %v", err)
	}

	// Initialize the RPC layer.
	if err := s.setupRPC(tlsWrap); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server.
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Serf and dynamic bind ports
	//
	// The LAN serf cluster announces the port of the WAN serf cluster
	// which creates a race when the WAN cluster is supposed to bind to
	// a dynamic port (port 0). The current memberlist implementation will
	// update the bind port in the configuration after the memberlist is
	// created, so we can pull it out from there reliably, even though it's
	// a little gross to be reading the updated config.

	// Initialize the WAN Serf.
	serfBindPortWAN := config.SerfWANConfig.MemberlistConfig.BindPort
	s.serfWAN, err = s.setupSerf(config.SerfWANConfig, s.eventChWAN, serfWANSnapshot, true, serfBindPortWAN)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start WAN Serf: %v", err)
	}

	// See big comment above why we are doing this.
	if serfBindPortWAN == 0 {
		serfBindPortWAN = config.SerfWANConfig.MemberlistConfig.BindPort
		if serfBindPortWAN == 0 {
			return nil, fmt.Errorf("Failed to get dynamic bind port for WAN Serf")
		}
		s.logger.Printf("[INFO] agent: Serf WAN TCP bound to port %d", serfBindPortWAN)
	}

	// Initialize the LAN Serf.
	s.serfLAN, err = s.setupSerf(config.SerfLANConfig, s.eventChLAN, serfLANSnapshot, false, serfBindPortWAN)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start LAN Serf: %v", err)
	}
	go s.lanEventHandler()

	// Add a "static route" to the WAN Serf and hook it up to Serf events.
	if err := s.router.AddArea(types.AreaWAN, s.serfWAN, s.connPool); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to add WAN serf route: %v", err)
	}
	go servers.HandleSerfEvents(s.logger, s.router, types.AreaWAN, s.serfWAN.ShutdownCh(), s.eventChWAN)

	// Fire up the LAN <-> WAN join flooder.
	portFn := func(s *agent.Server) (int, bool) {
		if s.WanJoinPort > 0 {
			return s.WanJoinPort, true
		}
		return 0, false
	}
	go s.Flood(portFn, s.serfWAN)

	// Start monitoring leadership. This must happen after Serf is set up
	// since it can fire events when leadership is obtained.
	go s.monitorLeadership()

	// Start ACL replication.
	if s.IsACLReplicationEnabled() {
		go s.runACLReplication()
	}

	// Start listening for RPC requests.
	go s.listen()

	// Start the metrics handlers.
	go s.sessionStats()

	// Start the server health checking.
	go s.serverHealthLoop()

	return s, nil
}

// setupSerf is used to setup and initialize a Serf
func (s *Server) setupSerf(conf *serf.Config, ch chan serf.Event, path string, wan bool, wanPort int) (*serf.Serf, error) {
	addr := s.Listener.Addr().(*net.TCPAddr)
	conf.Init()
	if wan {
		conf.NodeName = fmt.Sprintf("%s.%s", s.config.NodeName, s.config.Datacenter)
	} else {
		conf.NodeName = s.config.NodeName
		conf.Tags["wan_join_port"] = fmt.Sprintf("%d", wanPort)
	}
	conf.Tags["role"] = "consul"
	conf.Tags["dc"] = s.config.Datacenter
	conf.Tags["id"] = string(s.config.NodeID)
	conf.Tags["vsn"] = fmt.Sprintf("%d", s.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["raft_vsn"] = fmt.Sprintf("%d", s.config.RaftConfig.ProtocolVersion)
	conf.Tags["build"] = s.config.Build
	conf.Tags["port"] = fmt.Sprintf("%d", addr.Port)
	if s.config.Bootstrap {
		conf.Tags["bootstrap"] = "1"
	}
	if s.config.BootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", s.config.BootstrapExpect)
	}
	if s.config.NonVoter {
		conf.Tags["nonvoter"] = "1"
	}
	if s.config.UseTLS {
		conf.Tags["use_tls"] = "1"
	}
	conf.MemberlistConfig.LogOutput = s.config.LogOutput
	conf.LogOutput = s.config.LogOutput
	conf.Logger = s.logger
	conf.EventCh = ch
	if !s.config.DevMode {
		conf.SnapshotPath = filepath.Join(s.config.DataDir, path)
	}
	conf.ProtocolVersion = protocolVersionMap[s.config.ProtocolVersion]
	conf.RejoinAfterLeave = s.config.RejoinAfterLeave
	if wan {
		conf.Merge = &wanMergeDelegate{}
	} else {
		conf.Merge = &lanMergeDelegate{
			dc:       s.config.Datacenter,
			nodeID:   s.config.NodeID,
			nodeName: s.config.NodeName,
		}
	}

	// Until Consul supports this fully, we disable automatic resolution.
	// When enabled, the Serf gossip may just turn off if we are the minority
	// node which is rather unexpected.
	conf.EnableNameConflictResolution = false
	if err := lib.EnsurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}

	return serf.Create(conf)
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we have an unclean exit then attempt to close the Raft store.
	defer func() {
		if s.raft == nil && s.raftStore != nil {
			if err := s.raftStore.Close(); err != nil {
				s.logger.Printf("[ERR] consul: failed to close Raft store: %v", err)
			}
		}
	}()

	// Create the FSM.
	var err error
	s.fsm, err = NewFSM(s.tombstoneGC, s.config.LogOutput)
	if err != nil {
		return err
	}

	// Create a transport layer.
	trans := raft.NewNetworkTransport(s.raftLayer, 3, 10*time.Second, s.config.LogOutput)
	s.raftTransport = trans

	// Make sure we set the LogOutput.
	s.config.RaftConfig.LogOutput = s.config.LogOutput
	s.config.RaftConfig.Logger = s.logger

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
				s.logger.Printf("[INFO] consul: deleted peers.json file (see peers.info for details)")
			}
		} else if _, err := os.Stat(peersFile); err == nil {
			s.logger.Printf("[INFO] consul: found peers.json file, recovering Raft configuration...")

			var configuration raft.Configuration
			if s.config.RaftConfig.ProtocolVersion < 3 {
				configuration, err = raft.ReadPeersJSON(peersFile)
			} else {
				configuration, err = raft.ReadConfigJSON(peersFile)
			}
			if err != nil {
				return fmt.Errorf("recovery failed to parse peers.json: %v", err)
			}

			tmpFsm, err := NewFSM(s.tombstoneGC, s.config.LogOutput)
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
			s.logger.Printf("[INFO] consul: deleted peers.json file after successful recovery")
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
			// TODO (slackpad) - This will need to be updated when
			// we add support for node IDs.
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
	raftNotifyCh := make(chan bool, 1)
	s.config.RaftConfig.NotifyCh = raftNotifyCh
	s.raftNotifyCh = raftNotifyCh

	// Setup the Raft store.
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, log, stable, snap, trans)
	if err != nil {
		return err
	}
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC(tlsWrap tlsutil.DCWrapper) error {
	// Create endpoints
	s.endpoints.ACL = &ACL{s}
	s.endpoints.Catalog = &Catalog{s}
	s.endpoints.Coordinate = NewCoordinate(s)
	s.endpoints.Health = &Health{s}
	s.endpoints.Internal = &Internal{s}
	s.endpoints.KVS = &KVS{s}
	s.endpoints.Operator = &Operator{s}
	s.endpoints.PreparedQuery = &PreparedQuery{s}
	s.endpoints.Session = &Session{s}
	s.endpoints.Status = &Status{s}
	s.endpoints.Txn = &Txn{s}

	// Register the handlers
	s.rpcServer.Register(s.endpoints.ACL)
	s.rpcServer.Register(s.endpoints.Catalog)
	s.rpcServer.Register(s.endpoints.Coordinate)
	s.rpcServer.Register(s.endpoints.Health)
	s.rpcServer.Register(s.endpoints.Internal)
	s.rpcServer.Register(s.endpoints.KVS)
	s.rpcServer.Register(s.endpoints.Operator)
	s.rpcServer.Register(s.endpoints.PreparedQuery)
	s.rpcServer.Register(s.endpoints.Session)
	s.rpcServer.Register(s.endpoints.Status)
	s.rpcServer.Register(s.endpoints.Txn)

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
	wrapper := tlsutil.SpecificDC(s.config.Datacenter, tlsWrap)

	// Define a callback for determining whether to wrap a connection with TLS
	tlsFunc := func(address raft.ServerAddress) bool {
		if s.config.VerifyOutgoing {
			return true
		}

		s.localLock.RLock()
		server, ok := s.localConsuls[address]
		s.localLock.RUnlock()

		if !ok {
			return false
		}

		return server.UseTLS
	}
	s.raftLayer = NewRaftLayer(s.config.RPCSrcAddr, s.config.RPCAdvertise, wrapper, tlsFunc)
	return nil
}

// Shutdown is used to shutdown the server
func (s *Server) Shutdown() error {
	s.logger.Printf("[INFO] consul: shutting down server")
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()

	if s.shutdown {
		return nil
	}

	s.shutdown = true
	close(s.shutdownCh)

	if s.serfLAN != nil {
		s.serfLAN.Shutdown()
	}

	if s.serfWAN != nil {
		s.serfWAN.Shutdown()
		if err := s.router.RemoveArea(types.AreaWAN); err != nil {
			s.logger.Printf("[WARN] consul: error removing WAN area: %v", err)
		}
	}
	s.router.Shutdown()

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Printf("[WARN] consul: error shutting down raft: %s", err)
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

	return nil
}

// Leave is used to prepare for a graceful shutdown of the server
func (s *Server) Leave() error {
	s.logger.Printf("[INFO] consul: server starting leave")

	// Check the number of known peers
	numPeers, err := s.numPeers()
	if err != nil {
		s.logger.Printf("[ERR] consul: failed to check raft peers: %v", err)
		return err
	}

	addr := s.raftTransport.LocalAddr()

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemoveServer/RemovePeer to safely reduce the quorum size.
	// If we are not the leader, then we should issue our leave intention and wait to be
	// removed for some sane period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 1 {
		minRaftProtocol, err := ServerMinRaftProtocol(s.serfLAN.Members())
		if err != nil {
			return err
		}

		if minRaftProtocol >= 2 && s.config.RaftConfig.ProtocolVersion >= 3 {
			future := s.raft.RemoveServer(raft.ServerID(s.config.NodeID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Printf("[ERR] consul: failed to remove ourself as raft peer: %v", err)
			}
		} else {
			future := s.raft.RemovePeer(addr)
			if err := future.Error(); err != nil {
				s.logger.Printf("[ERR] consul: failed to remove ourself as raft peer: %v", err)
			}
		}
	}

	// Leave the WAN pool
	if s.serfWAN != nil {
		if err := s.serfWAN.Leave(); err != nil {
			s.logger.Printf("[ERR] consul: failed to leave WAN Serf cluster: %v", err)
		}
	}

	// Leave the LAN pool
	if s.serfLAN != nil {
		if err := s.serfLAN.Leave(); err != nil {
			s.logger.Printf("[ERR] consul: failed to leave LAN Serf cluster: %v", err)
		}
	}

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
				s.logger.Printf("[ERR] consul: failed to get raft configuration: %v", err)
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
			s.logger.Printf("[WARN] consul: failed to leave raft configuration gracefully, timeout")
		}
	}

	return nil
}

// numPeers is used to check on the number of known peers, including the local
// node.
func (s *Server) numPeers() (int, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return 0, err
	}
	configuration := future.Configuration()
	return len(configuration.Servers), nil
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
	return s.serfWAN.Join(addrs, true)
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
	return s.serfWAN.Members()
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (s *Server) RemoveFailedNode(node string) error {
	if err := s.serfLAN.RemoveFailedNode(node); err != nil {
		return err
	}
	if err := s.serfWAN.RemoveFailedNode(node); err != nil {
		return err
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
	return s.serfLAN.EncryptionEnabled() && s.serfWAN.EncryptionEnabled()
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
	if err := s.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.err
}

// SnapshotRPC dispatches the given snapshot request, reading from the streaming
// input and writing to the streaming output depending on the operation.
func (s *Server) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer,
	replyFn structs.SnapshotReplyFn) error {

	// Perform the operation.
	var reply structs.SnapshotResponse
	snap, err := s.dispatchSnapshotRequest(args, in, &reply)
	if err != nil {
		return err
	}
	defer func() {
		if err := snap.Close(); err != nil {
			s.logger.Printf("[ERR] consul: Failed to close snapshot: %v", err)
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
	s.logger.Printf("[WARN] consul: endpoint injected; this should only be used for testing")
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
		"serf_wan": s.serfWAN.Stats(),
		"runtime":  runtimeStats(),
	}
	return stats
}

// GetLANCoordinate returns the coordinate of the server in the LAN gossip pool.
func (s *Server) GetLANCoordinate() (*coordinate.Coordinate, error) {
	return s.serfLAN.GetCoordinate()
}

// GetWANCoordinate returns the coordinate of the server in the WAN gossip pool.
func (s *Server) GetWANCoordinate() (*coordinate.Coordinate, error) {
	return s.serfWAN.GetCoordinate()
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
