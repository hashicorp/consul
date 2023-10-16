// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/google/tcpproxy"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/multilimiter"
	rpcRate "github.com/hashicorp/consul/agent/consul/rate"
	external "github.com/hashicorp/consul/agent/grpc-external"
	grpcmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

const (
	TestDefaultInitialManagementToken = "d9f05e83-a7ae-47ce-839e-c0d53a68c00a"
)

// testTLSCertificates Generates a TLS CA and server key/cert and returns them
// in PEM encoded form.
func testTLSCertificates(serverName string) (cert string, key string, cacert string, err error) {
	signer, _, err := tlsutil.GeneratePrivateKey()
	if err != nil {
		return "", "", "", err
	}

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	if err != nil {
		return "", "", "", err
	}

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return "", "", "", err
	}

	return cert, privateKey, ca, nil
}

func testServerACLConfig(c *Config) {
	c.PrimaryDatacenter = "dc1"
	c.ACLsEnabled = true
	c.ACLInitialManagementToken = TestDefaultInitialManagementToken
	c.ACLResolverSettings.ACLDefaultPolicy = "deny"
}

func configureTLS(config *Config) {
	config.TLSConfig.InternalRPC.CAFile = "../../test/ca/root.cer"
	config.TLSConfig.InternalRPC.CertFile = "../../test/key/ourdomain.cer"
	config.TLSConfig.InternalRPC.KeyFile = "../../test/key/ourdomain.key"
}

var id int64

func uniqueNodeName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	return fmt.Sprintf("%s-node-%d", name, atomic.AddInt64(&id, 1))
}

// This will find the leader of a list of servers and verify that leader establishment has completed
func waitForLeaderEstablishment(t *testing.T, servers ...*Server) {
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		hasLeader := false
		for _, srv := range servers {
			if srv.IsLeader() {
				hasLeader = true
				require.True(r, srv.isReadyForConsistentReads(), "Leader %s hasn't finished establishing leadership yet", srv.config.NodeName)
			}
		}
		require.True(r, hasLeader, "Cluster has not elected a leader yet")
	})
}

func testServerConfig(t *testing.T) (string, *Config) {
	dir := testutil.TempDir(t, "consul")
	config := DefaultConfig()

	ports := freeport.GetN(t, 4) // {server, serf_lan, serf_wan, grpc}
	config.NodeName = uniqueNodeName(t.Name())
	config.Bootstrap = true
	config.Datacenter = "dc1"
	config.PrimaryDatacenter = "dc1"
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
	config.SerfLANConfig.MemberlistConfig.DeadNodeReclaimTime = 100 * time.Millisecond

	config.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfWANConfig.MemberlistConfig.BindPort = ports[2]
	config.SerfWANConfig.MemberlistConfig.AdvertisePort = ports[2]
	config.SerfWANConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfWANConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond
	config.SerfWANConfig.MemberlistConfig.DeadNodeReclaimTime = 100 * time.Millisecond

	config.RaftConfig.LeaderLeaseTimeout = 100 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 200 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 200 * time.Millisecond

	config.ReconcileInterval = 300 * time.Millisecond

	config.AutopilotConfig.ServerStabilizationTime = 100 * time.Millisecond
	config.ServerHealthInterval = 50 * time.Millisecond
	config.AutopilotInterval = 100 * time.Millisecond

	config.CoordinateUpdatePeriod = 100 * time.Millisecond
	config.LeaveDrainTime = 1 * time.Millisecond

	// TODO (slackpad) - We should be able to run all tests w/o this, but it
	// looks like several depend on it.
	config.RPCHoldTimeout = 10 * time.Second

	config.GRPCPort = ports[3]

	config.ConnectEnabled = true
	config.CAConfig = &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
		Provider:  structs.ConsulCAProvider,
		Config: map[string]interface{}{
			"PrivateKey":          "",
			"RootCert":            "",
			"LeafCertTTL":         "72h",
			"IntermediateCertTTL": "288h",
		},
	}
	config.PeeringEnabled = true
	return dir, config
}

// Deprecated: use testServerWithConfig instead. It does the same thing and more.
func testServer(t *testing.T) (string, *Server) {
	return testServerWithConfig(t)
}

// Deprecated: use testServerWithConfig
func testServerDC(t *testing.T, dc string) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = true
	})
}

// Deprecated: use testServerWithConfig
func testServerDCBootstrap(t *testing.T, dc string, bootstrap bool) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.PrimaryDatacenter = dc
		c.Bootstrap = bootstrap
	})
}

// Deprecated: use testServerWithConfig
func testServerDCExpect(t *testing.T, dc string, expect int) (string, *Server) {
	return testServerWithConfig(t, func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = expect
	})
}

