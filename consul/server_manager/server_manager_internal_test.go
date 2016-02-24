package server_manager

import (
	"bytes"
	"log"
	"testing"

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

func testServerManager() (sm *ServerManager) {
	logger := GetBufferedLogger()
	shutdownCh := make(chan struct{})
	sm = NewServerManager(logger, shutdownCh, nil)
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

// func NewServerManager(logger *log.Logger, shutdownCh chan struct{}) (sm *ServerManager) {
func TestServerManagerInternal_NewServerManager(t *testing.T) {
	sm := testServerManager()
	if sm == nil {
		t.Fatalf("ServerManager nil")
	}

	if sm.logger == nil {
		t.Fatalf("ServerManager.logger nil")
	}

	if sm.consulServersCh == nil {
		t.Fatalf("ServerManager.consulServersCh nil")
	}

	if sm.shutdownCh == nil {
		t.Fatalf("ServerManager.shutdownCh nil")
	}
}

// func (sc *serverConfig) resetRebalanceTimer(sm *ServerManager) {

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
