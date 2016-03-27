package server_manager

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/server_details"
)

var (
	localLogger    *log.Logger
	localLogBuffer *bytes.Buffer
)

func init() {
	localLogBuffer = new(bytes.Buffer)
	localLogger = log.New(localLogBuffer, "", 0)
}

func GetBufferedLogger() *log.Logger {
	return localLogger
}

type fauxConnPool struct {
}

func (s *fauxConnPool) PingConsulServer(server *server_details.ServerDetails) bool {
	return true
}

type fauxSerf struct {
	numNodes int
}

func (s *fauxSerf) NumNodes() int {
	return s.numNodes
}

func testServerManager() (sm *ServerManager) {
	logger := GetBufferedLogger()
	shutdownCh := make(chan struct{})
	sm = New(logger, shutdownCh, &fauxSerf{numNodes: 16384}, &fauxConnPool{})
	return sm
}

// func (sc *serverConfig) cycleServer() (servers []*server_details.ServerDetails) {
func TestServerManagerInternal_cycleServer(t *testing.T) {
	sm := testServerManager()
	sc := sm.getServerConfig()

	server0 := &server_details.ServerDetails{Name: "server1"}
	server1 := &server_details.ServerDetails{Name: "server2"}
	server2 := &server_details.ServerDetails{Name: "server3"}
	sc.servers = append(sc.servers, server0, server1, server2)
	sm.saveServerConfig(sc)

	sc = sm.getServerConfig()
	if len(sc.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(sc.servers))
	}
	if sc.servers[0] != server0 &&
		sc.servers[1] != server1 &&
		sc.servers[2] != server2 {
		t.Fatalf("initial server ordering not correct")
	}

	sc.servers = sc.cycleServer()
	if len(sc.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(sc.servers))
	}
	if sc.servers[0] != server1 &&
		sc.servers[1] != server2 &&
		sc.servers[2] != server0 {
		t.Fatalf("server ordering after one cycle not correct")
	}

	sc.servers = sc.cycleServer()
	if len(sc.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(sc.servers))
	}
	if sc.servers[0] != server2 &&
		sc.servers[1] != server0 &&
		sc.servers[2] != server1 {
		t.Fatalf("server ordering after two cycles not correct")
	}

	sc.servers = sc.cycleServer()
	if len(sc.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(sc.servers))
	}
	if sc.servers[0] != server0 &&
		sc.servers[1] != server1 &&
		sc.servers[2] != server2 {
		t.Fatalf("server ordering after three cycles not correct")
	}
}

// func (sm *ServerManager) getServerConfig() serverConfig {
func TestServerManagerInternal_getServerConfig(t *testing.T) {
	sm := testServerManager()
	sc := sm.getServerConfig()
	if sc.servers == nil {
		t.Fatalf("serverConfig.servers nil")
	}

	if len(sc.servers) != 0 {
		t.Fatalf("serverConfig.servers length not zero")
	}
}

// func New(logger *log.Logger, shutdownCh chan struct{}, clusterInfo ConsulClusterInfo) (sm *ServerManager) {
func TestServerManagerInternal_New(t *testing.T) {
	sm := testServerManager()
	if sm == nil {
		t.Fatalf("ServerManager nil")
	}

	if sm.clusterInfo == nil {
		t.Fatalf("ServerManager.clusterInfo nil")
	}

	if sm.logger == nil {
		t.Fatalf("ServerManager.logger nil")
	}

	if sm.shutdownCh == nil {
		t.Fatalf("ServerManager.shutdownCh nil")
	}
}

// func (sc *serverConfig) refreshServerRebalanceTimer(timer *time.Timer) {
func TestServerManagerInternal_refreshServerRebalanceTimer(t *testing.T) {
	sm := testServerManager()

	timer := time.NewTimer(time.Duration(1 * time.Nanosecond))
	time.Sleep(1 * time.Millisecond)
	sm.refreshServerRebalanceTimer(timer)

	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})

	type clusterSizes struct {
		numNodes     int
		numServers   int
		minRebalance time.Duration
	}
	clusters := []clusterSizes{
		{0, 3, 2 * time.Minute},
		{1, 0, 2 * time.Minute}, // partitioned cluster
		{1, 3, 2 * time.Minute},
		{2, 3, 2 * time.Minute},
		{100, 0, 2 * time.Minute}, // partitioned
		{100, 1, 2 * time.Minute}, // partitioned
		{100, 3, 2 * time.Minute},
		{1024, 1, 2 * time.Minute}, // partitioned
		{1024, 3, 2 * time.Minute}, // partitioned
		{1024, 5, 2 * time.Minute},
		{16384, 1, 4 * time.Minute}, // partitioned
		{16384, 2, 2 * time.Minute}, // partitioned
		{16384, 3, 2 * time.Minute}, // partitioned
		{16384, 5, 2 * time.Minute},
		{65535, 0, 2 * time.Minute}, // partitioned
		{65535, 1, 8 * time.Minute}, // partitioned
		{65535, 2, 3 * time.Minute}, // partitioned
		{65535, 3, 5 * time.Minute}, // partitioned
		{65535, 5, 3 * time.Minute}, // partitioned
		{65535, 7, 2 * time.Minute},
		{1000000, 1, 4 * time.Hour},     // partitioned
		{1000000, 2, 2 * time.Hour},     // partitioned
		{1000000, 3, 80 * time.Minute},  // partitioned
		{1000000, 5, 50 * time.Minute},  // partitioned
		{1000000, 11, 20 * time.Minute}, // partitioned
		{1000000, 19, 10 * time.Minute},
	}

	for _, s := range clusters {
		sm := New(logger, shutdownCh, &fauxSerf{numNodes: s.numNodes}, &fauxConnPool{})

		for i := 0; i < s.numServers; i++ {
			nodeName := fmt.Sprintf("s%02d", i)
			sm.AddServer(&server_details.ServerDetails{Name: nodeName})
		}

		d := sm.refreshServerRebalanceTimer(timer)
		if d < s.minRebalance {
			t.Errorf("duration too short for cluster of size %d and %d servers (%s < %s)", s.numNodes, s.numServers, d, s.minRebalance)
		}
	}
}

// func (sm *ServerManager) saveServerConfig(sc serverConfig) {
func TestServerManagerInternal_saveServerConfig(t *testing.T) {
	sm := testServerManager()

	// Initial condition
	func() {
		sc := sm.getServerConfig()
		if len(sc.servers) != 0 {
			t.Fatalf("ServerManager.saveServerConfig failed to load init config")
		}

		newServer := new(server_details.ServerDetails)
		sc.servers = append(sc.servers, newServer)
		sm.saveServerConfig(sc)
	}()

	// Test that save works
	func() {
		sc1 := sm.getServerConfig()
		t1NumServers := len(sc1.servers)
		if t1NumServers != 1 {
			t.Fatalf("ServerManager.saveServerConfig failed to save mutated config")
		}
	}()

	// Verify mutation w/o a save doesn't alter the original
	func() {
		newServer := new(server_details.ServerDetails)
		sc := sm.getServerConfig()
		sc.servers = append(sc.servers, newServer)

		sc_orig := sm.getServerConfig()
		origNumServers := len(sc_orig.servers)
		if origNumServers >= len(sc.servers) {
			t.Fatalf("ServerManager.saveServerConfig unsaved config overwrote original")
		}
	}()
}