func testServerWithConfig(t *testing.T, configOpts ...func(*Config)) (string, *Server) {
	var dir string
	var srv *Server

	var config *Config
	var deps Deps
	// Retry added to avoid cases where bind addr is already in use
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		dir, config = testServerConfig(t)
		for _, fn := range configOpts {
			fn(config)
		}

		// Apply config to copied fields because many tests only set the old
		// values.
		config.ACLResolverSettings.ACLsEnabled = config.ACLsEnabled
		config.ACLResolverSettings.NodeName = config.NodeName
		config.ACLResolverSettings.Datacenter = config.Datacenter
		config.ACLResolverSettings.EnterpriseMeta = *config.AgentEnterpriseMeta()

		var err error
		deps = newDefaultDeps(t, config)
		srv, err = newServerWithDeps(t, config, deps)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
	})
	t.Cleanup(func() { srv.Shutdown() })

	for _, grpcPort := range []int{srv.config.GRPCPort, srv.config.GRPCTLSPort} {
		if grpcPort == 0 {
			continue
		}

		// Normally the gRPC server listener is created at the agent level and
		// passed down into the Server creation.
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", grpcPort))
		require.NoError(t, err)

		protocol := grpcmiddleware.ProtocolPlaintext
		if grpcPort == srv.config.GRPCTLSPort || deps.TLSConfigurator.GRPCServerUseTLS() {
			protocol = grpcmiddleware.ProtocolTLS
			// Set the internally managed server certificate. The cert manager is hooked to the Agent, so we need to bypass that here.
			if srv.config.PeeringEnabled && srv.config.ConnectEnabled {
				key, _ := srv.config.CAConfig.Config["PrivateKey"].(string)
				cert, _ := srv.config.CAConfig.Config["RootCert"].(string)
				if key != "" && cert != "" {
					ca := &structs.CARoot{
						SigningKey: key,
						RootCert:   cert,
					}
					require.NoError(t, deps.TLSConfigurator.UpdateAutoTLSCert(connect.TestServerLeaf(t, srv.config.Datacenter, ca)))
					deps.TLSConfigurator.UpdateAutoTLSPeeringServerName(connect.PeeringServerSAN("dc1", connect.TestTrustDomain))
				}
			}

		}
		ln = grpcmiddleware.LabelledListener{Listener: ln, Protocol: protocol}

		go func() {
			_ = srv.externalGRPCServer.Serve(ln)
		}()
		t.Cleanup(srv.externalGRPCServer.Stop)
	}

	return dir, srv
}

// cb is a function that can alter the test servers configuration prior to the server starting.
func testACLServerWithConfig(t *testing.T, cb func(*Config), initReplicationToken bool) (string, *Server, rpc.ClientCodec) {
	opts := []func(*Config){testServerACLConfig}
	if cb != nil {
		opts = append(opts, cb)
	}
	dir, srv := testServerWithConfig(t, opts...)

	if initReplicationToken {
		// setup some tokens here so we get less warnings in the logs
		srv.tokens.UpdateReplicationToken(TestDefaultInitialManagementToken, token.TokenSourceConfig)
	}

	codec := rpcClient(t, srv)
	return dir, srv, codec
}

func testGRPCIntegrationServer(t *testing.T, cb func(*Config)) (*Server, *grpc.ClientConn, rpc.ClientCodec) {
	_, srv, codec := testACLServerWithConfig(t, cb, false)

	grpcAddr := fmt.Sprintf("127.0.0.1:%d", srv.config.GRPCPort)
	//nolint:staticcheck
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return srv, conn, codec
}

func newServer(t *testing.T, c *Config) (*Server, error) {
	return newServerWithDeps(t, c, newDefaultDeps(t, c))
}

func newServerWithDeps(t *testing.T, c *Config, deps Deps) (*Server, error) {
	// chain server up notification
	oldNotify := c.NotifyListen
	up := make(chan struct{})
	c.NotifyListen = func() {
		close(up)
		if oldNotify != nil {
			oldNotify()
		}
	}
	grpcServer := external.NewServer(deps.Logger.Named("grpc.external"), nil, deps.TLSConfigurator, rpcRate.NullRequestLimitsHandler())
	srv, err := NewServer(c, deps, grpcServer, nil, deps.Logger, nil)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { srv.Shutdown() })

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
	_, s1 := testServer(t)
	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Shut down again, which should be idempotent.
	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestServer_fixupACLDatacenter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "aye"
		c.PrimaryDatacenter = "aye"
		c.ACLsEnabled = true
	})
	defer s1.Shutdown()

	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "bee"
		c.PrimaryDatacenter = "aye"
		c.ACLsEnabled = true
	})
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

	testrpc.WaitForLeader(t, s1.RPC, "aye")
	testrpc.WaitForLeader(t, s2.RPC, "bee")

	require.Equal(t, "aye", s1.config.Datacenter)
	require.Equal(t, "aye", s1.config.PrimaryDatacenter)
	require.Equal(t, "aye", s1.config.PrimaryDatacenter)

	require.Equal(t, "bee", s2.config.Datacenter)
	require.Equal(t, "aye", s2.config.PrimaryDatacenter)
	require.Equal(t, "aye", s2.config.PrimaryDatacenter)
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
		if got, want := len(s1.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d s1 LAN members want %d", got, want)
		}
		if got, want := len(s2.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
	})
}

