package consul

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/golang-lru"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-mdb"
	"github.com/hashicorp/serf/serf"
)

// These are the protocol versions that Consul can _understand_. These are
// Consul-level protocol versions, that are used to configure the Serf
// protocol versions.
const (
	ProtocolVersionMin uint8 = 1
	ProtocolVersionMax       = 2
)

const (
	serfLANSnapshot          = "serf/local.snapshot"
	serfWANSnapshot          = "serf/remote.snapshot"
	raftState                = "raft/"
	tmpStatePath             = "tmp/"
	snapshotsRetained        = 2
	raftDBSize32bit   uint64 = 64 * 1024 * 1024       // Limit Raft log to 64MB
	raftDBSize64bit   uint64 = 8 * 1024 * 1024 * 1024 // Limit Raft log to 8GB

	// serverRPCCache controls how long we keep an idle connection
	// open to a server
	serverRPCCache = 2 * time.Minute

	// serverMaxStreams controsl how many idle streams we keep
	// open to a server
	serverMaxStreams = 64

	// Maximum number of cached ACL entries
	aclCacheSize = 256
)

// Server is Consul server which manages the service discovery,
// health checking, DC forwarding, Raft, and multiple Serf pools.
type Server struct {
	// aclAuthCache is the authoritative ACL cache
	aclAuthCache *acl.Cache

	// aclCache is a non-authoritative ACL cache
	aclCache *lru.Cache

	// aclPolicyCache is a policy cache
	aclPolicyCache *lru.Cache

	// Consul configuration
	config *Config

	// Connection pool to other consul servers
	connPool *ConnPool

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

	// Have we attempted to leave the cluster
	left bool

	// localConsuls is used to track the known consuls
	// in the local data center. Used to do leader forwarding.
	localConsuls map[string]*serverParts
	localLock    sync.RWMutex

	// Logger uses the provided LogOutput
	logger *log.Logger

	// The raft instance is used among Consul nodes within the
	// DC to protect operations that require strong consistency
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftPeers     raft.PeerStore
	raftStore     *raftmdb.MDBStore
	raftTransport *raft.NetworkTransport

	// reconcileCh is used to pass events from the serf handler
	// into the leader manager, so that the strong state can be
	// updated
	reconcileCh chan serf.Member

	// remoteConsuls is used to track the known consuls in
	// remote data centers. Used to do DC forwarding.
	remoteConsuls map[string][]*serverParts
	remoteLock    sync.RWMutex

	// rpcListener is used to listen for incoming connections
	rpcListener net.Listener
	rpcServer   *rpc.Server

	// rpcTLS is the TLS config for incoming TLS requests
	rpcTLS *tls.Config

	// serfLAN is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serfLAN *serf.Serf

	// serfWAN is the Serf cluster maintained between DC's
	// which SHOULD only consist of Consul servers
	serfWAN *serf.Serf

	// sessionTimers track the expiration time of each Session that has
	// a TTL. On expiration, a SessionDestroy event will occur, and
	// destroy the session via standard session destory processing
	sessionTimers     map[string]*time.Timer
	sessionTimersLock sync.RWMutex

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// Holds the RPC endpoints
type endpoints struct {
	Catalog  *Catalog
	Health   *Health
	Status   *Status
	KVS      *KVS
	Session  *Session
	Internal *Internal
	ACL      *ACL
}

// NewServer is used to construct a new Consul server from the
// configuration, potentially returning an error
func NewServer(config *Config) (*Server, error) {
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

	// Create the tlsConfig for outgoing connections
	tlsConf := config.tlsConfig()
	tlsConfig, err := tlsConf.OutgoingTLSConfig()
	if err != nil {
		return nil, err
	}

	// Get the incoming tls config
	incomingTLS, err := tlsConf.IncomingTLSConfig()
	if err != nil {
		return nil, err
	}

	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create server
	s := &Server{
		config:        config,
		connPool:      NewPool(config.LogOutput, serverRPCCache, serverMaxStreams, tlsConfig),
		eventChLAN:    make(chan serf.Event, 256),
		eventChWAN:    make(chan serf.Event, 256),
		localConsuls:  make(map[string]*serverParts),
		logger:        logger,
		reconcileCh:   make(chan serf.Member, 32),
		remoteConsuls: make(map[string][]*serverParts),
		rpcServer:     rpc.NewServer(),
		rpcTLS:        incomingTLS,
		shutdownCh:    make(chan struct{}),
	}

	// Initialize the authoritative ACL cache
	s.aclAuthCache, err = acl.NewCache(aclCacheSize, s.aclFault)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL cache: %v", err)
	}

	// Initialize the non-authoritative ACL cache
	s.aclCache, err = lru.New(aclCacheSize)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL cache: %v", err)
	}

	// Initialize the ACL policy cache
	s.aclPolicyCache, err = lru.New(aclCacheSize)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to create ACL policy cache: %v", err)
	}

	// Initialize the RPC layer
	if err := s.setupRPC(tlsConfig); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Initialize the lan Serf
	s.serfLAN, err = s.setupSerf(config.SerfLANConfig,
		s.eventChLAN, serfLANSnapshot, false)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}
	go s.lanEventHandler()

	// Initialize the wan Serf
	s.serfWAN, err = s.setupSerf(config.SerfWANConfig,
		s.eventChWAN, serfWANSnapshot, true)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start wan serf: %v", err)
	}
	go s.wanEventHandler()

	// Start listening for RPC requests
	go s.listen()
	return s, nil
}

