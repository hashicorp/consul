package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"
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
	return testServerDC(t, "dc1")
}

func testServerDC(t *testing.T, dc string) (string, *Server) {
	dir := tmpDir(t)
	config := DefaultConfig()
	config.Datacenter = dc
	config.DataDir = dir

	// Adjust the ports
	p := getPort()
	config.NodeName = fmt.Sprintf("Node %d", p)
	config.RPCAddr = fmt.Sprintf("127.0.0.1:%d", p)
	config.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfLANConfig.MemberlistConfig.Port = getPort()
	config.SerfLANConfig.MemberlistConfig.ProbeTimeout = 200 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.ProbeInterval = time.Second
	config.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfWANConfig.MemberlistConfig.Port = getPort()
	config.SerfWANConfig.MemberlistConfig.ProbeTimeout = 200 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.ProbeInterval = time.Second
	config.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 40 * time.Millisecond

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

	// Check the members
	if len(s1.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	if len(s2.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}
}

func TestServer_JoinWAN(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.Port)
	if err := s2.JoinWAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the members
	if len(s1.WANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	if len(s2.WANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	time.Sleep(10 * time.Millisecond)

	// Check the remoteConsuls has both
	if len(s1.remoteConsuls) != 2 {
		t.Fatalf("remote consul missing")
	}

	if len(s2.remoteConsuls) != 2 {
		t.Fatalf("remote consul missing")
	}
}

func TestServer_Leave(t *testing.T) {
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

	time.Sleep(time.Second)

	p1, _ := s1.raftPeers.Peers()
	if len(p1) != 2 {
		t.Fatalf("should have 2 peers: %v", p1)
	}

	p2, _ := s2.raftPeers.Peers()
	if len(p2) != 2 {
		t.Fatalf("should have 2 peers: %v", p2)
	}

	// Issue a leave
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should lose a peer
	p1, _ = s1.raftPeers.Peers()
	if len(p1) != 1 {
		t.Fatalf("should have 1 peer: %v", p1)
	}
}

func TestServer_RPC(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	var out struct{}
	if err := s1.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}