// TestServer_JoinLAN_SerfAllowedCIDRs test that IPs might be blocked with
// Serf.
//
// To run properly, this test requires to be able to bind and have access on
// 127.0.1.1 which is the case for most Linux machines and Windows, so Unit
// test will run in the CI.
//
// To run it on Mac OS, please run this command first, otherwise the test will
// be skipped: `sudo ifconfig lo0 alias 127.0.1.1 up`
func TestServer_JoinLAN_SerfAllowedCIDRs(t *testing.T) {
	t.Parallel()

	const targetAddr = "127.0.1.1"

	skipIfCannotBindToIP(t, targetAddr)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.BootstrapExpect = 1
		lan, err := memberlist.ParseCIDRs([]string{"127.0.0.1/32"})
		require.NoError(t, err)
		c.SerfLANConfig.MemberlistConfig.CIDRsAllowed = lan
		wan, err := memberlist.ParseCIDRs([]string{"127.0.0.0/24", "::1/128"})
		require.NoError(t, err)
		c.SerfWANConfig.MemberlistConfig.CIDRsAllowed = wan
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, a2 := testClientWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.BindAddr = targetAddr
	})
	defer os.RemoveAll(dir2)
	defer a2.Shutdown()

	dir3, rs3 := testServerWithConfig(t, func(c *Config) {
		c.BootstrapExpect = 1
		c.Datacenter = "dc2"
	})
	defer os.RemoveAll(dir3)
	defer rs3.Shutdown()

	leaderAddr := joinAddrLAN(s1)
	if _, err := a2.JoinLAN([]string{leaderAddr}, nil); err != nil {
		t.Fatalf("Expected no error, had: %#v", err)
	}
	// Try to join
	joinWAN(t, rs3, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.LANMembersInAgentPartition()), 1; got != want {
			// LAN is blocked, should be 1 only
			r.Fatalf("got %d s1 LAN members want %d", got, want)
		}
		if got, want := len(a2.LANMembersInAgentPartition()), 2; got != want {
			// LAN is blocked a2 can see s1, but not s1
			r.Fatalf("got %d a2 LAN members want %d", got, want)
		}
		if got, want := len(s1.WANMembers()), 2; got != want {
			r.Fatalf("got %d s1 WAN members want %d", got, want)
		}
		if got, want := len(rs3.WANMembers()), 2; got != want {
			r.Fatalf("got %d rs3 WAN members want %d", got, want)
		}
	})
}

// TestServer_JoinWAN_SerfAllowedCIDRs test that IPs might be
// blocked with Serf.
//
// To run properly, this test requires to be able to bind and have access on
// 127.0.1.1 which is the case for most Linux machines and Windows, so Unit
// test will run in the CI.
//
// To run it on Mac OS, please run this command first, otherwise the test will
// be skipped: `sudo ifconfig lo0 alias 127.0.1.1 up`
func TestServer_JoinWAN_SerfAllowedCIDRs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	const targetAddr = "127.0.1.1"

	skipIfCannotBindToIP(t, targetAddr)

	wanCIDRs, err := memberlist.ParseCIDRs([]string{"127.0.0.1/32"})
	require.NoError(t, err)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
		c.Datacenter = "dc1"
		c.SerfWANConfig.MemberlistConfig.CIDRsAllowed = wanCIDRs
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	waitForLeaderEstablishment(t, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = true
		c.BootstrapExpect = 1
		c.PrimaryDatacenter = "dc1"
		c.Datacenter = "dc2"
		c.SerfWANConfig.MemberlistConfig.BindAddr = targetAddr
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	waitForLeaderEstablishment(t, s2)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Joining should be fine
	joinWANWithNoMembershipChecks(t, s2, s1)

	// But membership is blocked if you go and take a peek on the server.
	t.Run("LAN membership should only show each other", func(t *testing.T) {
		require.Len(t, s1.LANMembersInAgentPartition(), 1)
		require.Len(t, s2.LANMembersInAgentPartition(), 1)
	})
	t.Run("WAN membership in the primary should not show the secondary", func(t *testing.T) {
		require.Len(t, s1.WANMembers(), 1)
	})
	t.Run("WAN membership in the secondary can show the primary", func(t *testing.T) {
		require.Len(t, s2.WANMembers(), 2)
	})
}

func skipIfCannotBindToIP(t *testing.T, ip string) {
	l, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Skipf("Cannot bind on %s, to run on Mac OS: `sudo ifconfig lo0 alias %s up`", ip, ip)
	}
	l.Close()
}

func TestServer_LANReap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	configureServer := func(c *Config) {
		c.SerfFloodInterval = 100 * time.Millisecond
		c.SerfLANConfig.ReconnectTimeout = 250 * time.Millisecond
		c.SerfLANConfig.TombstoneTimeout = 250 * time.Millisecond
		c.SerfLANConfig.ReapInterval = 300 * time.Millisecond
	}

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		configureServer(c)
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		configureServer(c)
	})
	defer os.RemoveAll(dir2)

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		configureServer(c)
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")
	testrpc.WaitForLeader(t, s3.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.LANMembersInAgentPartition(), 3)
		require.Len(r, s2.LANMembersInAgentPartition(), 3)
		require.Len(r, s3.LANMembersInAgentPartition(), 3)
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.serverLookup.Servers(), 3)
		require.Len(r, s2.serverLookup.Servers(), 3)
		require.Len(r, s3.serverLookup.Servers(), 3)
	})

	// shutdown the second dc
	s2.Shutdown()

	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.LANMembersInAgentPartition(), 2)
		servers := s1.serverLookup.Servers()
		require.Len(r, servers, 2)
		// require.Equal(r, s1.config.NodeName, servers[0].Name)
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

func TestServer_WANReap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.SerfFloodInterval = 100 * time.Millisecond
		c.SerfWANConfig.ReconnectTimeout = 250 * time.Millisecond
		c.SerfWANConfig.TombstoneTimeout = 250 * time.Millisecond
		c.SerfWANConfig.ReapInterval = 500 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)

	// Try to join
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.WANMembers(), 2)
		require.Len(r, s2.WANMembers(), 2)
	})

	// Check the router has both
	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.router.GetDatacenters(), 2)
		require.Len(r, s2.router.GetDatacenters(), 2)
	})

	// shutdown the second dc
	s2.Shutdown()

	retry.Run(t, func(r *retry.R) {
		require.Len(r, s1.WANMembers(), 1)
		datacenters := s1.router.GetDatacenters()
		require.Len(r, datacenters, 1)
		require.Equal(r, "dc1", datacenters[0])
	})

}

