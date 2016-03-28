package server_manager_test

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/server_details"
	"github.com/hashicorp/consul/consul/server_manager"
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
	// failPct between 0.0 and 1.0 == pct of time a Ping should fail
	failPct float64
}

func (cp *fauxConnPool) PingConsulServer(server *server_details.ServerDetails) (bool, error) {
	var success bool
	successProb := rand.Float64()
	if successProb > cp.failPct {
		success = true
	}
	return success, nil
}

type fauxSerf struct {
}

func (s *fauxSerf) NumNodes() int {
	return 16384
}

func testServerManager() (sm *server_manager.ServerManager) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	sm = server_manager.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{})
	return sm
}

func testServerManagerFailProb(failPct float64) (sm *server_manager.ServerManager) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	sm = server_manager.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{failPct: failPct})
	return sm
}

// func (sm *ServerManager) AddServer(server *server_details.ServerDetails) {
func TestServerManager_AddServer(t *testing.T) {
	sm := testServerManager()
	var num int
	num = sm.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &server_details.ServerDetails{Name: "s1"}
	sm.AddServer(s1)
	num = sm.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server")
	}

	sm.AddServer(s1)
	num = sm.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server (still)")
	}

	s2 := &server_details.ServerDetails{Name: "s2"}
	sm.AddServer(s2)
	num = sm.NumServers()
	if num != 2 {
		t.Fatalf("Expected two servers")
	}
}

// func (sm *ServerManager) FindServer() (server *server_details.ServerDetails) {
func TestServerManager_FindServer(t *testing.T) {
	sm := testServerManager()

	if sm.FindServer() != nil {
		t.Fatalf("Expected nil return")
	}

	sm.AddServer(&server_details.ServerDetails{Name: "s1"})
	if sm.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	s1 := sm.FindServer()
	if s1 == nil {
		t.Fatalf("Expected non-nil server")
	}
	if s1.Name != "s1" {
		t.Fatalf("Expected s1 server")
	}

	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	sm.AddServer(&server_details.ServerDetails{Name: "s2"})
	if sm.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}
	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	sm.NotifyFailedServer(s1)
	s2 := sm.FindServer()
	if s2 == nil || s2.Name != "s2" {
		t.Fatalf("Expected s2 server")
	}

	sm.NotifyFailedServer(s2)
	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server")
	}
}

// func New(logger *log.Logger, shutdownCh chan struct{}) (sm *ServerManager) {
func TestServerManager_New(t *testing.T) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	sm := server_manager.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{})
	if sm == nil {
		t.Fatalf("ServerManager nil")
	}
}

// func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {
func TestServerManager_NotifyFailedServer(t *testing.T) {
	sm := testServerManager()

	if sm.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &server_details.ServerDetails{Name: "s1"}
	s2 := &server_details.ServerDetails{Name: "s2"}

	// Try notifying for a server that is not part of the server manager
	sm.NotifyFailedServer(s1)
	if sm.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}
	sm.AddServer(s1)

	// Test again w/ a server not in the list
	sm.NotifyFailedServer(s2)
	if sm.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	sm.AddServer(s2)
	if sm.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}

	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server")
	}

	sm.NotifyFailedServer(s2)
	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	sm.NotifyFailedServer(s1)
	s2 = sm.FindServer()
	if s2 == nil || s2.Name != "s2" {
		t.Fatalf("Expected s2 server")
	}

	sm.NotifyFailedServer(s2)
	s1 = sm.FindServer()
	if s1 == nil || s1.Name != "s1" {
		t.Fatalf("Expected s1 server")
	}
}

// func (sm *ServerManager) NumServers() (numServers int) {
func TestServerManager_NumServers(t *testing.T) {
	sm := testServerManager()
	var num int
	num = sm.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s := &server_details.ServerDetails{}
	sm.AddServer(s)
	num = sm.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server after AddServer")
	}
}

