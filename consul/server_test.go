package consul

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
)

var nextPort int32 = 15000

func getPort() int {
	return int(atomic.AddInt32(&nextPort, 1))
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func configureTLS(config *Config) {
	config.CAFile = "../test/ca/root.cer"
	config.CertFile = "../test/key/ourdomain.cer"
	config.KeyFile = "../test/key/ourdomain.key"
}

func testServerConfig(t *testing.T, NodeName string) (string, *Config) {
	dir := tmpDir(t)
	config := DefaultConfig()

	config.NodeName = NodeName
	config.Bootstrap = true
	config.Datacenter = "dc1"
	config.DataDir = dir
	config.RPCAddr = &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: getPort(),
	}
	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	config.NodeID = types.NodeID(nodeID)
	config.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfLANConfig.MemberlistConfig.BindPort = getPort()
	config.SerfLANConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfLANConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfWANConfig.MemberlistConfig.BindPort = getPort()
	config.SerfWANConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfWANConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	config.ReconcileInterval = 100 * time.Millisecond

	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

	config.Build = "0.8.0"

	config.CoordinateUpdatePeriod = 100 * time.Millisecond
	return dir, config
}

func testServer(t *testing.T) (string, *Server) {
	return testServerDC(t, "dc1")
}

func testServerDC(t *testing.T, dc string) (string, *Server) {
	return testServerDCBootstrap(t, dc, true)
}

func testServerDCBootstrap(t *testing.T, dc string, bootstrap bool) (string, *Server) {
	name := fmt.Sprintf("Node %d", getPort())
	dir, config := testServerConfig(t, name)
	config.Datacenter = dc
	config.Bootstrap = bootstrap
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func testServerDCExpect(t *testing.T, dc string, expect int) (string, *Server) {
	name := fmt.Sprintf("Node %d", getPort())
	dir, config := testServerConfig(t, name)
	config.Datacenter = dc
	config.Bootstrap = false
	config.BootstrapExpect = expect
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func testServerWithConfig(t *testing.T, cb func(c *Config)) (string, *Server) {
	name := fmt.Sprintf("Node %d", getPort())
	dir, config := testServerConfig(t, name)
	cb(config)
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func TestServer_StartStop(t *testing.T) {
	// Start up a server and then stop it.
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Shut down again, which should be idempotent.
	if err := s1.Shutdown(); err != nil {
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
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Check the members
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s1.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s1 want %d", got, want)
			continue
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s2 want %d", got, want)
			continue
		}
		break
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
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Check the members
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s1.WANMembers()), 2; got != want {
			t.Logf("got %d WAN members for s1 want %d", got, want)
			continue
		}
		if got, want := len(s2.WANMembers()), 2; got != want {
			t.Logf("got %d WAN members for s2 want %d", got, want)
			continue
		}
		break
	}

	// Check the router has both
	if len(s1.router.GetDatacenters()) != 2 {
		t.Fatalf("remote consul missing")
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s2.router.GetDatacenters()), 2; got != want {
			t.Logf("got %d datacenters want %d", got, want)
			continue
		}
		break
	}
}

func TestServer_JoinWAN_Flood(t *testing.T) {
	// Set up two servers in a WAN.
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range []*Server{s1, s2} {
		for r := retry.OneSec(); r.NextOr(t.FailNow); {
			if got, want := len(s.WANMembers()), 2; got != want {
				t.Logf("got %d WAN members want %d", got, want)
				continue
			}
			break
		}
	}

	dir3, s3 := testServer(t)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Do just a LAN join for the new server and make sure it
	// shows up in the WAN.
	addr = fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range []*Server{s1, s2, s3} {
		for r := retry.OneSec(); r.NextOr(t.FailNow); {
			if got, want := len(s.WANMembers()), 3; got != want {
				t.Logf("got %d WAN members want %d", got, want)
				continue
			}
			break
		}
	}
}

func TestServer_JoinSeparateLanAndWanAddresses(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = "s2"
		c.Datacenter = "dc2"
		// This wan address will be expected to be seen on s1
		c.SerfWANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.2"
		// This lan address will be expected to be seen on s3
		c.SerfLANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.3"
	})

	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Join s2 to s1 on wan
	addrs1 := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addrs1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Join s3 to s2 on lan
	addrs2 := fmt.Sprintf("127.0.0.1:%d",
		s2.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s3.JoinLAN([]string{addrs2}); err != nil {
		t.Fatalf("err: %v", err)
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s1.WANMembers()), 3; got != want {
			t.Logf("got %d WAN members for s1 want %d", got, want)
			continue
		}
		if got, want := len(s2.WANMembers()), 3; got != want {
			t.Logf("got %d WAN members for s2 want %d", got, want)
			continue
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s2 want %d", got, want)
			continue
		}
		if got, want := len(s3.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s3 want %d", got, want)
			continue
		}
		break
	}

	// Check the router has both
	if len(s1.router.GetDatacenters()) != 2 {
		t.Fatalf("remote consul missing")
	}

	if len(s2.router.GetDatacenters()) != 2 {
		t.Fatalf("remote consul missing")
	}

	if len(s2.localConsuls) != 2 {
		t.Fatalf("local consul fellow s3 for s2 missing")
	}

	// Get and check the wan address of s2 from s1
	var s2WanAddr string
	for _, member := range s1.WANMembers() {
		if member.Name == "s2.dc2" {
			s2WanAddr = member.Addr.String()
		}
	}
	if s2WanAddr != "127.0.0.2" {
		t.Fatalf("s1 sees s2 on a wrong address: %s, expecting: %s", s2WanAddr, "127.0.0.2")
	}

	// Get and check the lan address of s2 from s3
	var s2LanAddr string
	for _, lanmember := range s3.LANMembers() {
		if lanmember.Name == "s2" {
			s2LanAddr = lanmember.Addr.String()
		}
	}
	if s2LanAddr != "127.0.0.3" {
		t.Fatalf("s3 sees s2 on a wrong address: %s, expecting: %s", s2LanAddr, "127.0.0.3")
	}
}

