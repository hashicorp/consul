package consul

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/types"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func waitForLeader(servers ...*Server) error {
	if len(servers) == 0 {
		return errors.New("no servers")
	}
	dc := servers[0].config.Datacenter
	for _, s := range servers {
		if s.config.Datacenter != dc {
			return fmt.Errorf("servers are in different datacenters %s and %s", s.config.Datacenter, dc)
		}
	}
	for _, s := range servers {
		if s.IsLeader() {
			return nil
		}
	}
	return errors.New("no leader")
}

// wantPeers determines whether the server has the given
// number of voting raft peers.
func wantPeers(s *Server, peers int) error {
	n, err := s.numPeers()
	if err != nil {
		return err
	}
	if got, want := n, peers; got != want {
		return fmt.Errorf("got %d peers want %d", got, want)
	}
	return nil
}

// wantRaft determines if the servers have all of each other in their
// Raft configurations,
func wantRaft(servers []*Server) error {
	// Make sure all the servers are represented in the Raft config,
	// and that there are no extras.
	verifyRaft := func(c raft.Configuration) error {
		want := make(map[raft.ServerID]bool)
		for _, s := range servers {
			want[s.config.RaftConfig.LocalID] = true
		}

		for _, s := range c.Servers {
			if !want[s.ID] {
				return fmt.Errorf("don't want %q", s.ID)
			}
			delete(want, s.ID)
		}

		if len(want) > 0 {
			return fmt.Errorf("didn't find %v", want)
		}
		return nil
	}

	for _, s := range servers {
		future := s.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		if err := verifyRaft(future.Configuration()); err != nil {
			return err
		}
	}
	return nil
}