func TestServer_JoinWAN_Flood(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

// This is a mirror of a similar test in agent/agent_test.go
func TestServer_JoinWAN_viaMeshGateway(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	port := freeport.GetOne(t)
	gwAddr := ipaddr.FormatAddressPort("127.0.0.1", port)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.TLSConfig.Domain = "consul"
		c.NodeName = "bob"
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.Bootstrap = true
		// tls
		c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Bob.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Bob.key"
		c.TLSConfig.InternalRPC.VerifyIncoming = true
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		// wanfed
		c.ConnectMeshGatewayWANFederationEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.TLSConfig.Domain = "consul"
		c.NodeName = "betty"
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Bootstrap = true
		// tls
		c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Betty.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Betty.key"
		c.TLSConfig.InternalRPC.VerifyIncoming = true
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		// wanfed
		c.ConnectMeshGatewayWANFederationEnabled = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.TLSConfig.Domain = "consul"
		c.NodeName = "bonnie"
		c.Datacenter = "dc3"
		c.PrimaryDatacenter = "dc1"
		c.Bootstrap = true
		// tls
		c.TLSConfig.InternalRPC.CAFile = "../../test/hostname/CertAuth.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/hostname/Bonnie.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/hostname/Bonnie.key"
		c.TLSConfig.InternalRPC.VerifyIncoming = true
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		// wanfed
		c.ConnectMeshGatewayWANFederationEnabled = true
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// We'll use the same gateway for all datacenters since it doesn't care.
	var p tcpproxy.Proxy
	p.AddSNIRoute(gwAddr, "bob.server.dc1.consul", tcpproxy.To(s1.config.RPCAddr.String()))
	p.AddSNIRoute(gwAddr, "betty.server.dc2.consul", tcpproxy.To(s2.config.RPCAddr.String()))
	p.AddSNIRoute(gwAddr, "bonnie.server.dc3.consul", tcpproxy.To(s3.config.RPCAddr.String()))
	p.AddStopACMESearch(gwAddr)
	require.NoError(t, p.Start())
	defer func() {
		p.Close()
		p.Wait()
	}()

	t.Logf("routing %s => %s", "bob.server.dc1.consul", s1.config.RPCAddr.String())
	t.Logf("routing %s => %s", "betty.server.dc2.consul", s2.config.RPCAddr.String())
	t.Logf("routing %s => %s", "bonnie.server.dc3.consul", s3.config.RPCAddr.String())

	// Register this into the catalog in dc1.
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bob",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				Port:    port,
			},
		}

		var out struct{}
		require.NoError(t, s1.RPC(context.Background(), "Catalog.Register", &arg, &out))
	}

	// Wait for it to make it into the gateway locator.
	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, s1.gatewayLocator.PickGateway("dc1"))
	})

	// Seed the secondaries with the address of the primary and wait for that to
	// be in their locators.
	s2.RefreshPrimaryGatewayFallbackAddresses([]string{gwAddr})
	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, s2.gatewayLocator.PickGateway("dc1"))
	})
	s3.RefreshPrimaryGatewayFallbackAddresses([]string{gwAddr})
	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, s3.gatewayLocator.PickGateway("dc1"))
	})

	// Try to join from secondary to primary. We can't use joinWAN() because we
	// are simulating proper bootstrapping and if ACLs were on we would have to
	// delay gateway registration in the secondary until after one directional
	// join. So this way we explicitly join secondary-to-primary as a standalone
	// operation and follow it up later with a full join.
	_, err := s2.JoinWAN([]string{joinAddrWAN(s1)})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s2.WANMembers()), 2; got != want {
			r.Fatalf("got %d s2 WAN members want %d", got, want)
		}
	})
	_, err = s3.JoinWAN([]string{joinAddrWAN(s1)})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s3.WANMembers()), 3; got != want {
			r.Fatalf("got %d s3 WAN members want %d", got, want)
		}
	})

	// Now we can register this into the catalog in dc2 and dc3.
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc2",
			Node:       "betty",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				Port:    port,
			},
		}

		var out struct{}
		require.NoError(t, s2.RPC(context.Background(), "Catalog.Register", &arg, &out))
	}
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc3",
			Node:       "bonnie",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Meta:    map[string]string{structs.MetaWANFederationKey: "1"},
				Port:    port,
			},
		}

		var out struct{}
		require.NoError(t, s3.RPC(context.Background(), "Catalog.Register", &arg, &out))
	}

	// Wait for it to make it into the gateway locator in dc2 and then for
	// AE to carry it back to the primary
	retry.Run(t, func(r *retry.R) {
		require.NotEmpty(r, s3.gatewayLocator.PickGateway("dc2"))
		require.NotEmpty(r, s2.gatewayLocator.PickGateway("dc2"))
		require.NotEmpty(r, s1.gatewayLocator.PickGateway("dc2"))

		require.NotEmpty(r, s3.gatewayLocator.PickGateway("dc3"))
		require.NotEmpty(r, s2.gatewayLocator.PickGateway("dc3"))
		require.NotEmpty(r, s1.gatewayLocator.PickGateway("dc3"))
	})

	// Try to join again using the standard verification method now that
	// all of the plumbing is in place.
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.WANMembers()), 3; got != want {
			r.Fatalf("got %d s1 WAN members want %d", got, want)
		}
		if got, want := len(s2.WANMembers()), 3; got != want {
			r.Fatalf("got %d s2 WAN members want %d", got, want)
		}
	})

	// Check the router has all of them
	retry.Run(t, func(r *retry.R) {
		if got, want := len(s1.router.GetDatacenters()), 3; got != want {
			r.Fatalf("got %d routes want %d", got, want)
		}
		if got, want := len(s2.router.GetDatacenters()), 3; got != want {
			r.Fatalf("got %d datacenters want %d", got, want)
		}
		if got, want := len(s3.router.GetDatacenters()), 3; got != want {
			r.Fatalf("got %d datacenters want %d", got, want)
		}
	})

	// Ensure we can do some trivial RPC in all directions.
	servers := map[string]*Server{"dc1": s1, "dc2": s2, "dc3": s3}
	names := map[string]string{"dc1": "bob", "dc2": "betty", "dc3": "bonnie"}
	for _, srcDC := range []string{"dc1", "dc2", "dc3"} {
		srv := servers[srcDC]
		for _, dstDC := range []string{"dc1", "dc2", "dc3"} {
			if srcDC == dstDC {
				continue
			}
			t.Run(srcDC+" to "+dstDC, func(t *testing.T) {
				arg := structs.DCSpecificRequest{
					Datacenter: dstDC,
				}
				var out structs.IndexedNodes
				require.NoError(t, srv.RPC(context.Background(), "Catalog.ListNodes", &arg, &out))
				require.Len(t, out.Nodes, 1)
				node := out.Nodes[0]
				require.Equal(t, dstDC, node.Datacenter)
				require.Equal(t, names[dstDC], node.Node)
			})
		}
	}
}

func TestServer_JoinSeparateLanAndWanAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		if got, want := len(s2.LANMembersInAgentPartition()), 2; got != want {
			r.Fatalf("got %d s2 LAN members want %d", got, want)
		}
		if got, want := len(s3.LANMembersInAgentPartition()), 2; got != want {
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
	for _, lanmember := range s3.LANMembersInAgentPartition() {
		if lanmember.Name == s2Name {
			s2LanAddr = lanmember.Addr.String()
		}
	}
	if s2LanAddr != "127.0.0.3" {
		t.Fatalf("s3 sees s2 on a wrong address: %s, expecting: %s", s2LanAddr, "127.0.0.3")
	}
}

func TestServer_LeaveLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if err := s1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

// TestServer_RPC_MetricsIntercept_Off proves that we can turn off net/rpc interceptors all together.
func TestServer_RPC_MetricsIntercept_Off(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	storage := &sync.Map{} // string -> float32
	keyMakingFunc := func(key []string, labels []metrics.Label) string {
		allKey := strings.Join(key, "+")

		for _, label := range labels {
			if label.Name == "method" {
				allKey = allKey + "+" + label.Value
			}
		}

		return allKey
	}

	simpleRecorderFunc := func(key []string, val float32, labels []metrics.Label) {
		storage.Store(keyMakingFunc(key, labels), val)
	}

	t.Run("test no net/rpc interceptor metric with nil func", func(t *testing.T) {
		_, conf := testServerConfig(t)
		deps := newDefaultDeps(t, conf)

		// "disable" metrics net/rpc interceptor
		deps.GetNetRPCInterceptorFunc = nil
		// "hijack" the rpc recorder for asserts;
		// note that there will be "internal" net/rpc calls made
		// that will still show up; those don't go thru the net/rpc interceptor;
		// see consul.agent.rpc.middleware.RPCTypeInternal for context
		deps.NewRequestRecorderFunc = func(logger hclog.Logger, isLeader func() bool, localDC string) *middleware.RequestRecorder {
			// for the purposes of this test, we don't need isLeader or localDC
			return &middleware.RequestRecorder{
				Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
				RecorderFunc: simpleRecorderFunc,
			}
		}

		s1, err := NewServer(conf, deps, grpc.NewServer(), nil, deps.Logger, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		t.Cleanup(func() { s1.Shutdown() })

		var out struct{}
		if err := s1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		key := keyMakingFunc(middleware.OneTwelveRPCSummary[0].Name, []metrics.Label{{Name: "method", Value: "Status.Ping"}})

		if _, ok := storage.Load(key); ok {
			t.Fatalf("Did not expect to find key %s in the metrics log, ", key)
		}
	})

	t.Run("test no net/rpc interceptor metric with func that gives nil", func(t *testing.T) {
		_, conf := testServerConfig(t)
		deps := newDefaultDeps(t, conf)

		// "hijack" the rpc recorder for asserts;
		// note that there will be "internal" net/rpc calls made
		// that will still show up; those don't go thru the net/rpc interceptor;
		// see consul.agent.rpc.middleware.RPCTypeInternal for context
		deps.NewRequestRecorderFunc = func(logger hclog.Logger, isLeader func() bool, localDC string) *middleware.RequestRecorder {
			// for the purposes of this test, we don't need isLeader or localDC
			return &middleware.RequestRecorder{
				Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
				RecorderFunc: simpleRecorderFunc,
			}
		}

		deps.GetNetRPCInterceptorFunc = func(recorder *middleware.RequestRecorder) rpc.ServerServiceCallInterceptor {
			return nil
		}

		s2, err := NewServer(conf, deps, grpc.NewServer(), nil, deps.Logger, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		t.Cleanup(func() { s2.Shutdown() })
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		var out struct{}
		if err := s2.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		key := keyMakingFunc(middleware.OneTwelveRPCSummary[0].Name, []metrics.Label{{Name: "method", Value: "Status.Ping"}})

		if _, ok := storage.Load(key); ok {
			t.Fatalf("Did not expect to find key %s in the metrics log, ", key)
		}
	})
}

// TestServer_RPC_RequestRecorder proves that we cannot make a server without a valid RequestRecorder provider func
// or a non nil RequestRecorder.
func TestServer_RPC_RequestRecorder(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("test nil func provider", func(t *testing.T) {
		_, conf := testServerConfig(t)
		deps := newDefaultDeps(t, conf)
		deps.NewRequestRecorderFunc = nil

		s1, err := NewServer(conf, deps, grpc.NewServer(), nil, deps.Logger, nil)

		require.Error(t, err, "need err when provider func is nil")
		require.Equal(t, err.Error(), "cannot initialize server without an RPC request recorder provider")

		t.Cleanup(func() {
			if s1 != nil {
				s1.Shutdown()
			}
		})
	})

	t.Run("test nil RequestRecorder", func(t *testing.T) {
		_, conf := testServerConfig(t)
		deps := newDefaultDeps(t, conf)
		deps.NewRequestRecorderFunc = func(logger hclog.Logger, isLeader func() bool, localDC string) *middleware.RequestRecorder {
			return nil
		}

		s2, err := NewServer(conf, deps, grpc.NewServer(), nil, deps.Logger, nil)

		require.Error(t, err, "need err when RequestRecorder is nil")
		require.Equal(t, err.Error(), "cannot initialize server with a nil RPC request recorder")

		t.Cleanup(func() {
			if s2 != nil {
				s2.Shutdown()
			}
		})
	})
}

// TestServer_RPC_MetricsIntercept mocks a request recorder and asserts that RPC calls are observed.
func TestServer_RPC_MetricsIntercept(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, conf := testServerConfig(t)
	deps := newDefaultDeps(t, conf)

	// The method used to record metric observations here is similar to that used in
	// interceptors_test.go.
	storage := &sync.Map{} // string -> float32
	keyMakingFunc := func(key []string, labels []metrics.Label) string {
		allKey := strings.Join(key, "+")

		for _, label := range labels {
			allKey = allKey + "+" + label.Value
		}

		return allKey
	}

	simpleRecorderFunc := func(key []string, val float32, labels []metrics.Label) {
		storage.Store(keyMakingFunc(key, labels), val)
	}
	deps.NewRequestRecorderFunc = func(logger hclog.Logger, isLeader func() bool, localDC string) *middleware.RequestRecorder {
		// for the purposes of this test, we don't need isLeader or localDC
		return &middleware.RequestRecorder{
			Logger:       hclog.NewInterceptLogger(&hclog.LoggerOptions{}),
			RecorderFunc: simpleRecorderFunc,
		}
	}

	deps.GetNetRPCInterceptorFunc = func(recorder *middleware.RequestRecorder) rpc.ServerServiceCallInterceptor {
		return func(reqServiceMethod string, argv, replyv reflect.Value, handler func() error) {
			reqStart := time.Now()

			err := handler()

			recorder.Record(reqServiceMethod, "test", reqStart, argv.Interface(), err != nil)
		}
	}

	s, err := newServerWithDeps(t, conf, deps)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()
	testrpc.WaitForTestAgent(t, s.RPC, "dc1")

	// asserts
	t.Run("test happy path for metrics interceptor", func(t *testing.T) {
		var out struct{}
		if err := s.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		expectedLabels := []metrics.Label{
			{Name: "method", Value: "Status.Ping"},
			{Name: "errored", Value: "false"},
			{Name: "request_type", Value: "read"},
			{Name: "rpc_type", Value: "test"},
			{Name: "server_role", Value: "unreported"},
		}

		key := keyMakingFunc(middleware.OneTwelveRPCSummary[0].Name, expectedLabels)

		if _, ok := storage.Load(key); !ok {
			// the compound key will look like: "rpc+server+call+Status.Ping+false+read+test+unreported"
			t.Fatalf("Did not find key %s in the metrics log, ", key)
		}
	})
}

func TestServer_JoinLAN_TLS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, conf1 := testServerConfig(t)
	conf1.TLSConfig.InternalRPC.VerifyIncoming = true
	conf1.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf1)
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	_, conf2 := testServerConfig(t)
	conf2.Bootstrap = false
	conf2.TLSConfig.InternalRPC.VerifyIncoming = true
	conf2.TLSConfig.InternalRPC.VerifyOutgoing = true
	configureTLS(conf2)
	s2, err := newServer(t, conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	// Try to join
	joinLAN(t, s2, s1)
	testrpc.WaitForTestAgent(t, s2.RPC, "dc1")

	// Verify Raft has established a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})
}

func TestServer_Expect(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	// Join the fourth node.
	joinLAN(t, s4, s1)

	// Wait for the new server to see itself added to the cluster.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2, s3, s4}))
	})
}

