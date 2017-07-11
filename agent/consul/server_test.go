package consul

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/agent"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
)

func getPort() int {
	return 1030 + int(rand.Int31n(64400))
}

func configureTLS(config *Config) {
	config.CAFile = "../../test/ca/root.cer"
	config.CertFile = "../../test/key/ourdomain.cer"
	config.KeyFile = "../../test/key/ourdomain.key"
}

func testServerConfig(t *testing.T, NodeName string) (string, *Config) {
	dir := testutil.TempDir(t, "consul")
	config := DefaultConfig()

	config.NodeName = NodeName
	config.Bootstrap = true
	config.Datacenter = "dc1"
	config.DataDir = dir
	config.RPCAddr = &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: getPort(),
	}
	config.RPCAdvertise = config.RPCAddr
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
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d s1 LAN members want %d", got, want)
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
	})
}

func TestServer_JoinWAN(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 2; got != want {
			r.Fatalf("got %d s1 WAN members want %d", got, want)
		}
		if got, want := len(s2.WANMembers()), 2; got != want {
			r.Fatalf("got %d s2 WAN members want %d", got, want)
		}
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.router.GetDatacenters()), 2; got != want {
			r.Fatalf("got %d routes want %d", got, want)
		}
		if got, want := len(s2.router.GetDatacenters()), 2; got != want {
			r.Fatalf("got %d datacenters want %d", got, want)
		}
	})
}

func TestServer_JoinWAN_Flood(t *testing.T) {
	// Set up two servers in a WAN.
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	joinWAN(t, s2, s1)

	for _, s := range []*Server{s1, s2} {
		retry.Run(t, func(r *retry.R) {
			if got, want := len(s.WANMembers()), 2; got != want {
				r.Fatalf("got %d WAN members want %d", got, want)
			}
		})
	}

	dir3, s3 := testServer(t)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Do just a LAN join for the new server and make sure it
	// shows up in the WAN.
	joinLAN(t, s3, s1)

	for _, s := range []*Server{s1, s2, s3} {
		retry.Run(t, func(r *retry.R) {
			if got, want := len(s.WANMembers()), 3; got != want {
				r.Fatalf("got %d WAN members want %d", got, want)
			}
		})
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
	joinWAN(t, s2, s1)

	// Join s3 to s2 on lan
	joinLAN(t, s3, s2)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 2; got != want {
			r.Fatalf("got %d s1 WAN members want %d", got, want)
		}
		if got, want := len(s2.WANMembers()), 2; got != want {
			r.Fatalf("got %d s2 WAN members want %d", got, want)
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
		if got, want := len(s3.LANMembers()), 2; got != want {
			r.Fatalf("got %d s3 WAN members want %d", got, want)
		}
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		if len(s1.router.GetDatacenters()) != 2 {
			r.Fatalf("remote consul missing")
		}
		if len(s2.router.GetDatacenters()) != 2 {
			r.Fatalf("remote consul missing")
		}
		if len(s2.localConsuls) != 2 {
			r.Fatalf("local consul fellow s3 for s2 missing")
		}
	})

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
	joinLAN(t, s2, s1)

	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 2))
		r.Check(wantPeers(s2, 2))
	})

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
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 1))
		r.Check(wantPeers(s2, 1))
	})
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
	joinLAN(t, s2, s1)

	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 2))
		r.Check(wantPeers(s2, 2))
	})

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
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 1))
		r.Check(wantPeers(s2, 1))
	})
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
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembers()), 2; got != want {
			r.Fatalf("got %d s1 LAN members want %d", got, want)
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
	})
	// Verify Raft has established a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 2))
		r.Check(wantPeers(s2, 2))
	})
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
	joinLAN(t, s2, s1)

	// Should have no peers yet since the bootstrap didn't occur.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 0))
		r.Check(wantPeers(s2, 0))
	})

	// Join the third node.
	joinLAN(t, s3, s1)

	// Now we have three servers so we should bootstrap.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 3))
		r.Check(wantPeers(s2, 3))
		r.Check(wantPeers(s3, 3))
	})

	// Make sure a leader is elected, grab the current term and then add in
	// the fourth server.
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	termBefore := s1.raft.Stats()["last_log_term"]
	joinLAN(t, s4, s1)

	// Wait for the new server to see itself added to the cluster.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 4))
		r.Check(wantPeers(s2, 4))
		r.Check(wantPeers(s3, 4))
		r.Check(wantPeers(s4, 4))
	})

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
	joinLAN(t, s2, s1)

	// should have no peers yet
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 0))
		r.Check(wantPeers(s2, 0))
	})

	// join the third node
	joinLAN(t, s3, s1)

	// should still have no peers (because s2 is in expect=2 mode)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 0))
		r.Check(wantPeers(s2, 0))
		r.Check(wantPeers(s3, 0))
	})
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
	retry.Run(t, func(r *retry.R) {
		if len(s1.router.GetDatacenters()) != 1 {
			r.Fatal(nil)
		}
	})

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

func testVerifyRPC(s1, s2 *Server, t *testing.T) (bool, error) {
	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the members
	retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s1, 2)) })

	// Have s2 make an RPC call to s1
	s2.localLock.RLock()
	var leader *agent.Server
	for _, server := range s2.localConsuls {
		if server.Name == s1.config.NodeName {
			leader = server
		}
	}
	s2.localLock.RUnlock()
	if leader == nil {
		t.Fatal("no leader")
	}
	return s2.connPool.Ping(leader.Datacenter, leader.Addr, leader.Version, leader.UseTLS)
}

func TestServer_TLSToNoTLS(t *testing.T) {
	// Set up a server with no TLS configured
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS configured
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.CAFile = "../../test/client_certs/rootca.crt"
		c.CertFile = "../../test/client_certs/server.crt"
		c.KeyFile = "../../test/client_certs/server.key"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	success, err := testVerifyRPC(s1, s2, t)
	if err != nil {
		t.Fatal(err)
	}
	if !success {
		t.Fatalf("bad: %v", success)
	}
}

func TestServer_TLSForceOutgoingToNoTLS(t *testing.T) {
	// Set up a server with no TLS configured
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS and VerifyOutgoing set
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.CAFile = "../../test/client_certs/rootca.crt"
		c.CertFile = "../../test/client_certs/server.crt"
		c.KeyFile = "../../test/client_certs/server.key"
		c.VerifyOutgoing = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	_, err := testVerifyRPC(s1, s2, t)
	if err == nil || !strings.Contains(err.Error(), "remote error: tls") {
		t.Fatalf("should fail")
	}
}

func TestServer_TLSToFullVerify(t *testing.T) {
	// Set up a server with TLS and VerifyIncoming set
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.CAFile = "../../test/client_certs/rootca.crt"
		c.CertFile = "../../test/client_certs/server.crt"
		c.KeyFile = "../../test/client_certs/server.key"
		c.VerifyIncoming = true
		c.VerifyOutgoing = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS configured
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.CAFile = "../../test/client_certs/rootca.crt"
		c.CertFile = "../../test/client_certs/server.crt"
		c.KeyFile = "../../test/client_certs/server.key"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	success, err := testVerifyRPC(s1, s2, t)
	if err != nil {
		t.Fatal(err)
	}
	if !success {
		t.Fatalf("bad: %v", success)
	}
}
