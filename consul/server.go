package consul

import (
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	serfLANSnapshot = "serf/local.snapshot"
	serfWANSnapshot = "serf/remote.snapshot"
	raftState       = "raft/"
)

// Server is Consul server which manages the service discovery,
// health checking, DC forwarding, Raft, and multiple Serf pools.
type Server struct {
	config *Config

	// Connection pool to other consul servers
	connPool *ConnPool

	// eventChLAN is used to receive events from the
	// serf cluster in the datacenter
	eventChLAN chan serf.Event

	// eventChWAN is used to receive events from the
	// serf cluster that spans datacenters
	eventChWAN chan serf.Event

	// fsm is the state machine used with Raft to provide
	// strong consistency.
	fsm *consulFSM

	// Logger uses the provided LogOutput
	logger *log.Logger

	// The raft instance is used among Consul nodes within the
	// DC to protect operations that require strong consistency
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftPeers     raft.PeerStore
	raftStore     *raft.MDBStore
	raftTransport *raft.NetworkTransport

	// remoteConsuls is used to track the known consuls in
	// remote data centers. Used to do DC forwarding.
	remoteConsuls map[string][]net.Addr
	remoteLock    sync.RWMutex

	// rpcClients is used to track active clients
	rpcClients    map[net.Conn]struct{}
	rpcClientLock sync.Mutex

	// rpcListener is used to listen for incoming connections
	rpcListener net.Listener
	rpcServer   *rpc.Server

	// serfLAN is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serfLAN *serf.Serf

	// serfWAN is the Serf cluster maintained between DC's
	// which SHOULD only consist of Consul servers
	serfWAN *serf.Serf

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewServer is used to construct a new Consul server from the
// configuration, potentially returning an error
func NewServer(config *Config) (*Server, error) {
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
	s := &Server{
		config:        config,
		connPool:      NewPool(5),
		eventChLAN:    make(chan serf.Event, 256),
		eventChWAN:    make(chan serf.Event, 256),
		logger:        logger,
		remoteConsuls: make(map[string][]net.Addr),
		rpcClients:    make(map[net.Conn]struct{}),
		rpcServer:     rpc.NewServer(),
		shutdownCh:    make(chan struct{}),
	}

	// Initialize the RPC layer
	if err := s.setupRPC(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Start the Serf listeners to prevent a deadlock
	go s.lanEventHandler()
	go s.wanEventHandler()

	// Initialize the lan Serf
	var err error
	s.serfLAN, err = s.setupSerf(config.SerfLANConfig,
		s.eventChLAN, serfLANSnapshot)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}

	// Initialize the wan Serf
	s.serfWAN, err = s.setupSerf(config.SerfWANConfig,
		s.eventChWAN, serfWANSnapshot)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start wan serf: %v", err)
	}

	return s, nil
}

// ensurePath is used to make sure a path exists
func (s *Server) ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// setupSerf is used to setup and initialize a Serf
func (s *Server) setupSerf(conf *serf.Config, ch chan serf.Event, path string) (*serf.Serf, error) {
	addr := s.rpcListener.Addr().(*net.TCPAddr)
	conf.NodeName = s.config.NodeName
	conf.Role = fmt.Sprintf("consul:%s:%d", s.config.Datacenter, addr.Port)
	conf.MemberlistConfig.LogOutput = s.config.LogOutput
	conf.LogOutput = s.config.LogOutput
	conf.EventCh = ch
	conf.SnapshotPath = filepath.Join(s.config.DataDir, path)
	if err := s.ensurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}
	return serf.Create(conf)
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// Create the base path
	path := filepath.Join(s.config.DataDir, raftState)
	if err := s.ensurePath(path, true); err != nil {
		return err
	}

	// Create the FSM
	var err error
	s.fsm, err = NewFSM()
	if err != nil {
		return err
	}

	// Create the MDB store for logs and stable storage
	store, err := raft.NewMDBStore(path)
	if err != nil {
		return err
	}
	s.raftStore = store

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(path, 3)
	if err != nil {
		store.Close()
		return err
	}

	// Create a transport layer
	trans := raft.NewNetworkTransport(s.raftLayer, 3, 10*time.Second)
	s.raftTransport = trans

	// Setup the peer store
	s.raftPeers = raft.NewJSONPeers(path, trans)

	// Setup the Raft store
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, store, store,
		snapshots, s.raftPeers, trans)
	if err != nil {
		store.Close()
		trans.Close()
		return err
	}
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC() error {
	// Register the handlers
	s.rpcServer.Register(&Status{server: s})
	s.rpcServer.Register(&Raft{server: s})
	s.rpcServer.Register(&Catalog{s})

	list, err := net.Listen("tcp", s.config.RPCAddr)
	if err != nil {
		return err
	}
	s.rpcListener = list
	s.raftLayer = NewRaftLayer(s.rpcListener.Addr())
	go s.listen()
	return nil
}

// Shutdown is used to shutdown the server
func (s *Server) Shutdown() error {
	s.logger.Printf("[INFO] Shutting down Consul server")
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
			s.logger.Printf("[WARN] Error shutting down raft: %s", err)
		}
		s.raftStore.Close()
	}

	if s.rpcListener != nil {
		s.rpcListener.Close()
	}

	// Close all the RPC connections
	s.rpcClientLock.Lock()
	for conn := range s.rpcClients {
		conn.Close()
	}
	s.rpcClientLock.Unlock()

	// Close the connection pool
	s.connPool.Shutdown()

	return nil
}

// Leave is used to prepare for a graceful shutdown of the server
func (s *Server) Leave() error {
	s.logger.Printf("[INFO] Consul server starting leave")

	// Leave the WAN pool
	if s.serfWAN != nil {
		if err := s.serfWAN.Leave(); err != nil {
			s.logger.Printf("[ERR] Failed to leave WAN Serf cluster: %v", err)
		}
	}

	// Leave the LAN pool
	if s.serfLAN != nil {
		if err := s.serfLAN.Leave(); err != nil {
			s.logger.Printf("[ERR] Failed to leave LAN Serf cluster: %v", err)
		}
	}

	// Leave the Raft cluster
	if s.raft != nil {
		// Get the leader
		leader := s.raft.Leader()
		if leader == nil {
			s.logger.Printf("[ERR] Failed to leave Raft cluster: no leader")
			goto AFTER_LEAVE
		}

		// Request that we are removed
		ch := make(chan error, 1)
		go func() {
			var out struct{}
			peer := s.rpcListener.Addr().String()
			err := s.connPool.RPC(leader, "Raft.RemovePeer", peer, &out)
			ch <- err
		}()

		// Wait for the commit
		select {
		case err := <-ch:
			if err != nil {
				s.logger.Printf("[ERR] Failed to leave Raft cluster: %v", err)
			}
		case <-time.After(3 * time.Second):
			s.logger.Printf("[ERR] Timedout leaving Raft cluster")
		}
	}
AFTER_LEAVE:
	return nil
}

// JoinLAN is used to have Consul join the inner-DC pool
// The target address should be another node inside the DC
// listening on the Serf LAN address
func (s *Server) JoinLAN(addr string) error {
	_, err := s.serfLAN.Join([]string{addr}, false)
	return err
}

// JoinWAN is used to have Consul join the cross-WAN Consul ring
// The target address should be another node listening on the
// Serf WAN address
func (s *Server) JoinWAN(addr string) error {
	_, err := s.serfWAN.Join([]string{addr}, false)
	return err
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