func (s *Server) getRemoteConsuls() map[string][]*serverParts {
	s.remoteLock.RLock()
	defer s.remoteLock.RUnlock()
	return s.remoteConsuls
}

func (s *Server) setRemoteConsuls(existing []*serverParts, datacenters string) {
	s.remoteLock.Lock()
	defer s.remoteLock.Unlock()
	s.remoteConsuls[datacenters] = existing
}

func (s *Server) addRemoteConsuls(existing []*serverParts, parts *serverParts) {
	s.remoteLock.Lock()
	defer s.remoteLock.Unlock()
	s.remoteConsuls[parts.Datacenter] = append(existing, parts)
}

func (s *Server) delRemoteConsuls(datacenters string) {
	s.remoteLock.Lock()
	defer s.remoteLock.Unlock()
	delete(s.remoteConsuls, datacenters)
}

// setupSerf is used to setup and initialize a Serf
func (s *Server) setupSerf(conf *serf.Config, ch chan serf.Event, path string, wan bool) (*serf.Serf, error) {
	addr := s.rpcListener.Addr().(*net.TCPAddr)
	conf.Init()
	if wan {
		conf.NodeName = fmt.Sprintf("%s.%s", s.config.NodeName, s.config.Datacenter)
	} else {
		conf.NodeName = s.config.NodeName
	}
	conf.Tags["role"] = "consul"
	conf.Tags["dc"] = s.config.Datacenter
	conf.Tags["vsn"] = fmt.Sprintf("%d", s.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["build"] = s.config.Build
	conf.Tags["port"] = fmt.Sprintf("%d", addr.Port)
	if s.config.Bootstrap {
		conf.Tags["bootstrap"] = "1"
	}
	if s.config.BootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", s.config.BootstrapExpect)
	}
	conf.MemberlistConfig.LogOutput = s.config.LogOutput
	conf.LogOutput = s.config.LogOutput
	conf.EventCh = ch
	conf.SnapshotPath = filepath.Join(s.config.DataDir, path)
	conf.ProtocolVersion = protocolVersionMap[s.config.ProtocolVersion]
	conf.RejoinAfterLeave = s.config.RejoinAfterLeave

	// Until Consul supports this fully, we disable automatic resolution.
	// When enabled, the Serf gossip may just turn off if we are the minority
	// node which is rather unexpected.
	conf.EnableNameConflictResolution = false
	if err := ensurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}
	return serf.Create(conf)
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we are in bootstrap mode, enable a single node cluster
	if s.config.Bootstrap {
		s.config.RaftConfig.EnableSingleNode = true
	}

	// Create the base state path
	statePath := filepath.Join(s.config.DataDir, tmpStatePath)
	if err := os.RemoveAll(statePath); err != nil {
		return err
	}
	if err := ensurePath(statePath, true); err != nil {
		return err
	}

	// Create the FSM
	var err error
	s.fsm, err = NewFSM(statePath, s.config.LogOutput)
	if err != nil {
		return err
	}

	// Set the maximum raft size based on 32/64bit. Since we are
	// doing an mmap underneath, we need to limit our use of virtual
	// address space on 32bit, but don't have to care on 64bit.
	dbSize := raftDBSize32bit
	if runtime.GOARCH == "amd64" {
		dbSize = raftDBSize64bit
	}

	// Create the base raft path
	path := filepath.Join(s.config.DataDir, raftState)
	if err := ensurePath(path, true); err != nil {
		return err
	}

	// Create the MDB store for logs and stable storage
	store, err := raftmdb.NewMDBStoreWithSize(path, dbSize)
	if err != nil {
		return err
	}
	s.raftStore = store

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(path, snapshotsRetained, s.config.LogOutput)
	if err != nil {
		store.Close()
		return err
	}

	// Create a transport layer
	trans := raft.NewNetworkTransport(s.raftLayer, 3, 10*time.Second, s.config.LogOutput)
	s.raftTransport = trans

	// Setup the peer store
	s.raftPeers = raft.NewJSONPeers(path, trans)

	// Ensure local host is always included if we are in bootstrap mode
	if s.config.Bootstrap {
		peers, err := s.raftPeers.Peers()
		if err != nil {
			store.Close()
			return err
		}
		if !raft.PeerContained(peers, trans.LocalAddr()) {
			s.raftPeers.SetPeers(raft.AddUniquePeer(peers, trans.LocalAddr()))
		}
	}

	// Make sure we set the LogOutput
	s.config.RaftConfig.LogOutput = s.config.LogOutput

	// Setup the Raft store
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, store, store,
		snapshots, s.raftPeers, trans)
	if err != nil {
		store.Close()
		trans.Close()
		return err
	}

	// Start monitoring leadership
	go s.monitorLeadership()
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC(tlsConfig *tls.Config) error {
	// Create endpoints
	s.endpoints.Status = &Status{s}
	s.endpoints.Catalog = &Catalog{s}
	s.endpoints.Health = &Health{s}
	s.endpoints.KVS = &KVS{s}
	s.endpoints.Session = &Session{s}
	s.endpoints.Internal = &Internal{s}
	s.endpoints.ACL = &ACL{s}

	// Register the handlers
	s.rpcServer.Register(s.endpoints.Status)
	s.rpcServer.Register(s.endpoints.Catalog)
	s.rpcServer.Register(s.endpoints.Health)
	s.rpcServer.Register(s.endpoints.KVS)
	s.rpcServer.Register(s.endpoints.Session)
	s.rpcServer.Register(s.endpoints.Internal)
	s.rpcServer.Register(s.endpoints.ACL)

	list, err := net.ListenTCP("tcp", s.config.RPCAddr)
	if err != nil {
		return err
	}
	s.rpcListener = list

	var advertise net.Addr
	if s.config.RPCAdvertise != nil {
		advertise = s.config.RPCAdvertise
	} else {
		advertise = s.rpcListener.Addr()
	}

	// Verify that we have a usable advertise address
	addr, ok := advertise.(*net.TCPAddr)
	if !ok {
		list.Close()
		return fmt.Errorf("RPC advertise address is not a TCP Address: %v", addr)
	}
	if addr.IP.IsUnspecified() {
		list.Close()
		return fmt.Errorf("RPC advertise address is not advertisable: %v", addr)
	}

	s.raftLayer = NewRaftLayer(advertise, tlsConfig)
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
	}

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Printf("[WARN] consul: Error shutting down raft: %s", err)
		}
		s.raftStore.Close()

		// Clear the peer set on a graceful leave to avoid
		// triggering elections on a rejoin.
		if s.left {
			s.raftPeers.SetPeers(nil)
		}
	}

	if s.rpcListener != nil {
		s.rpcListener.Close()
	}

	// Close the connection pool
	s.connPool.Shutdown()

	// Close the fsm
	if s.fsm != nil {
		s.fsm.Close()
	}

	return nil
}

// Leave is used to prepare for a graceful shutdown of the server
func (s *Server) Leave() error {
	s.logger.Printf("[INFO] consul: server starting leave")
	s.left = true

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
	return nil
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
func (c *Server) LocalMember() serf.Member {
	return c.serfLAN.LocalMember()
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

// UserEvent is used to fire an event via the Serf layer on the LAN
func (s *Server) UserEvent(name string, payload []byte) error {
	return s.serfLAN.UserEvent(userEventName(name), payload, false)
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

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (s *Server) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}

	remoteConsuls := s.getRemoteConsuls()

	stats := map[string]map[string]string{
		"consul": {
			"server":            "true",
			"leader":            fmt.Sprintf("%v", s.IsLeader()),
			"bootstrap":         fmt.Sprintf("%v", s.config.Bootstrap),
			"known_datacenters": toString(uint64(len(remoteConsuls))),
		},
		"raft":     s.raft.Stats(),
		"serf_lan": s.serfLAN.Stats(),
		"serf_wan": s.serfWAN.Stats(),
		"runtime":  runtimeStats(),
	}
	return stats
}