// Should not trigger bootstrap and new election when s3 joins, since cluster exists
func TestServer_AvoidReBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServerDCExpect(t, "dc1", 2)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCExpect(t, "dc1", 0)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCExpect(t, "dc1", 2)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Join the first two servers
	joinLAN(t, s2, s1)

	// Make sure a leader is elected, grab the current term and then add in
	// the third server.
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	termBefore := s1.raft.Stats()["last_log_term"]
	joinLAN(t, s3, s1)

	// Wait for the new server to see itself added to the cluster.
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2, s3}))
	})

	// Make sure there's still a leader and that the term didn't change,
	// so we know an election didn't occur.
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	termAfter := s1.raft.Stats()["last_log_term"]
	if termAfter != termBefore {
		t.Fatalf("looks like an election took place")
	}
}

func TestServer_Expect_NonVoters(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 2
		c.ReadReplica = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCExpect(t, "dc1", 2)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCExpect(t, "dc1", 2)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

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
		r.Check(wantPeers(s1, 2))
		r.Check(wantPeers(s2, 2))
		r.Check(wantPeers(s3, 2))
	})

	// Make sure a leader is elected
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2, s3}))
	})
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
}

func (r *fakeGlobalResp) New() interface{} {
	return struct{}{}
}

func TestServer_keyringRPCs(t *testing.T) {
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
	_, err := s1.keyringRPCs("Bad.Method", nil, []string{s1.config.Datacenter})
	if err == nil {
		t.Fatalf("should have errored")
	}
	if !strings.Contains(err.Error(), "Bad.Method") {
		t.Fatalf("unexpected error: %s", err)
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
	return s2.connPool.Ping(leader.Datacenter, leader.ShortName, leader.Addr)
}

func TestServer_TLSToNoTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Set up a server with no TLS configured
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS configured
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.TLSConfig.InternalRPC.CAFile = "../../test/client_certs/rootca.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/client_certs/server.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/client_certs/server.key"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Set up a server with no TLS configured
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS and VerifyOutgoing set
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.TLSConfig.InternalRPC.CAFile = "../../test/client_certs/rootca.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/client_certs/server.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/client_certs/server.key"
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	_, err := testVerifyRPC(s1, s2, t)
	if err == nil || !strings.Contains(err.Error(), "remote error: tls") {
		t.Fatalf("should fail")
	}
}

