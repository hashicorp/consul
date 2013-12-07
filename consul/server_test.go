package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

var nextPort = 15000

func getPort() int {
	p := nextPort
	nextPort++
	return p
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func testServer(t *testing.T) (string, *Server) {
	dir := tmpDir(t)
	config := DefaultConfig()
	config.DataDir = dir

	// Adjust the ports
	p := getPort()
	config.NodeName = fmt.Sprintf("Node %d", p)
	config.RaftBindAddr = fmt.Sprintf("127.0.0.1:%d", p)
	config.RPCAddr = fmt.Sprintf("127.0.0.1:%d", getPort())
	config.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfLANConfig.MemberlistConfig.Port = getPort()
	config.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfWANConfig.MemberlistConfig.Port = getPort()

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func TestServer_StartStop(t *testing.T) {
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	config := DefaultConfig()
	config.DataDir = dir

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := server.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Idempotent
	if err := server.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_JoinLAN(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.Port)
	if err := s2.JoinLAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_JoinWAN(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.Port)
	if err := s2.JoinWAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}
	t.Fatalf("fail")
}
