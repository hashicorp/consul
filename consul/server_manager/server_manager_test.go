package server_manager_test

import (
	"bytes"
	"log"
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

func makeMockServerManager() (sm *server_manager.ServerManager) {
	logger, shutdownCh := mockServerManager()
	sm = server_manager.NewServerManager(logger, shutdownCh, nil)
	return sm
}

func mockServerManager() (logger *log.Logger, shutdownCh chan struct{}) {
	logger = GetBufferedLogger()
	shutdownCh = make(chan struct{})
	return logger, shutdownCh
}

// func (sm *ServerManager) AddServer(server *server_details.ServerDetails) {

// func (sm *ServerManager) CycleFailedServers() {
// func (sm *ServerManager) FindServer() (server *server_details.ServerDetails) {
func TestServerManager_FindServer(t *testing.T) {
	sm := testServerManager()

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

// func (sm *ServerManager) GetNumServers() (numServers int) {
func TestServerManager_GetNumServers(t *testing.T) {
	sm := makeMockServerManager()
	var num int
	num = sm.GetNumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s := &server_details.ServerDetails{}
	sm.AddServer(s)
	num = sm.GetNumServers()
	if num != 1 {
		t.Fatalf("Expected one server after AddServer")
	}
}

// func NewServerManager(logger *log.Logger, shutdownCh chan struct{}) (sm *ServerManager) {
func TestServerManager_NewServerManager(t *testing.T) {
	sm := makeMockServerManager()
	if sm == nil {
		t.Fatalf("ServerManager nil")
	}
}

// func (sm *ServerManager) NotifyFailedServer(server *server_details.ServerDetails) {

// func (sm *ServerManager) RebalanceServers() {

// func (sm *ServerManager) RemoveServer(server *server_details.ServerDetails) {

// func (sm *ServerManager) Start() {