// func (sm *ServerManager) RebalanceServers() {
func TestServerManager_RebalanceServers(t *testing.T) {
	const failPct = 0.5
	sm := testServerManagerFailProb(failPct)
	const maxServers = 100
	const numShuffleTests = 100
	const uniquePassRate = 0.5

	// Make a huge list of nodes.
	for i := 0; i < maxServers; i++ {
		nodeName := fmt.Sprintf("s%02d", i)
		sm.AddServer(&server_details.ServerDetails{Name: nodeName})
	}

	// Keep track of how many unique shuffles we get.
	uniques := make(map[string]struct{}, maxServers)
	for i := 0; i < numShuffleTests; i++ {
		sm.RebalanceServers()

		var names []string
		for j := 0; j < maxServers; j++ {
			server := sm.FindServer()
			sm.NotifyFailedServer(server)
			names = append(names, server.Name)
		}
		key := strings.Join(names, "|")
		uniques[key] = struct{}{}
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < int(maxServers*uniquePassRate) {
		t.Fatalf("unique shuffle ratio too low: %d/%d", len(uniques), maxServers)
	}
}

// func (sm *ServerManager) RemoveServer(server *server_details.ServerDetails) {
func TestServerManager_RemoveServer(t *testing.T) {
	const nodeNameFmt = "s%02d"
	sm := testServerManager()

	if sm.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Test removing server before its added
	nodeName := fmt.Sprintf(nodeNameFmt, 1)
	s1 := &server_details.ServerDetails{Name: nodeName}
	sm.RemoveServer(s1)
	sm.AddServer(s1)

	nodeName = fmt.Sprintf(nodeNameFmt, 2)
	s2 := &server_details.ServerDetails{Name: nodeName}
	sm.RemoveServer(s2)
	sm.AddServer(s2)

	const maxServers = 19
	servers := make([]*server_details.ServerDetails, maxServers)
	// Already added two servers above
	for i := maxServers; i > 2; i-- {
		nodeName := fmt.Sprintf(nodeNameFmt, i)
		server := &server_details.ServerDetails{Name: nodeName}
		servers = append(servers, server)
		sm.AddServer(server)
	}
	if sm.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, sm.NumServers())
	}

	sm.RebalanceServers()

	if sm.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, sm.NumServers())
	}

	findServer := func(server *server_details.ServerDetails) bool {
		for i := sm.NumServers(); i > 0; i-- {
			s := sm.FindServer()
			if s == server {
				return true
			}
		}
		return false
	}

	expectedNumServers := maxServers
	removedServers := make([]*server_details.ServerDetails, 0, maxServers)

	// Remove servers from the front of the list
	for i := 3; i > 0; i-- {
		server := sm.FindServer()
		if server == nil {
			t.Fatalf("FindServer returned nil")
		}
		sm.RemoveServer(server)
		expectedNumServers--
		if sm.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, sm.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s after removal from the front", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the end of the list
	for i := 3; i > 0; i-- {
		server := sm.FindServer()
		sm.NotifyFailedServer(server)
		sm.RemoveServer(server)
		expectedNumServers--
		if sm.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, sm.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the middle of the list
	for i := 3; i > 0; i-- {
		server := sm.FindServer()
		sm.NotifyFailedServer(server)
		server2 := sm.FindServer()
		sm.NotifyFailedServer(server2) // server2 now at end of the list

		sm.RemoveServer(server)
		expectedNumServers--
		if sm.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, sm.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	if sm.NumServers()+len(removedServers) != maxServers {
		t.Fatalf("Expected %d+%d=%d servers", sm.NumServers(), len(removedServers), maxServers)
	}

	// Drain the remaining servers from the middle
	for i := sm.NumServers(); i > 0; i-- {
		server := sm.FindServer()
		sm.NotifyFailedServer(server)
		server2 := sm.FindServer()
		sm.NotifyFailedServer(server2) // server2 now at end of the list
		sm.RemoveServer(server)
		removedServers = append(removedServers, server)
	}

	if sm.NumServers() != 0 {
		t.Fatalf("Expected an empty server list")
	}
	if len(removedServers) != maxServers {
		t.Fatalf("Expected all servers to be in removed server list")
	}
}

// func (sm *ServerManager) Start() {
