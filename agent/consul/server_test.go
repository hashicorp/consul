package consul

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib/freeport"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
)

func configureTLS(config *Config) {
	config.CAFile = "../../test/ca/root.cer"
	config.CertFile = "../../test/key/ourdomain.cer"
	config.KeyFile = "../../test/key/ourdomain.key"
}

var id int64

func uniqueNodeName(name string) string {
	return fmt.Sprintf("%s-node-%d", name, atomic.AddInt64(&id, 1))
}

func testServerConfig(t *testing.T) (string, *Config) {
	dir := testutil.TempDir(t, "consul")
	config := DefaultConfig()

	ports := freeport.Get(3)
	config.NodeName = uniqueNodeName(t.Name())
	config.Bootstrap = true
	config.Datacenter = "dc1"
	config.DataDir = dir

	// bind the rpc server to a random port. config.RPCAdvertise will be
	// set to the listen address unless it was set in the configuration.
	// In that case get the address from srv.Listener.Addr().
	config.RPCAddr = &net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: ports[0]}

	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	config.NodeID = types.NodeID(nodeID)

	// set the memberlist bind port to 0 to bind to a random port.
	// memberlist will update the value of BindPort after bind
	// to the actual value.
	config.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfLANConfig.MemberlistConfig.BindPort = ports[1]
	config.SerfLANConfig.MemberlistConfig.AdvertisePort = ports[1]
	config.SerfLANConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfLANConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfWANConfig.MemberlistConfig.BindPort = ports[2]
	config.SerfWANConfig.MemberlistConfig.AdvertisePort = ports[2]
	config.SerfWANConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfWANConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	config.RaftConfig.LeaderLeaseTimeout = 100 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 200 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 200 * time.Millisecond

	config.ReconcileInterval = 300 * time.Millisecond

	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

	config.Build = "0.8.0"

	config.CoordinateUpdatePeriod = 100 * time.Millisecond
	config.LeaveDrainTime = 1 * time.Millisecond

	// TODO (slackpad) - We should be able to run all tests w/o this, but it
	// looks like several depend on it.
	config.RPCHoldTimeout = 5 * time.Second

	config.ConnectEnabled = true
	config.CAConfig = &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
		Provider:  structs.ConsulCAProvider,
		Config: map[string]interface{}{
			"PrivateKey":     "",
			"RootCert":       "",
			"RotationPeriod": 90 * 24 * time.Hour,
		},
	}

	return dir, config
}

func testServer(t *testing.T) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})
}

func testServerDC(t *testing.T, dc string) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = true
	})
}

func testServerDCBootstrap(t *testing.T, dc string, bootstrap bool) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = bootstrap
	})
}

func testServerDCExpect(t *testing.T, dc string, expect int) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = expect
	})
}

func testServerWithConfig(t *testing.T, cb func(*Config)) (string, *Server) {
	dir, config := testServerConfig(t)
	if cb != nil {
		cb(config)
	}
	srv, err := newServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, srv
}

func newServer(c *Config) (*Server, error) {
	// chain server up notification
	oldNotify := c.NotifyListen
	up := make(chan struct{})
	c.NotifyListen = func() {
		close(up)
		if oldNotify != nil {
			oldNotify()
		}
	}

	// start server
	w := c.LogOutput
	if w == nil {
		w = os.Stderr
	}
	logger := log.New(w, c.NodeName+" - ", log.LstdFlags|log.Lmicroseconds)
	srv, err := NewServerLogger(c, logger, new(token.Store))
	if err != nil {
		return nil, err
	}

	// wait until after listen
	<-up

	// get the real address
	//
	// the server already sets the RPCAdvertise address
	// if it wasn't configured since it needs it for
	// some initialization
	//
	// todo(fs): setting RPCAddr should probably be guarded
	// todo(fs): but for now it is a shortcut to avoid fixing
	// todo(fs): tests which depend on that value. They should
	// todo(fs): just get the listener address instead.
	c.RPCAddr = srv.Listener.Addr().(*net.TCPAddr)
	return srv, nil
}

func TestServer_StartStop(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	// Set up two servers in a WAN.
	dir1, s1 := testServerDCBootstrap(t, "dc1", true)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc2", true)
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

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Do just a LAN join for the new server and make sure it
	// shows up in the WAN.
	joinLAN(t, s3, s1)

	for _, s := range []*Server{s1, s2, s3} {
		retry.Run(t, func(r *retry.R) {
			if got, want := len(s.WANMembers()), 3; got != want {
				r.Fatalf("got %d WAN members for %s want %d", got, s.config.NodeName, want)
			}
		})
	}
}

