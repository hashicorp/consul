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

// func (sm *ServerManager) FindHealthyServer() (server *server_details.ServerDetails) {

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

// func (sm *ServerManager) StartServerManager() {