func TestServer_TLSToFullVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	// Set up a server with TLS and VerifyIncoming set
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.TLSConfig.InternalRPC.CAFile = "../../test/client_certs/rootca.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/client_certs/server.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/client_certs/server.key"
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a second server with TLS configured
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
		c.TLSConfig.InternalRPC.CAFile = "../../test/client_certs/rootca.crt"
		c.TLSConfig.InternalRPC.CertFile = "../../test/client_certs/server.crt"
		c.TLSConfig.InternalRPC.KeyFile = "../../test/client_certs/server.key"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.revokeLeadership()
	s1.revokeLeadership()
}

func TestServer_ReloadConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	entryInit := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			// these are made a []uint8 and a int64 to allow the Equals test to pass
			// otherwise it will fail complaining about data types
			"foo": "bar",
			"bar": int64(1),
		},
	}

	dir1, s := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.5.0"
		c.RPCRateLimit = 500
		c.RPCMaxBurst = 5000
		c.RequestLimitsMode = "permissive"
		c.RequestLimitsReadRate = 500
		c.RequestLimitsWriteRate = 500
		c.RPCClientTimeout = 60 * time.Second
		// Set one raft param to be non-default in the initial config, others are
		// default.
		c.RaftConfig.TrailingLogs = 1234
	})
	defer os.RemoveAll(dir1)
	defer s.Shutdown()

	testrpc.WaitForTestAgent(t, s.RPC, "dc1")

	limiter := s.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(500), limiter.Limit())
	require.Equal(t, 5000, limiter.Burst())

	require.Equal(t, 60*time.Second, s.connPool.RPCClientTimeout())

	rc := ReloadableConfig{
		RequestLimits: &RequestLimits{
			Mode:      rpcRate.ModeEnforcing,
			ReadRate:  1000,
			WriteRate: 1100,
		},
		RPCClientTimeout:     2 * time.Minute,
		RPCRateLimit:         1000,
		RPCMaxBurst:          10000,
		ConfigEntryBootstrap: []structs.ConfigEntry{entryInit},
		// Reset the custom one to default be removing it from config file (it will
		// be a zero value here).
		RaftTrailingLogs: 0,

		// Set a different Raft param to something custom now
		RaftSnapshotThreshold: 4321,

		// Leave other raft fields default
	}

	mockHandler := rpcRate.NewMockRequestLimitsHandler(t)
	mockHandler.On("UpdateConfig", mock.Anything).Return(func(cfg rpcRate.HandlerConfig) {})

	s.incomingRPCLimiter = mockHandler
	require.NoError(t, s.ReloadConfig(rc))

	_, entry, err := s.fsm.State().ConfigEntry(nil, structs.ProxyDefaults, structs.ProxyConfigGlobal, structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.NotNil(t, entry)
	global, ok := entry.(*structs.ProxyConfigEntry)
	require.True(t, ok)
	require.Equal(t, entryInit.Kind, global.Kind)
	require.Equal(t, entryInit.Name, global.Name)
	require.Equal(t, entryInit.Config, global.Config)

	// Check rate limiter got updated
	limiter = s.rpcLimiter.Load().(*rate.Limiter)
	require.Equal(t, rate.Limit(1000), limiter.Limit())
	require.Equal(t, 10000, limiter.Burst())

	// Check the incoming RPC rate limiter got updated
	mockHandler.AssertCalled(t, "UpdateConfig", rpcRate.HandlerConfig{
		GlobalLimitConfig: rpcRate.GlobalLimitConfig{
			Mode: rc.RequestLimits.Mode,
			ReadWriteConfig: rpcRate.ReadWriteConfig{
				ReadConfig: multilimiter.LimiterConfig{
					Rate:  rc.RequestLimits.ReadRate,
					Burst: int(rc.RequestLimits.ReadRate) * requestLimitsBurstMultiplier,
				},
				WriteConfig: multilimiter.LimiterConfig{
					Rate:  rc.RequestLimits.WriteRate,
					Burst: int(rc.RequestLimits.WriteRate) * requestLimitsBurstMultiplier,
				},
			},
		},
	})

	// Check RPC client timeout got updated
	require.Equal(t, 2*time.Minute, s.connPool.RPCClientTimeout())

	// Check raft config
	defaults := DefaultConfig()
	got := s.raft.ReloadableConfig()
	require.Equal(t, uint64(4321), got.SnapshotThreshold,
		"should have been reloaded to new value")
	require.Equal(t, defaults.RaftConfig.SnapshotInterval, got.SnapshotInterval,
		"should have remained the default interval")
	require.Equal(t, defaults.RaftConfig.TrailingLogs, got.TrailingLogs,
		"should have reloaded to default trailing_logs")

	// Now check that update each of those raft fields separately works correctly
	// too.
}

