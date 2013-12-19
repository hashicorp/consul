package consul

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func testClient(t *testing.T) (string, *Client) {
	return testClientDC(t, "dc1")
}

func testClientDC(t *testing.T, dc string) (string, *Client) {
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

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, client
}

func TestClient_StartStop(t *testing.T) {
	dir, client := testClient(t)
	defer os.RemoveAll(dir)

	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClient_JoinLAN(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.Port)
	if err := c1.JoinLAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the members
	if len(s1.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	if len(c1.LANMembers()) != 2 {
		t.Fatalf("bad len")
	}

	time.Sleep(10 * time.Millisecond)

	// Check we have a new consul
	if len(c1.consuls) != 1 {
		t.Fatalf("expected consul server")
	}
}
