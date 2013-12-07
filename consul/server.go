package consul

import (
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"log"
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
	raftStore     *raft.SQLiteStore
	raftTransport *raft.NetworkTransport

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
		config:     config,
		eventChLAN: make(chan serf.Event, 256),
		eventChWAN: make(chan serf.Event, 256),
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}

	// Start the Serf listeners to prevent a deadlock
	go s.lanEventHandler()
	go s.wanEventHandler()

	// Initialize the lan Serf
	var err error
	s.serfLAN, err = s.setupSerf(config.SerfLocalConfig,
		s.eventChLAN, serfLANSnapshot)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start lan serf: %v", err)
	}

	// Initialize the wan Serf
	s.serfWAN, err = s.setupSerf(config.SerfRemoteConfig,
		s.eventChWAN, serfWANSnapshot)
	if err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start wan serf: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
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
	conf.NodeName = s.config.NodeName
	conf.Role = fmt.Sprintf("consul:%s", s.config.Datacenter)
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

	// Create the SQLite store for logs and stable storage
	store, err := raft.NewSQLiteStore(path)
	if err != nil {
		return err
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(path, 3)
	if err != nil {
		store.Close()
		return err
	}

	// Create a transport layer
	trans, err := raft.NewTCPTransport(s.config.RaftBindAddr, 3, 10*time.Second)
	if err != nil {
		store.Close()
		return err
	}

	// Setup the peer store
	peers := raft.NewJSONPeers(path, trans)

	// Create the FSM
	s.fsm = &consulFSM{server: s}

	// Setup the Raft store
	raft, err := raft.NewRaft(s.config.RaftConfig, s.fsm, store, store, snapshots,
		peers, trans)
	if err != nil {
		store.Close()
		trans.Close()
		return err
	}

	s.raft = raft
	s.raftStore = store
	s.raftTransport = trans
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
		s.serfLAN = nil
	}

	if s.serfWAN != nil {
		s.serfWAN.Shutdown()
		s.serfWAN = nil
	}

	if s.raft != nil {
		s.raft.Shutdown()
		s.raftStore.Close()
		s.raftTransport.Close()
		s.raft = nil
		s.raftStore = nil
		s.raftTransport = nil
	}
	return nil
}

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			s.logger.Printf("[INFO] LAN Event: %v", e)
		case <-s.shutdownCh:
			return
		}
	}
}

// wanEventHandler is used to handle events from the wan Serf cluster
func (s *Server) wanEventHandler() {
	for {
		select {
		case e := <-s.eventChWAN:
			s.logger.Printf("[INFO] WAN Event: %v", e)
		case <-s.shutdownCh:
			return
		}
	}
}