func TestServer_computeRaftReloadableConfig(t *testing.T) {

	defaults := DefaultConfig().RaftConfig

	cases := []struct {
		name string
		rc   ReloadableConfig
		want raft.ReloadableConfig
	}{
		{
			// This case is the common path - reload is called with a ReloadableConfig
			// populated from the RuntimeConfig which has zero values for the fields.
			// On startup we selectively pick non-zero runtime config fields to
			// override defaults so we need to do the same.
			name: "Still defaults",
			rc:   ReloadableConfig{},
			want: raft.ReloadableConfig{
				SnapshotThreshold: defaults.SnapshotThreshold,
				SnapshotInterval:  defaults.SnapshotInterval,
				TrailingLogs:      defaults.TrailingLogs,
				ElectionTimeout:   defaults.ElectionTimeout,
				HeartbeatTimeout:  defaults.HeartbeatTimeout,
			},
		},
		{
			name: "Threshold set",
			rc: ReloadableConfig{
				RaftSnapshotThreshold: 123456,
			},
			want: raft.ReloadableConfig{
				SnapshotThreshold: 123456,
				SnapshotInterval:  defaults.SnapshotInterval,
				TrailingLogs:      defaults.TrailingLogs,
				ElectionTimeout:   defaults.ElectionTimeout,
				HeartbeatTimeout:  defaults.HeartbeatTimeout,
			},
		},
		{
			name: "interval set",
			rc: ReloadableConfig{
				RaftSnapshotInterval: 13 * time.Minute,
			},
			want: raft.ReloadableConfig{
				SnapshotThreshold: defaults.SnapshotThreshold,
				SnapshotInterval:  13 * time.Minute,
				TrailingLogs:      defaults.TrailingLogs,
				ElectionTimeout:   defaults.ElectionTimeout,
				HeartbeatTimeout:  defaults.HeartbeatTimeout,
			},
		},
		{
			name: "trailing logs set",
			rc: ReloadableConfig{
				RaftTrailingLogs: 78910,
			},
			want: raft.ReloadableConfig{
				SnapshotThreshold: defaults.SnapshotThreshold,
				SnapshotInterval:  defaults.SnapshotInterval,
				TrailingLogs:      78910,
				ElectionTimeout:   defaults.ElectionTimeout,
				HeartbeatTimeout:  defaults.HeartbeatTimeout,
			},
		},
		{
			name: "all set",
			rc: ReloadableConfig{
				RaftSnapshotThreshold: 123456,
				RaftSnapshotInterval:  13 * time.Minute,
				RaftTrailingLogs:      78910,
				ElectionTimeout:       300 * time.Millisecond,
				HeartbeatTimeout:      400 * time.Millisecond,
			},
			want: raft.ReloadableConfig{
				SnapshotThreshold: 123456,
				SnapshotInterval:  13 * time.Minute,
				TrailingLogs:      78910,
				ElectionTimeout:   300 * time.Millisecond,
				HeartbeatTimeout:  400 * time.Millisecond,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := computeRaftReloadableConfig(tc.rc)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestServer_RPC_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, conf1 := testServerConfig(t)
	conf1.RPCRateLimit = 2
	conf1.RPCMaxBurst = 2
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := s1.RPC(context.Background(), "Status.Ping", struct{}{}, &out); err != structs.ErrRPCRateExceeded {
			r.Fatalf("err: %v", err)
		}
	})
}

// TestServer_Peering_LeadershipCheck tests that a peering service can receive the leader address
// through the LeaderAddress IRL.
func TestServer_Peering_LeadershipCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// given two servers: s1 (leader), s2 (follower)
	_, conf1 := testServerConfig(t)
	s1, err := newServer(t, conf1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	_, conf2 := testServerConfig(t)
	conf2.Bootstrap = false
	s2, err := newServer(t, conf2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	// Try to join
	joinLAN(t, s2, s1)

	// Verify Raft has established a peer
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")
	waitForLeaderEstablishment(t, s1)

	// the actual tests
	// when leadership has been established s2 should have the address of s1
	// in the peering service
	peeringLeaderAddr := s2.peeringBackend.GetLeaderAddress()

	require.Equal(t, s1.config.RPCAddr.String(), peeringLeaderAddr)
	// test corollary by transitivity to future-proof against any setup bugs
	require.NotEqual(t, s2.config.RPCAddr.String(), peeringLeaderAddr)
}

func TestServer_hcpManager(t *testing.T) {
	_, conf1 := testServerConfig(t)
	conf1.BootstrapExpect = 1
	conf1.RPCAdvertise = &net.TCPAddr{IP: []byte{127, 0, 0, 2}, Port: conf1.RPCAddr.Port}
	hcp1 := hcpclient.NewMockClient(t)
	hcp1.EXPECT().PushServerStatus(mock.Anything, mock.MatchedBy(func(status *hcpclient.ServerStatus) bool {
		return status.ID == string(conf1.NodeID)
	})).Run(func(ctx context.Context, status *hcpclient.ServerStatus) {
		require.Equal(t, status.LanAddress, "127.0.0.2")
	}).Call.Return(nil)

	deps1 := newDefaultDeps(t, conf1)
	deps1.HCP.Client = hcp1
	s1, err := newServerWithDeps(t, conf1, deps1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()
	require.NotNil(t, s1.hcpManager)
	waitForLeaderEstablishment(t, s1)
	hcp1.AssertExpectations(t)

}
