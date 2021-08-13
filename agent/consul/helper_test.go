package consul

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/types"
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
	n, err := s.autopilot.NumVoters()
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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

	retry.Run(t, func(r *retry.R) {
		require.False(r, server.UseLegacyACLs(), "Server cannot use new ACLs")
	})

	require.False(t, server.UseLegacyACLs(), "Server cannot use new ACLs")
}

func waitForNewACLReplication(t *testing.T, server *Server, expectedReplicationType structs.ACLReplicationType, minPolicyIndex, minTokenIndex, minRoleIndex uint64) {
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		status := server.getACLReplicationStatus()
		require.Equal(r, expectedReplicationType, status.ReplicationType, "Server not running new replicator yet")
		require.True(r, status.Running, "Server not running new replicator yet")
		require.True(r, status.ReplicatedIndex >= minPolicyIndex, "Server hasn't replicated enough policies")
		require.True(r, status.ReplicatedTokenIndex >= minTokenIndex, "Server hasn't replicated enough tokens")
		require.True(r, status.ReplicatedRoleIndex >= minRoleIndex, "Server hasn't replicated enough roles")
	})
}

func waitForFederationStateFeature(t *testing.T, server *Server) {
	t.Helper()

	retry.Run(t, func(r *retry.R) {
		require.True(r, server.DatacenterSupportsFederationStates())
	})

	require.True(t, server.DatacenterSupportsFederationStates())
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
		"Node foo": {
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
		"Service redis v1 on foo": {
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
		"Service redis v2 on foo": {
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
		"Node bar": {
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
		"Service redis v1 on bar": {
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
		"Service web v1 on bar": {
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
		"Node baz": {
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
		"Service web v1 on baz": {
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
		"Service web v2 on baz": {
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
		"Service critical on baz": {
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
		"Service warning on baz": {
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

func registerTestCatalogProxyEntries(t *testing.T, codec rpc.ClientCodec) {
	t.Helper()

	registrations := map[string]*structs.RegisterRequest{
		"Service tg-gw": {
			Datacenter: "dc1",
			Node:       "terminating-gateway",
			ID:         types.NodeID("3a9d7530-20d4-443a-98d3-c10fe78f09f4"),
			Address:    "10.1.2.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				ID:      "tg-gw-01",
				Service: "tg-gw",
				Port:    8443,
				Address: "198.18.1.3",
			},
		},
		"Service mg-gw": {
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
		"Service web-proxy": {
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

func registerTestTopologyEntries(t *testing.T, codec rpc.ClientCodec, token string) {
	t.Helper()

	// ingress-gateway on node edge - upstream: api
	// api and api-proxy on node foo - transparent proxy
	// web and web-proxy on node bar - upstream: redis
	// web and web-proxy on node baz - transparent proxy
	// redis and redis-proxy on node zip
	registrations := map[string]*structs.RegisterRequest{
		"Node edge": {
			Datacenter: "dc1",
			Node:       "edge",
			ID:         types.NodeID("8e3481c0-760e-4b5f-a3b8-6c8c559e8a15"),
			Address:    "127.0.0.1",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "edge",
					CheckID: "edge:alive",
					Name:    "edge-liveness",
					Status:  api.HealthPassing,
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service ingress on edge": {
			Datacenter:     "dc1",
			Node:           "edge",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindIngressGateway,
				ID:      "ingress",
				Service: "ingress",
				Port:    8443,
				Address: "198.18.1.1",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "edge",
					CheckID:     "edge:ingress",
					Name:        "ingress-liveness",
					Status:      api.HealthPassing,
					ServiceID:   "ingress",
					ServiceName: "ingress",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Node foo": {
			Datacenter: "dc1",
			Node:       "foo",
			ID:         types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:    "127.0.0.2",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "foo",
					CheckID: "foo:alive",
					Name:    "foo-liveness",
					Status:  api.HealthPassing,
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service api on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "api",
				Service: "api",
				Port:    9090,
				Address: "198.18.1.2",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:api",
					Name:        "api-liveness",
					Status:      api.HealthPassing,
					ServiceID:   "api",
					ServiceName: "api",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service api-proxy": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Port:    8443,
				Address: "198.18.1.2",
				Proxy: structs.ConnectProxyConfig{
					Mode:                   structs.ProxyModeTransparent,
					DestinationServiceName: "api",
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:api-proxy",
					Name:        "api proxy listening",
					Status:      api.HealthPassing,
					ServiceID:   "api-proxy",
					ServiceName: "api-proxy",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Node bar": {
			Datacenter: "dc1",
			Node:       "bar",
			ID:         types.NodeID("c3e5fc07-3b2d-4c06-b8fc-a1a12432d459"),
			Address:    "127.0.0.3",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "bar",
					CheckID: "bar:alive",
					Name:    "bar-liveness",
					Status:  api.HealthPassing,
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web on bar": {
			Datacenter:     "dc1",
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "web",
				Service: "web",
				Port:    80,
				Address: "198.18.1.20",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "bar",
					CheckID:     "bar:web",
					Name:        "web-liveness",
					Status:      api.HealthWarning,
					ServiceID:   "web",
					ServiceName: "web",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web-proxy on bar": {
			Datacenter:     "dc1",
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-proxy",
				Service: "web-proxy",
				Port:    8443,
				Address: "198.18.1.20",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
					Upstreams: structs.Upstreams{
						{
							DestinationName: "redis",
							LocalBindPort:   123,
						},
					},
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "bar",
					CheckID:     "bar:web-proxy",
					Name:        "web proxy listening",
					Status:      api.HealthCritical,
					ServiceID:   "web-proxy",
					ServiceName: "web-proxy",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Node baz": {
			Datacenter: "dc1",
			Node:       "baz",
			ID:         types.NodeID("37ea7c44-a2a1-4764-ae28-7dfebeb54a22"),
			Address:    "127.0.0.4",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "baz",
					CheckID: "baz:alive",
					Name:    "baz-liveness",
					Status:  api.HealthPassing,
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web on baz": {
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "web",
				Service: "web",
				Port:    80,
				Address: "198.18.1.40",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web",
					Name:        "web-liveness",
					Status:      api.HealthPassing,
					ServiceID:   "web",
					ServiceName: "web",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web-proxy on baz": {
			Datacenter:     "dc1",
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-proxy",
				Service: "web-proxy",
				Port:    8443,
				Address: "198.18.1.40",
				Proxy: structs.ConnectProxyConfig{
					Mode:                   structs.ProxyModeTransparent,
					DestinationServiceName: "web",
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web-proxy",
					Name:        "web proxy listening",
					Status:      api.HealthCritical,
					ServiceID:   "web-proxy",
					ServiceName: "web-proxy",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Node zip": {
			Datacenter: "dc1",
			Node:       "zip",
			ID:         types.NodeID("dc49fc8c-afc7-4a87-815d-74d144535075"),
			Address:    "127.0.0.5",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "zip",
					CheckID: "zip:alive",
					Name:    "zip-liveness",
					Status:  api.HealthPassing,
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service redis on zip": {
			Datacenter:     "dc1",
			Node:           "zip",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redis",
				Service: "redis",
				Port:    6379,
				Address: "198.18.1.60",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "zip",
					CheckID:     "zip:redis",
					Name:        "redis-liveness",
					Status:      api.HealthPassing,
					ServiceID:   "redis",
					ServiceName: "redis",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service redis-proxy on zip": {
			Datacenter:     "dc1",
			Node:           "zip",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "redis-proxy",
				Service: "redis-proxy",
				Port:    8443,
				Address: "198.18.1.60",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "redis",
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "zip",
					CheckID:     "zip:redis-proxy",
					Name:        "redis proxy listening",
					Status:      api.HealthCritical,
					ServiceID:   "redis-proxy",
					ServiceName: "redis-proxy",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
	}
	registerTestCatalogEntriesMap(t, codec, registrations)

	// ingress -> api gateway config entry (but no intention)
	// wildcard deny intention
	// api -> web exact intention
	// web -> redis exact intention
	entries := []structs.ConfigEntryRequest{
		{
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.IngressGatewayConfigEntry{
				Kind: structs.IngressGateway,
				Name: "ingress",
				Listeners: []structs.IngressListener{
					{
						Port:     8443,
						Protocol: "http",
						Services: []structs.IngressService{
							{
								Name: "api",
							},
						},
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "web",
				Sources: []*structs.SourceIntention{
					{
						Action: structs.IntentionActionAllow,
						Name:   "api",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "redis",
				Sources: []*structs.SourceIntention{
					{
						Name: "web",
						Permissions: []*structs.IntentionPermission{
							{
								Action: structs.IntentionActionAllow,
								HTTP: &structs.IntentionHTTPPermission{
									Methods: []string{"GET"},
								},
							},
						},
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "*",
				Meta: map[string]string{structs.MetaExternalSource: "nomad"},
				Sources: []*structs.SourceIntention{
					{
						Name:   "*",
						Action: structs.IntentionActionDeny,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
	}
	for _, req := range entries {
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
	}
}

func registerTestRoutingConfigTopologyEntries(t *testing.T, codec rpc.ClientCodec) {
	registrations := map[string]*structs.RegisterRequest{
		"Service dashboard": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "dashboard",
				Service: "dashboard",
				Port:    9002,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:dashboard",
					Status:      api.HealthPassing,
					ServiceID:   "dashboard",
					ServiceName: "dashboard",
				},
			},
		},
		"Service dashboard-proxy": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "dashboard-sidecar-proxy",
				Service: "dashboard-sidecar-proxy",
				Port:    5000,
				Address: "198.18.1.0",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "dashboard",
					DestinationServiceID:   "dashboard",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       9002,
					Upstreams: []structs.Upstream{
						{
							DestinationType: "service",
							DestinationName: "routing-config",
							LocalBindPort:   5000,
						},
					},
				},
				LocallyRegisteredAsSidecar: true,
			},
		},
		"Service counting": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "counting",
				Service: "counting",
				Port:    9003,
				Address: "198.18.1.1",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:api",
					Status:      api.HealthPassing,
					ServiceID:   "counting",
					ServiceName: "counting",
				},
			},
		},
		"Service counting-proxy": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "counting-proxy",
				Service: "counting-proxy",
				Port:    5001,
				Address: "198.18.1.1",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "counting",
				},
				LocallyRegisteredAsSidecar: true,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:counting-proxy",
					Status:      api.HealthPassing,
					ServiceID:   "counting-proxy",
					ServiceName: "counting-proxy",
				},
			},
		},
		"Service counting-v2": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "counting-v2",
				Service: "counting-v2",
				Port:    9004,
				Address: "198.18.1.2",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:api",
					Status:      api.HealthPassing,
					ServiceID:   "counting-v2",
					ServiceName: "counting-v2",
				},
			},
		},
		"Service counting-v2-proxy": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "counting-v2-proxy",
				Service: "counting-v2-proxy",
				Port:    5002,
				Address: "198.18.1.2",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "counting-v2",
				},
				LocallyRegisteredAsSidecar: true,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "foo",
					CheckID:     "foo:counting-v2-proxy",
					Status:      api.HealthPassing,
					ServiceID:   "counting-v2-proxy",
					ServiceName: "counting-v2-proxy",
				},
			},
		},
	}
	registerTestCatalogEntriesMap(t, codec, registrations)

	entries := []structs.ConfigEntryRequest{
		{
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceRouterConfigEntry{
				Kind: structs.ServiceRouter,
				Name: "routing-config",
				Routes: []structs.ServiceRoute{
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{
								PathPrefix: "/v2",
							},
						},
						Destination: &structs.ServiceRouteDestination{
							Service: "counting-v2",
						},
					},
					{
						Match: &structs.ServiceRouteMatch{
							HTTP: &structs.ServiceRouteHTTPMatch{
								PathPrefix: "/",
							},
						},
						Destination: &structs.ServiceRouteDestination{
							Service: "counting",
						},
					},
				},
			},
		},
	}
	for _, req := range entries {
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
	}
}

func registerIntentionUpstreamEntries(t *testing.T, codec rpc.ClientCodec, token string) {
	t.Helper()

	// api and api-proxy on node foo
	// web and web-proxy on node foo
	// redis and redis-proxy on node foo
	// * -> * (deny) intention
	// web -> api (allow)
	registrations := map[string]*structs.RegisterRequest{
		"Node foo": {
			Datacenter:   "dc1",
			Node:         "foo",
			ID:           types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:      "127.0.0.2",
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service api on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "api",
				Service: "api",
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service api-proxy": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "api-proxy",
				Service: "api-proxy",
				Port:    8443,
				Address: "198.18.1.2",
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "api",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "web",
				Service: "web",
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service web-proxy on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "web-proxy",
				Service: "web-proxy",
				Port:    8080,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "web",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service redis on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redis",
				Service: "redis",
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		"Service redis-proxy on foo": {
			Datacenter:     "dc1",
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "redis-proxy",
				Service: "redis-proxy",
				Port:    1234,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "redis",
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
	}
	registerTestCatalogEntriesMap(t, codec, registrations)

	// Add intentions: deny all and web -> api
	entries := []structs.ConfigEntryRequest{
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "api",
				Sources: []*structs.SourceIntention{
					{
						Name:   "web",
						Action: structs.IntentionActionAllow,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceIntentionsConfigEntry{
				Kind: structs.ServiceIntentions,
				Name: "*",
				Sources: []*structs.SourceIntention{
					{
						Name:   "*",
						Action: structs.IntentionActionDeny,
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: token},
		},
	}
	for _, req := range entries {
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
	}
}