func TestServer_LeaveLeader(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Second server not in bootstrap mode
	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 2; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 2; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}

	// Issue a leave to the leader
	for _, s := range []*Server{s1, s2} {
		if !s.IsLeader() {
			continue
		}
		if err := s.Leave(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Should lose a peer
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 1; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 1; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}
}

func TestServer_Leave(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Second server not in bootstrap mode
	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 2; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 2; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}

	// Issue a leave to the non-leader
	for _, s := range []*Server{s1, s2} {
		if s.IsLeader() {
			continue
		}
		if err := s.Leave(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Should lose a peer
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 1; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 1; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
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

func TestServer_JoinLAN_TLS(t *testing.T) {
	dir1, conf1 := testServerConfig(t, "a.testco.internal")
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := NewServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, conf2 := testServerConfig(t, "b.testco.internal")
	conf2.Bootstrap = false
	conf2.VerifyIncoming = true
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	s2, err := NewServer(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s1.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s1 want %d", got, want)
			continue
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			t.Logf("got %d LAN members for s2 want %d", got, want)
			continue
		}
		p1, _ := s1.numPeers()
		if got, want := p1, 2; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 2; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}
}

func TestServer_Expect(t *testing.T) {
	// All test servers should be in expect=3 mode, except for the 3rd one,
	// but one with expect=0 can cause a bootstrap to occur from the other
	// servers as currently implemented.
	dir1, s1 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCExpect(t, "dc1", 0)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	// Join the first two servers.
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have no peers yet since the bootstrap didn't occur.
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 0; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 0; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}

	// Join the third node.
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 3; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 3; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		p3, _ := s3.numPeers()
		if got, want := p3, 3; got != want {
			t.Logf("got %d peers for s3 want %d", got, want)
			continue
		}
		break
	}

	// Make sure a leader is elected, grab the current term and then add in
	// the fourth server.
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	termBefore := s1.raft.Stats()["last_log_term"]
	if _, err := s4.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the new server to see itself added to the cluster.
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 4; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 4; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		p3, _ := s3.numPeers()
		if got, want := p3, 4; got != want {
			t.Logf("got %d peers for s3 want %d", got, want)
			continue
		}
		p4, _ := s4.numPeers()
		if got, want := p4, 4; got != want {
			t.Logf("got %d peers for s4 want %d", got, want)
			continue
		}
		break
	}

	// Make sure there's still a leader and that the term didn't change,
	// so we know an election didn't occur.
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	termAfter := s1.raft.Stats()["last_log_term"]
	if termAfter != termBefore {
		t.Fatalf("looks like an election took place")
	}
}

func TestServer_BadExpect(t *testing.T) {
	// this one is in expect=3 mode
	dir1, s1 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// this one is in expect=2 mode
	dir2, s2 := testServerDCExpect(t, "dc1", 2)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// and this one is in expect=3 mode
	dir3, s3 := testServerDCExpect(t, "dc1", 3)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 0; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 0; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		break
	}

	// join the third node
	if _, err := s3.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// should still have no peers (because s2 is in expect=2 mode)
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		p1, _ := s1.numPeers()
		if got, want := p1, 0; got != want {
			t.Logf("got %d peers for s1 want %d", got, want)
			continue
		}
		p2, _ := s2.numPeers()
		if got, want := p2, 0; got != want {
			t.Logf("got %d peers for s2 want %d", got, want)
			continue
		}
		p3, _ := s3.numPeers()
		if got, want := p3, 0; got != want {
			t.Logf("got %d peers for s3 want %d", got, want)
			continue
		}
		break
	}
}

type fakeGlobalResp struct{}

func (r *fakeGlobalResp) Add(interface{}) {
	return
}

func (r *fakeGlobalResp) New() interface{} {
	return struct{}{}
}

func TestServer_globalRPCErrors(t *testing.T) {
	dir1, s1 := testServerDC(t, "dc1")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	for r := retry.OneSec(); r.NextOr(t.FailNow); {
		if got, want := len(s1.router.GetDatacenters()), 1; got != want {
			t.Logf("got %d datacenters want %d", got, want)
			continue
		}
		break
	}

	// Check that an error from a remote DC is returned
	err := s1.globalRPC("Bad.Method", nil, &fakeGlobalResp{})
	if err == nil {
		t.Fatalf("should have errored")
	}
	if !strings.Contains(err.Error(), "Bad.Method") {
		t.Fatalf("unexpcted error: %s", err)
	}
}

func TestServer_Encrypted(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	key := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = key
		c.SerfWANConfig.MemberlistConfig.SecretKey = key
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	if s1.Encrypted() {
		t.Fatalf("should not be encrypted")
	}
	if !s2.Encrypted() {
		t.Fatalf("should be encrypted")
	}
}