// joinAddrLAN returns the address other servers can
// use to join the cluster on the LAN interface.
func joinAddrLAN(s *Server) string {
	if s == nil {
		panic("no server")
	}
	port := s.config.SerfLANConfig.MemberlistConfig.BindPort
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// joinAddrWAN returns the address other servers can
// use to join the cluster on the WAN interface.
func joinAddrWAN(s *Server) string {
	if s == nil {
		panic("no server")
	}
	name := s.config.NodeName
	dc := s.config.Datacenter
	port := s.config.SerfWANConfig.MemberlistConfig.BindPort
	return fmt.Sprintf("%s.%s/127.0.0.1:%d", name, dc, port)
}

type clientOrServer interface {
	JoinLAN(addrs []string) (int, error)
	LANMembers() []serf.Member
}

// joinLAN is a convenience function for
//
//   member.JoinLAN("127.0.0.1:"+leader.config.SerfLANConfig.MemberlistConfig.BindPort)
func joinLAN(t *testing.T, member clientOrServer, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	var memberAddr string
	switch x := member.(type) {
	case *Server:
		memberAddr = joinAddrLAN(x)
	case *Client:
		memberAddr = fmt.Sprintf("127.0.0.1:%d", x.config.SerfLANConfig.MemberlistConfig.BindPort)
	}
	leaderAddr := joinAddrLAN(leader)
	if _, err := member.JoinLAN([]string{leaderAddr}); err != nil {
		t.Fatal(err)
	}
	retry.Run(t, func(r *retry.R) {
		if !seeEachOther(leader.LANMembers(), member.LANMembers(), leaderAddr, memberAddr) {
			r.Fatalf("leader and member cannot see each other on LAN")
		}
	})
	if !seeEachOther(leader.LANMembers(), member.LANMembers(), leaderAddr, memberAddr) {
		t.Fatalf("leader and member cannot see each other on LAN")
	}
}

// joinWAN is a convenience function for
//
//   member.JoinWAN("127.0.0.1:"+leader.config.SerfWANConfig.MemberlistConfig.BindPort)
func joinWAN(t *testing.T, member, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	leaderAddr, memberAddr := joinAddrWAN(leader), joinAddrWAN(member)
	if _, err := member.JoinWAN([]string{leaderAddr}); err != nil {
		t.Fatal(err)
	}
	retry.Run(t, func(r *retry.R) {
		if !seeEachOther(leader.WANMembers(), member.WANMembers(), leaderAddr, memberAddr) {
			r.Fatalf("leader and member cannot see each other on WAN")
		}
	})
	if !seeEachOther(leader.WANMembers(), member.WANMembers(), leaderAddr, memberAddr) {
		t.Fatalf("leader and member cannot see each other on WAN")
	}
}

func waitForNewACLs(t *testing.T, server *Server) {
	retry.Run(t, func(r *retry.R) {
		require.False(r, server.UseLegacyACLs(), "Server cannot use new ACLs")
	})

	require.False(t, server.UseLegacyACLs(), "Server cannot use new ACLs")
}

func waitForNewACLReplication(t *testing.T, server *Server, expectedReplicationType structs.ACLReplicationType, minPolicyIndex, minTokenIndex, minRoleIndex uint64) {
	retry.Run(t, func(r *retry.R) {
		status := server.getACLReplicationStatus()
		require.Equal(r, expectedReplicationType, status.ReplicationType, "Server not running new replicator yet")
		require.True(r, status.Running, "Server not running new replicator yet")
		require.True(r, status.ReplicatedIndex >= minPolicyIndex, "Server hasn't replicated enough policies")
		require.True(r, status.ReplicatedTokenIndex >= minTokenIndex, "Server hasn't replicated enough tokens")
		require.True(r, status.ReplicatedRoleIndex >= minRoleIndex, "Server hasn't replicated enough roles")
	})
}

func seeEachOther(a, b []serf.Member, addra, addrb string) bool {
	return serfMembersContains(a, addrb) && serfMembersContains(b, addra)
}

func serfMembersContains(members []serf.Member, addr string) bool {
	// There are tests that manipulate the advertise address, so we just
	// compare port numbers here, since that uniquely identifies a member
	// as we use the loopback interface for everything.
	_, want, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	for _, m := range members {
		if got := fmt.Sprintf("%d", m.Port); got == want {
			return true
		}
	}
	return false
}

func registerTestCatalogEntries(t *testing.T, codec rpc.ClientCodec) {
	t.Helper()

	// prep the cluster with some data we can use in our filters
	registrations := map[string]*structs.RegisterRequest{
		"Node foo": &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			ID:         types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:    "127.0.0.2",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.2",
				"wan": "198.18.0.2",
			},
			NodeMeta: map[string]string{
				"env": "production",
				"os":  "linux",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "foo",
					CheckID: "foo:alive",
					Name:    "foo-liveness",
					Status:  api.HealthPassing,
					Notes:   "foo is alive and well",
				},
				&structs.HealthCheck{
					Node:    "foo",
					CheckID: "foo:ssh",
					Name:    "foo-remote-ssh",
					Status:  api.HealthPassing,
					Notes:   "foo has ssh access",
				},
			},
		},
		"Service redis v1 on foo": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redisV1",
				Service: "redis",
				Tags:    []string{"v1"},
				Meta:    map[string]string{"version": "1"},
				Port:    1234,
				Address: "198.18.1.2",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:redisV1",
					Name:        "redis-liveness",
					Status:      api.HealthPassing,
					Notes:       "redis v1 is alive and well",
					ServiceID:   "redisV1",
					ServiceName: "redis",
				},
			},
		},
		"Service redis v2 on foo": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redisV2",
				Service: "redis",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    1235,
				Address: "198.18.1.2",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:redisV2",
					Name:        "redis-v2-liveness",
					Status:      api.HealthPassing,
					Notes:       "redis v2 is alive and well",
					ServiceID:   "redisV2",
					ServiceName: "redis",
				},
			},
		},
		"Node bar": &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			ID:         types.NodeID("c6e7a976-8f4f-44b5-bdd3-631be7e8ecac"),
			Address:    "127.0.0.3",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.3",
				"wan": "198.18.0.3",
			},
			NodeMeta: map[string]string{
				"env": "production",
				"os":  "windows",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "bar",
					CheckID: "bar:alive",
					Name:    "bar-liveness",
					Status:  api.HealthPassing,
					Notes:   "bar is alive and well",
				},
			},
		},
		"Service redis v1 on bar": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redisV1",
				Service: "redis",
				Tags:    []string{"v1"},
				Meta:    map[string]string{"version": "1"},
				Port:    1234,
				Address: "198.18.1.3",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "bar",
					CheckID:     "bar:redisV1",
					Name:        "redis-liveness",
					Status:      api.HealthPassing,
					Notes:       "redis v1 is alive and well",
					ServiceID:   "redisV1",
					ServiceName: "redis",
				},
			},
		},
		"Service web v1 on bar": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "webV1",
				Service: "web",
				Tags:    []string{"v1", "connect"},
				Meta:    map[string]string{"version": "1", "connect": "enabled"},
				Port:    443,
				Address: "198.18.1.4",
				Connect: structs.ServiceConnect{Native: true},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "bar",
					CheckID:     "bar:web:v1",
					Name:        "web-v1-liveness",
					Status:      api.HealthPassing,
					Notes:       "web connect v1 is alive and well",
					ServiceID:   "webV1",
					ServiceName: "web",
				},
			},
		},
		"Node baz": &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			ID:         types.NodeID("12f96b27-a7b0-47bd-add7-044a2bfc7bfb"),
			Address:    "127.0.0.4",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.4",
			},
			NodeMeta: map[string]string{
				"env": "qa",
				"os":  "linux",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "baz",
					CheckID: "baz:alive",
					Name:    "baz-liveness",
					Status:  api.HealthPassing,
					Notes:   "baz is alive and well",
				},
				&structs.HealthCheck{
					Node:    "baz",
					CheckID: "baz:ssh",
					Name:    "baz-remote-ssh",
					Status:  api.HealthPassing,
					Notes:   "baz has ssh access",
				},
			},
		},
		"Service web v1 on baz": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "webV1",
				Service: "web",
				Tags:    []string{"v1", "connect"},
				Meta:    map[string]string{"version": "1", "connect": "enabled"},
				Port:    443,
				Address: "198.18.1.4",
				Connect: structs.ServiceConnect{Native: true},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web:v1",
					Name:        "web-v1-liveness",
					Status:      api.HealthPassing,
					Notes:       "web connect v1 is alive and well",
					ServiceID:   "webV1",
					ServiceName: "web",
				},
			},
		},
		"Service web v2 on baz": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "webV2",
				Service: "web",
				Tags:    []string{"v2", "connect"},
				Meta:    map[string]string{"version": "2", "connect": "enabled"},
				Port:    8443,
				Address: "198.18.1.4",
				Connect: structs.ServiceConnect{Native: true},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web:v2",
					Name:        "web-v2-liveness",
					Status:      api.HealthPassing,
					Notes:       "web connect v2 is alive and well",
					ServiceID:   "webV2",
					ServiceName: "web",
				},
			},
		},
		"Service critical on baz": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "criticalV2",
				Service: "critical",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    8080,
				Address: "198.18.1.4",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:critical:v2",
					Name:        "critical-v2-liveness",
					Status:      api.HealthCritical,
					Notes:       "critical v2 is in the critical state",
					ServiceID:   "criticalV2",
					ServiceName: "critical",
				},
			},
		},
		"Service warning on baz": &structs.RegisterRequest{
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "warningV2",
				Service: "warning",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    8081,
				Address: "198.18.1.4",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:warning:v2",
					Name:        "warning-v2-liveness",
					Status:      api.HealthWarning,
					Notes:       "warning v2 is in the warning state",
					ServiceID:   "warningV2",
					ServiceName: "warning",
				},
			},
		},
	}

	registerTestCatalogEntriesMap(t, codec, registrations)
}

func registerTestCatalogEntriesMeshGateway(t *testing.T, codec rpc.ClientCodec) {
	t.Helper()

	registrations := map[string]*structs.RegisterRequest{
		"Service mg-gw": &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "gateway",
			ID:         types.NodeID("72e18a4c-85ec-4520-978f-2fc0378b06aa"),
			Address:    "10.1.2.3",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mg-gw-01",
				Service: "mg-gw",
				Port:    8443,
				Address: "198.18.1.4",
			},
		},
		"Service web-proxy": &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "proxy",
			ID:         types.NodeID("2d31602c-3291-4f94-842d-446bc2f945ce"),
			Address:    "10.1.2.4",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-proxy",
				Service: "web-proxy",
				Port:    8443,
				Address: "198.18.1.5",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
				},
			},
		},
	}

	registerTestCatalogEntriesMap(t, codec, registrations)
}

func registerTestCatalogEntriesMap(t *testing.T, codec rpc.ClientCodec, registrations map[string]*structs.RegisterRequest) {
	t.Helper()
	for name, reg := range registrations {
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", reg, nil)
		require.NoError(t, err, "Failed catalog registration %q: %v", name, err)
	}
}