func TestServer_JoinSeparateLanAndWanAddresses(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = t.Name() + "-s1"
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.SerfFloodInterval = 100 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	s2Name := t.Name() + "-s2"
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = s2Name
		c.Datacenter = "dc2"
		c.Bootstrap = false
		// This wan address will be expected to be seen on s1
		c.SerfWANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.2"
		// This lan address will be expected to be seen on s3
		c.SerfLANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.3"
		c.SerfFloodInterval = 100 * time.Millisecond
	})

	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.NodeName = t.Name() + "-s3"
		c.Datacenter = "dc2"
		c.Bootstrap = true
		c.SerfFloodInterval = 100 * time.Millisecond
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Join s2 to s1 on wan
	joinWAN(t, s2, s1)

	// Join s3 to s2 on lan
	joinLAN(t, s3, s2)

	// We rely on flood joining to fill across the LAN, so we expect s3 to
	// show up on the WAN as well, even though it's not explicitly joined.
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 3; got != want {
			r.Fatalf("got %d s1 WAN members want %d", got, want)
		}
		if got, want := len(s2.WANMembers()), 3; got != want {
			r.Fatalf("got %d s2 WAN members want %d", got, want)
		}
		if got, want := len(s2.LANMembers()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
		if got, want := len(s3.LANMembers()), 2; got != want {
			r.Fatalf("got %d s3 LAN members want %d", got, want)
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
		if len(s2.serverLookup.Servers()) != 2 {
			r.Fatalf("local consul fellow s3 for s2 missing")
		}
	})

	// Get and check the wan address of s2 from s1
	var s2WanAddr string
	for _, member := range s1.WANMembers() {
		if member.Name == s2Name+".dc2" {
			s2WanAddr = member.Addr.String()
		}
	}
	if s2WanAddr != "127.0.0.2" {
		t.Fatalf("s1 sees s2 on a wrong address: %s, expecting: %s", s2WanAddr, "127.0.0.2")
	}

	// Get and check the lan address of s2 from s3
	var s2LanAddr string
	for _, lanmember := range s3.LANMembers() {
		if lanmember.Name == s2Name {
			s2LanAddr = lanmember.Addr.String()
		}
	}
	if s2LanAddr != "127.0.0.3" {
		t.Fatalf("s3 sees s2 on a wrong address: %s, expecting: %s", s2LanAddr, "127.0.0.3")
	}
}

func TestServer_LeaveLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 3))
		r.Check(wantPeers(s2, 3))
		r.Check(wantPeers(s3, 3))
	})
	// Issue a leave to the leader
	var leader *Server
	switch {
	case s1.IsLeader():
		leader = s1
	case s2.IsLeader():
		leader = s2
	case s3.IsLeader():
		leader = s3
	default:
		t.Fatal("no leader")
	}
	if err := leader.Leave(); err != nil {
		t.Fatal("leave failed: ", err)
	}

	// Should lose a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 2))
		r.Check(wantPeers(s2, 2))
		r.Check(wantPeers(s3, 2))
	})
}

func TestServer_Leave(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// Second server not in bootstrap mode
	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Issue a leave to the non-leader
	var nonleader *Server
	switch {
	case s1.IsLeader():
		nonleader = s2
	case s2.IsLeader():
		nonleader = s1
	default:
		t.Fatal("no leader")
	}
	if err := nonleader.Leave(); err != nil {
		t.Fatal("leave failed: ", err)
	}

	// Should lose a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantPeers(s1, 1))
		r.Check(wantPeers(s2, 1))
	})
}

func TestServer_RPC(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	var out struct{}
	if err := s1.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_JoinLAN_TLS(t *testing.T) {
	t.Parallel()
	dir1, conf1 := testServerConfig(t)
	conf1.VerifyIncoming = true
	conf1.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := newServer(conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, conf2 := testServerConfig(t)
	conf2.Bootstrap = false
	conf2.VerifyIncoming = true
	conf2.VerifyOutgoing = true
	configureTLS(conf2)
	s2, err := newServer(conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinLAN(t, s2, s1)

	// Verify Raft has established a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})
}

func TestServer_Expect(t *testing.T) {
	t.Parallel()
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
		r.Check(wantRaft([]*Server{s1, s2, s3, s4}))
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
	t.Parallel()
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
	t.Parallel()
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
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestServer_Encrypted(t *testing.T) {
	t.Parallel()
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
	joinLAN(t, s1, s2)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})

	// Have s2 make an RPC call to s1
	var leader *metadata.Server
	for _, server := range s2.serverLookup.Servers() {
		if server.Name == s1.config.NodeName {
			leader = server
		}
	}
	if leader == nil {
		t.Fatal("no leader")
	}
	return s2.connPool.Ping(leader.Datacenter, leader.Addr, leader.Version, leader.UseTLS)
}

func TestServer_TLSToNoTLS(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestServer_RevokeLeadershipIdempotent(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	err := s1.revokeLeadership()
	if err != nil {
		t.Fatal(err)
	}
	err = s1.revokeLeadership()
	if err != nil {
		t.Fatal(err)
	}
}
