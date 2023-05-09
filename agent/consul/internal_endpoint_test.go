package consul

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestInternal_NodeInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(config *Config) {
		config.PeeringTestAllowPeerRegistrations = true
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	args := []*structs.RegisterRequest{
		{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthPassing,
				ServiceID: "db",
			},
		},
		{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.3",
			PeerName:   "peer1",
		},
	}

	for _, reg := range args {
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", reg, nil)
		require.NoError(t, err)
	}

	t.Run("get local node", func(t *testing.T) {
		var out structs.IndexedNodeDump
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       "foo",
		}
		if err := msgpackrpc.CallWithCodec(codec, "Internal.NodeInfo", &req, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		nodes := out.Dump
		if len(nodes) != 1 {
			t.Fatalf("Bad: %v", nodes)
		}
		if nodes[0].Node != "foo" {
			t.Fatalf("Bad: %v", nodes[0])
		}
		if !stringslice.Contains(nodes[0].Services[0].Tags, "primary") {
			t.Fatalf("Bad: %v", nodes[0])
		}
		if nodes[0].Checks[0].Status != api.HealthPassing {
			t.Fatalf("Bad: %v", nodes[0])
		}
	})

	t.Run("get peered node", func(t *testing.T) {
		var out structs.IndexedNodeDump
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       "foo",
			PeerName:   "peer1",
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeInfo", &req, &out))

		nodes := out.Dump
		require.Equal(t, 1, len(nodes))
		require.Equal(t, "foo", nodes[0].Node)
		require.Equal(t, "peer1", nodes[0].PeerName)
	})
}

func TestInternal_NodeDump(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(config *Config) {
		config.PeeringTestAllowPeerRegistrations = true
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	args := []*structs.RegisterRequest{
		{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthPassing,
				ServiceID: "db",
			},
		},
		{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"replica"},
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		},
		{
			Datacenter: "dc1",
			Node:       "foo-peer",
			Address:    "127.0.0.3",
			PeerName:   "peer1",
		},
	}

	for _, reg := range args {
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", reg, nil)
		require.NoError(t, err)
	}

	err := s1.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			Name: "peer1",
		},
	})
	require.NoError(t, err)

	var out2 structs.IndexedNodeDump
	req := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes := out2.Dump
	if len(nodes) != 3 {
		t.Fatalf("Bad: %v", nodes)
	}

	var foundFoo, foundBar bool
	for _, node := range nodes {
		switch node.Node {
		case "foo":
			foundFoo = true
			if !stringslice.Contains(node.Services[0].Tags, "primary") {
				t.Fatalf("Bad: %v", nodes[0])
			}
			if node.Checks[0].Status != api.HealthPassing {
				t.Fatalf("Bad: %v", nodes[0])
			}

		case "bar":
			foundBar = true
			if !stringslice.Contains(node.Services[0].Tags, "replica") {
				t.Fatalf("Bad: %v", nodes[1])
			}
			if node.Checks[0].Status != api.HealthWarning {
				t.Fatalf("Bad: %v", nodes[1])
			}

		default:
			continue
		}
	}
	if !foundFoo || !foundBar {
		t.Fatalf("missing foo or bar")
	}

	require.Len(t, out2.ImportedDump, 1)
	require.Equal(t, "peer1", out2.ImportedDump[0].PeerName)
	require.Equal(t, "foo-peer", out2.ImportedDump[0].Node)
}

func TestInternal_NodeDump_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(config *Config) {
		config.PeeringTestAllowPeerRegistrations = true
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	args := []*structs.RegisterRequest{
		{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"primary"},
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthPassing,
				ServiceID: "db",
			},
		},
		{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Tags:    []string{"replica"},
			},
			Check: &structs.HealthCheck{
				Name:      "db connect",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		},
		{
			Datacenter: "dc1",
			Node:       "foo-peer",
			Address:    "127.0.0.3",
			PeerName:   "peer1",
		},
	}

	for _, reg := range args {
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", reg, nil)
		require.NoError(t, err)
	}

	err := s1.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			Name: "peer1",
		},
	})
	require.NoError(t, err)

	t.Run("filter on the local node", func(t *testing.T) {
		var out2 structs.IndexedNodeDump
		req := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Filter: "primary in Services.Tags"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req, &out2))

		nodes := out2.Dump
		require.Len(t, nodes, 1)
		require.Equal(t, "foo", nodes[0].Node)
	})

	t.Run("filter on imported dump", func(t *testing.T) {
		var out3 structs.IndexedNodeDump
		req2 := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Filter: "friend in PeerName"},
		}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req2, &out3))
		require.Len(t, out3.Dump, 0)
		require.Len(t, out3.ImportedDump, 0)
	})

	t.Run("filter look for peer nodes (non local nodes)", func(t *testing.T) {
		var out3 structs.IndexedNodeDump
		req2 := structs.DCSpecificRequest{
			QueryOptions: structs.QueryOptions{Filter: "PeerName != \"\""},
		}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req2, &out3))
		require.Len(t, out3.Dump, 0)
		require.Len(t, out3.ImportedDump, 1)
	})

	t.Run("filter look for a specific peer", func(t *testing.T) {
		var out3 structs.IndexedNodeDump
		req2 := structs.DCSpecificRequest{
			QueryOptions: structs.QueryOptions{Filter: "PeerName == peer1"},
		}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req2, &out3))
		require.Len(t, out3.Dump, 0)
		require.Len(t, out3.ImportedDump, 1)
	})
}

func TestInternal_KeyringOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key1 := "H1dfkSZOVnP/JUnaBfTzXg=="
	keyBytes1, err := base64.StdEncoding.DecodeString(key1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var out structs.KeyringResponses
	req := structs.KeyringRequest{
		Operation:  structs.KeyringList,
		Datacenter: "dc1",
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.KeyringOperation", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Two responses (local lan/wan pools) from single-node cluster
	if len(out.Responses) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	if _, ok := out.Responses[0].Keys[key1]; !ok {
		t.Fatalf("bad: %#v", out)
	}
	wanResp, lanResp := 0, 0
	for _, resp := range out.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 1 || wanResp != 1 {
		t.Fatalf("should have one lan and one wan response")
	}

	// Start a second agent to test cross-dc queries
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.Datacenter = "dc2"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)

	var out2 structs.KeyringResponses
	req2 := structs.KeyringRequest{
		Operation: structs.KeyringList,
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.KeyringOperation", &req2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 3 responses (one from each DC LAN, one from WAN) in two-node cluster
	if len(out2.Responses) != 3 {
		t.Fatalf("bad: %#v", out2)
	}
	wanResp, lanResp = 0, 0
	for _, resp := range out2.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 2 || wanResp != 1 {
		t.Fatalf("should have two lan and one wan response")
	}
}

func TestInternal_KeyringOperationList_LocalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key1 := "H1dfkSZOVnP/JUnaBfTzXg=="
	keyBytes1, err := base64.StdEncoding.DecodeString(key1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Start a second agent to test cross-dc queries
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.Datacenter = "dc2"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)

	// --
	// Try request with `LocalOnly` set to true
	var out structs.KeyringResponses
	req := structs.KeyringRequest{
		Operation: structs.KeyringList,
		LocalOnly: true,
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.KeyringOperation", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 1 response (from this DC LAN)
	if len(out.Responses) != 1 {
		t.Fatalf("expected num responses to be 1, got %d; out is: %#v", len(out.Responses), out)
	}
	wanResp, lanResp := 0, 0
	for _, resp := range out.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 1 || wanResp != 0 {
		t.Fatalf("should have 1 lan and 0 wan response, got (lan=%d) (wan=%d)", lanResp, wanResp)
	}

	// --
	// Try same request again but with `LocalOnly` set to false
	req.LocalOnly = false
	if err := msgpackrpc.CallWithCodec(codec, "Internal.KeyringOperation", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 3 responses (one from each DC LAN, one from WAN)
	if len(out.Responses) != 3 {
		t.Fatalf("expected num responses to be 3, got %d; out is: %#v", len(out.Responses), out)
	}
	wanResp, lanResp = 0, 0
	for _, resp := range out.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 2 || wanResp != 1 {
		t.Fatalf("should have 2 lan and 1 wan response, got (lan=%d) (wan=%d)", lanResp, wanResp)
	}
}

func TestInternal_KeyringOperationWrite_LocalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	key1 := "H1dfkSZOVnP/JUnaBfTzXg=="
	keyBytes1, err := base64.StdEncoding.DecodeString(key1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Try request with `LocalOnly` set to true
	var out structs.KeyringResponses
	req := structs.KeyringRequest{
		Operation: structs.KeyringRemove,
		LocalOnly: true,
	}
	err = msgpackrpc.CallWithCodec(codec, "Internal.KeyringOperation", &req, &out)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "LocalOnly") {
		t.Fatalf("expected error to contain string 'LocalOnly'. Got: %v", err)
	}
}

func TestInternal_NodeInfo_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.NodeSpecificRequest{
		Datacenter:   "dc1",
		Node:         srv.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedNodeDump{}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.NodeInfo", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, info := range reply.Dump {
		found := false
		for _, chk := range info.Checks {
			if chk.ServiceName == "foo" {
				found = true
			}
			if chk.ServiceName == "bar" {
				t.Fatalf("bad: %#v", info.Checks)
			}
		}
		if !found {
			t.Fatalf("bad: %#v", info.Checks)
		}

		found = false
		for _, svc := range info.Services {
			if svc.Service == "foo" {
				found = true
			}
			if svc.Service == "bar" {
				t.Fatalf("bad: %#v", info.Services)
			}
		}
		if !found {
			t.Fatalf("bad: %#v", info.Services)
		}
	}

	if !reply.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestInternal_NodeDump_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedNodeDump{}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, info := range reply.Dump {
		found := false
		for _, chk := range info.Checks {
			if chk.ServiceName == "foo" {
				found = true
			}
			if chk.ServiceName == "bar" {
				t.Fatalf("bad: %#v", info.Checks)
			}
		}
		if !found {
			t.Fatalf("bad: %#v", info.Checks)
		}

		found = false
		for _, svc := range info.Services {
			if svc.Service == "foo" {
				found = true
			}
			if svc.Service == "bar" {
				t.Fatalf("bad: %#v", info.Services)
			}
		}
		if !found {
			t.Fatalf("bad: %#v", info.Services)
		}
	}

	if !reply.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestInternal_EventFire_Token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, srv := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDownPolicy = "deny"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir)
	defer srv.Shutdown()

	codec := rpcClient(t, srv)
	defer codec.Close()

	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	// No token is rejected
	event := structs.EventFireRequest{
		Name:       "foo",
		Datacenter: "dc1",
		Payload:    []byte("nope"),
	}
	err := msgpackrpc.CallWithCodec(codec, "Internal.EventFire", &event, nil)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %s", err)
	}

	// Root token is allowed to fire
	event.Token = "root"
	err = msgpackrpc.CallWithCodec(codec, "Internal.EventFire", &event, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestInternal_ServiceDump(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// prep the cluster with some data we can use in our filters
	registerTestCatalogEntries(t, codec)

	// Register a gateway config entry to ensure gateway-services is dumped
	{
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.TerminatingGatewayConfigEntry{
				Name: "terminating-gateway",
				Kind: structs.TerminatingGateway,
				Services: []structs.LinkedService{
					{
						Name: "api",
					},
					{
						Name: "cache",
					},
				},
			},
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	doRequest := func(t *testing.T, filter string) structs.IndexedNodesWithGateways {
		t.Helper()
		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Filter: filter},
		}

		var out structs.IndexedNodesWithGateways
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out))

		// The GatewayServices dump is currently cannot be bexpr filtered
		// so the response should be the same in all subtests
		expectedGW := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
			},
			{
				Service:     structs.NewServiceName("cache", nil),
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
			},
		}
		assert.Len(t, out.Gateways, 2)
		assert.Equal(t, expectedGW[0].Service, out.Gateways[0].Service)
		assert.Equal(t, expectedGW[0].Gateway, out.Gateways[0].Gateway)
		assert.Equal(t, expectedGW[0].GatewayKind, out.Gateways[0].GatewayKind)

		assert.Equal(t, expectedGW[1].Service, out.Gateways[1].Service)
		assert.Equal(t, expectedGW[1].Gateway, out.Gateways[1].Gateway)
		assert.Equal(t, expectedGW[1].GatewayKind, out.Gateways[1].GatewayKind)

		return out
	}

	// Run the tests against the test server
	t.Run("No Filter", func(t *testing.T) {
		nodes := doRequest(t, "")
		// redis (3), web (3), critical (1), warning (1) and consul (1)
		require.Len(t, nodes.Nodes, 9)

	})

	t.Run("Filter Node foo and service version 1", func(t *testing.T) {
		resp := doRequest(t, "Node.Node == foo and Service.Meta.version == 1")
		require.Len(t, resp.Nodes, 1)
		require.Equal(t, "redis", resp.Nodes[0].Service.Service)
		require.Equal(t, "redisV1", resp.Nodes[0].Service.ID)
	})

	t.Run("Filter service web", func(t *testing.T) {
		resp := doRequest(t, "Service.Service == web")
		require.Len(t, resp.Nodes, 3)
	})
}

func TestInternal_ServiceDump_Kind(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// prep the cluster with some data we can use in our filters
	registerTestCatalogEntries(t, codec)
	registerTestCatalogProxyEntries(t, codec)

	doRequest := func(t *testing.T, kind structs.ServiceKind) structs.CheckServiceNodes {
		t.Helper()
		args := structs.ServiceDumpRequest{
			Datacenter:     "dc1",
			ServiceKind:    kind,
			UseServiceKind: true,
		}

		var out structs.IndexedNodesWithGateways
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out))
		return out.Nodes
	}

	// Run the tests against the test server
	t.Run("Typical", func(t *testing.T) {
		nodes := doRequest(t, structs.ServiceKindTypical)
		// redis (3), web (3), critical (1), warning (1) and consul (1)
		require.Len(t, nodes, 9)
	})

	t.Run("Terminating Gateway", func(t *testing.T) {
		nodes := doRequest(t, structs.ServiceKindTerminatingGateway)
		require.Len(t, nodes, 1)
		require.Equal(t, "tg-gw", nodes[0].Service.Service)
		require.Equal(t, "tg-gw-01", nodes[0].Service.ID)
	})

	t.Run("Mesh Gateway", func(t *testing.T) {
		nodes := doRequest(t, structs.ServiceKindMeshGateway)
		require.Len(t, nodes, 1)
		require.Equal(t, "mg-gw", nodes[0].Service.Service)
		require.Equal(t, "mg-gw-01", nodes[0].Service.ID)
	})

	t.Run("Connect Proxy", func(t *testing.T) {
		nodes := doRequest(t, structs.ServiceKindConnectProxy)
		require.Len(t, nodes, 1)
		require.Equal(t, "web-proxy", nodes[0].Service.Service)
		require.Equal(t, "web-proxy", nodes[0].Service.ID)
	})
}

func TestInternal_ServiceDump_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir, s := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	defer codec.Close()

	testrpc.WaitForLeader(t, s.RPC, "dc1")

	registrations := []*structs.RegisterRequest{
		// Service `redis` on `node1`
		{
			Datacenter: "dc1",
			Node:       "node1",
			ID:         types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:    "192.18.1.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTypical,
				ID:      "redis",
				Service: "redis",
				Port:    5678,
			},
			Check: &structs.HealthCheck{
				Name:      "redis check",
				Status:    api.HealthPassing,
				ServiceID: "redis",
			},
		},
		// Ingress gateway `igw` on `node2`
		{
			Datacenter: "dc1",
			Node:       "node2",
			ID:         types.NodeID("3a9d7530-20d4-443a-98d3-c10fe78f09f4"),
			Address:    "192.18.1.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindIngressGateway,
				ID:      "igw",
				Service: "igw",
			},
			Check: &structs.HealthCheck{
				Name:      "igw check",
				Status:    api.HealthPassing,
				ServiceID: "igw",
			},
		},
	}
	for _, reg := range registrations {
		reg.Token = "root"
		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", reg, nil)
		require.NoError(t, err)
	}

	{
		req := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.IngressGatewayConfigEntry{
				Kind: structs.IngressGateway,
				Name: "igw",
				Listeners: []structs.IngressListener{
					{
						Port:     8765,
						Protocol: "tcp",
						Services: []structs.IngressService{
							{Name: "redis"},
						},
					},
				},
			},
		}
		req.Token = "root"

		var out bool
		err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out)
		require.NoError(t, err)
	}

	tokenWithRules := func(t *testing.T, rules string) string {
		t.Helper()
		tok, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", rules)
		require.NoError(t, err)
		return tok.SecretID
	}

	t.Run("can read all", func(t *testing.T) {

		token := tokenWithRules(t, `
			node_prefix "" {
				policy = "read"
			}
			service_prefix "" {
				policy = "read"
			}
		`)

		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var out structs.IndexedNodesWithGateways
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out)
		require.NoError(t, err)
		require.NotEmpty(t, out.Nodes)
		require.NotEmpty(t, out.Gateways)
		require.False(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	t.Run("cannot read service node", func(t *testing.T) {

		token := tokenWithRules(t, `
			node "node1" {
				policy = "deny"
			}
			service "redis" {
				policy = "read"
			}
		`)

		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var out structs.IndexedNodesWithGateways
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out)
		require.NoError(t, err)
		require.Empty(t, out.Nodes)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("cannot read service", func(t *testing.T) {

		token := tokenWithRules(t, `
			node "node1" {
				policy = "read"
			}
			service "redis" {
				policy = "deny"
			}
		`)

		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var out structs.IndexedNodesWithGateways
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out)
		require.NoError(t, err)
		require.Empty(t, out.Nodes)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("cannot read gateway node", func(t *testing.T) {

		token := tokenWithRules(t, `
			node "node2" {
				policy = "deny"
			}
			service "mgw" {
				policy = "read"
			}
		`)

		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var out structs.IndexedNodesWithGateways
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out)
		require.NoError(t, err)
		require.Empty(t, out.Gateways)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("cannot read gateway", func(t *testing.T) {

		token := tokenWithRules(t, `
			node "node2" {
				policy = "read"
			}
			service "mgw" {
				policy = "deny"
			}
		`)

		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}
		var out structs.IndexedNodesWithGateways
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out)
		require.NoError(t, err)
		require.Empty(t, out.Gateways)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})
}

func TestInternal_GatewayServiceDump_Terminating(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register gateway and two service instances that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db2-passing",
				Status:    api.HealthPassing,
				ServiceID: "db2",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	// Register terminating-gateway config entry, linking it to db, api, and redis (dne)
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "db",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	var out structs.IndexedServiceDump
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "terminating-gateway",
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out))

	dump := out.Dump

	// Reset raft indices to facilitate assertion
	for i := 0; i < len(dump); i++ {
		svc := dump[i]
		if svc.Node != nil {
			svc.Node.RaftIndex = structs.RaftIndex{}
		}
		if svc.Service != nil {
			svc.Service.RaftIndex = structs.RaftIndex{}
		}
		if len(svc.Checks) > 0 && svc.Checks[0] != nil {
			svc.Checks[0].RaftIndex = structs.RaftIndex{}
		}
		if svc.GatewayService != nil {
			svc.GatewayService.RaftIndex = structs.RaftIndex{}
		}
	}

	expect := structs.ServiceDump{
		{
			Node: &structs.Node{
				Node:       "baz",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				Address:    "127.0.0.3",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "baz",
					CheckID:        types.CheckID("db2-passing"),
					Name:           "db2-passing",
					Status:         "passing",
					ServiceID:      "db2",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "terminating-gateway",
				ServiceKind: structs.GatewayServiceKindService,
			},
		},
		{
			Node: &structs.Node{
				Node:       "bar",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				Address:    "127.0.0.2",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "bar",
					CheckID:        types.CheckID("db-warning"),
					Name:           "db-warning",
					Status:         "warning",
					ServiceID:      "db",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "terminating-gateway",
				ServiceKind: structs.GatewayServiceKindService,
			},
		},
		{
			// Only GatewayService should be returned when linked service isn't registered
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				Service:     structs.NewServiceName("redis", nil),
				GatewayKind: "terminating-gateway",
				CAFile:      "/etc/certs/ca.pem",
				CertFile:    "/etc/certs/cert.pem",
				KeyFile:     "/etc/certs/key.pem",
			},
		},
	}
	assert.ElementsMatch(t, expect, dump)
}

func TestInternal_GatewayServiceDump_Terminating_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Create the ACL.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
  	service "db" { policy = "read" }
	service "terminating-gateway" { policy = "read" }
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	// Register gateway and two service instances that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "api",
				Service: "api",
			},
			Check: &structs.HealthCheck{
				Name:      "api-passing",
				Status:    api.HealthPassing,
				ServiceID: "api",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	// Register terminating-gateway config entry, linking it to db and api
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{Name: "db"},
				{Name: "api"},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	var out structs.IndexedServiceDump

	// Not passing a token with service:read on Gateway leads to PermissionDenied
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "terminating-gateway",
	}
	err = msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out)
	require.Error(t, err, acl.ErrPermissionDenied)

	// Passing a token without service:read on api leads to it getting filtered out
	req = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "terminating-gateway",
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out))

	nodes := out.Dump
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node.Node, "bar")
	require.Equal(t, nodes[0].Service.Service, "db")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthWarning)
	require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
}

func TestInternal_GatewayServiceDump_Ingress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register gateway and service instance that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "ingress-gateway",
				Service: "ingress-gateway",
				Kind:    structs.ServiceKindIngressGateway,
				Port:    8443,
			},
			Check: &structs.HealthCheck{
				Name:      "ingress connect",
				Status:    api.HealthPassing,
				ServiceID: "ingress-gateway",
			},
		}
		var regOutput struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &regOutput))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db2-passing",
				Status:    api.HealthPassing,
				ServiceID: "db2",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &regOutput))

		// Register ingress-gateway config entry, linking it to db and redis (dne)
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "db",
						},
					},
				},
				{
					Port:     8080,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "web",
						},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	var out structs.IndexedServiceDump
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "ingress-gateway",
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out))

	dump := out.Dump

	// Reset raft indices to facilitate assertion
	for i := 0; i < len(dump); i++ {
		svc := dump[i]
		if svc.Node != nil {
			svc.Node.RaftIndex = structs.RaftIndex{}
		}
		if svc.Service != nil {
			svc.Service.RaftIndex = structs.RaftIndex{}
		}
		if len(svc.Checks) > 0 && svc.Checks[0] != nil {
			svc.Checks[0].RaftIndex = structs.RaftIndex{}
		}
		if svc.GatewayService != nil {
			svc.GatewayService.RaftIndex = structs.RaftIndex{}
		}
	}

	expect := structs.ServiceDump{
		{
			Node: &structs.Node{
				Node:       "bar",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				Address:    "127.0.0.2",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Kind:    "",
				ID:      "db",
				Service: "db",
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "bar",
					CheckID:        types.CheckID("db-warning"),
					Name:           "db-warning",
					Status:         "warning",
					ServiceID:      "db",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("ingress-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "ingress-gateway",
				Port:        8888,
				Protocol:    "tcp",
			},
		},
		{
			Node: &structs.Node{
				Node:       "baz",
				Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				Address:    "127.0.0.3",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "baz",
					CheckID:        types.CheckID("db2-passing"),
					Name:           "db2-passing",
					Status:         "passing",
					ServiceID:      "db2",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("ingress-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "ingress-gateway",
				Port:        8888,
				Protocol:    "tcp",
			},
		},
		{
			// Only GatewayService should be returned when upstream isn't registered
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("ingress-gateway", nil),
				Service:     structs.NewServiceName("web", nil),
				GatewayKind: "ingress-gateway",
				Port:        8080,
				Protocol:    "tcp",
			},
		},
	}
	assert.ElementsMatch(t, expect, dump)
}

func TestInternal_GatewayServiceDump_Ingress_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Create the ACL.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
  	service "db" { policy = "read" }
	service "ingress-gateway" { policy = "read" }
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	// Register gateway and two service instances that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "ingress-gateway",
				Service: "ingress-gateway",
				Kind:    structs.ServiceKindIngressGateway,
			},
			Check: &structs.HealthCheck{
				Name:      "ingress connect",
				Status:    api.HealthPassing,
				ServiceID: "ingress-gateway",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "api",
				Service: "api",
			},
			Check: &structs.HealthCheck{
				Name:      "api-passing",
				Status:    api.HealthPassing,
				ServiceID: "api",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	// Register ingress-gateway config entry, linking it to db and api
	{
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "db",
						},
					},
				},
				{
					Port:     8080,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "web",
						},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	var out structs.IndexedServiceDump

	// Not passing a token with service:read on Gateway leads to PermissionDenied
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "ingress-gateway",
	}
	err = msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out)
	require.Error(t, err, acl.ErrPermissionDenied)

	// Passing a token without service:read on api leads to it getting filtered out
	req = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "ingress-gateway",
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServiceDump", &req, &out))

	nodes := out.Dump
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node.Node, "bar")
	require.Equal(t, nodes[0].Service.Service, "db")
	require.Equal(t, nodes[0].Checks[0].Status, api.HealthWarning)
}

func TestInternal_ServiceDump_Peering(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(config *Config) {
		config.PeeringTestAllowPeerRegistrations = true
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// prep the cluster with some data we can use in our filters
	registerTestCatalogEntries(t, codec)

	doRequest := func(t *testing.T, filter string) structs.IndexedNodesWithGateways {
		t.Helper()
		args := structs.DCSpecificRequest{
			QueryOptions: structs.QueryOptions{Filter: filter},
		}

		var out structs.IndexedNodesWithGateways
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out))

		return out
	}

	t.Run("No peerings", func(t *testing.T) {
		nodes := doRequest(t, "")
		// redis (3), web (3), critical (1), warning (1) and consul (1)
		require.Len(t, nodes.Nodes, 9)
		require.Len(t, nodes.ImportedNodes, 0)
	})

	addPeerService(t, codec)

	err := s1.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			Name: "peer1",
		},
	})
	require.NoError(t, err)

	t.Run("peerings", func(t *testing.T) {
		nodes := doRequest(t, "")
		// redis (3), web (3), critical (1), warning (1) and consul (1)
		require.Len(t, nodes.Nodes, 9)
		// service (1)
		require.Len(t, nodes.ImportedNodes, 1)
	})

	t.Run("peerings w filter", func(t *testing.T) {
		nodes := doRequest(t, "Node.PeerName == foo")
		require.Len(t, nodes.Nodes, 0)
		require.Len(t, nodes.ImportedNodes, 0)

		nodes2 := doRequest(t, "Node.PeerName == peer1")
		require.Len(t, nodes2.Nodes, 0)
		require.Len(t, nodes2.ImportedNodes, 1)
	})
}

func addPeerService(t *testing.T, codec rpc.ClientCodec) {
	// prep the cluster with some data we can use in our filters
	registrations := map[string]*structs.RegisterRequest{
		"Peer node foo with peer service": {
			Datacenter: "dc1",
			Node:       "foo",
			ID:         types.NodeID("e0155642-135d-4739-9853-a1ee6c9f945b"),
			Address:    "127.0.0.2",
			PeerName:   "peer1",
			Service: &structs.NodeService{
				Kind:     structs.ServiceKindTypical,
				ID:       "serviceID",
				Service:  "service",
				Port:     1235,
				Address:  "198.18.1.2",
				PeerName: "peer1",
			},
		},
	}

	registerTestCatalogEntriesMap(t, codec, registrations)
}

func TestInternal_GatewayIntentions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register terminating gateway and config entry linking it to postgres + redis
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
		}
		var regOutput struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &regOutput))

		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "postgres",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// create some symmetric intentions to ensure we are only matching on destination
	{
		for _, v := range []string{"*", "mysql", "redis", "postgres"} {
			req := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = "api"
			req.Intention.DestinationName = v

			var reply string
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))

			req = structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = v
			req.Intention.DestinationName = "api"
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))
		}
	}

	// Request intentions matching the gateway named "terminating-gateway"
	req := structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Partition: acl.DefaultPartitionName,
					Name:      "terminating-gateway",
				},
			},
		},
	}
	var reply structs.IndexedIntentions
	assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayIntentions", &req, &reply))
	assert.Len(t, reply.Intentions, 3)

	// Only intentions with linked services as a destination should be returned, and wildcard matches should be deduped
	expected := []string{"postgres", "*", "redis"}
	actual := []string{
		reply.Intentions[0].DestinationName,
		reply.Intentions[1].DestinationName,
		reply.Intentions[2].DestinationName,
	}
	assert.ElementsMatch(t, expected, actual)
}

func TestInternal_GatewayIntentions_aclDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServerWithConfig(t, testServerACLConfig)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken(TestDefaultInitialManagementToken))

	// Register terminating gateway and config entry linking it to postgres + redis
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var regOutput struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &regOutput))

		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "postgres",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	// create some symmetric intentions to ensure we are only matching on destination
	{
		for _, v := range []string{"*", "mysql", "redis", "postgres"} {
			req := structs.IntentionRequest{
				Datacenter:   "dc1",
				Op:           structs.IntentionOpCreate,
				Intention:    structs.TestIntention(t),
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			req.Intention.SourceName = "api"
			req.Intention.DestinationName = v

			var reply string
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))

			req = structs.IntentionRequest{
				Datacenter:   "dc1",
				Op:           structs.IntentionOpCreate,
				Intention:    structs.TestIntention(t),
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			req.Intention.SourceName = v
			req.Intention.DestinationName = "api"
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))
		}
	}

	userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `
service_prefix "redis" { policy = "read" }
service_prefix "terminating-gateway" { policy = "read" }
`)
	require.NoError(t, err)

	// Request intentions matching the gateway named "terminating-gateway"
	req := structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Partition: acl.DefaultPartitionName,
					Name:      "terminating-gateway",
				},
			},
		},
		QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
	}
	var reply structs.IndexedIntentions
	assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.GatewayIntentions", &req, &reply))
	assert.Len(t, reply.Intentions, 2)

	// Only intentions for redis should be returned, due to ACLs
	expected := []string{"*", "redis"}
	actual := []string{
		reply.Intentions[0].DestinationName,
		reply.Intentions[1].DestinationName,
	}
	assert.ElementsMatch(t, expected, actual)
}

func TestInternal_ServiceTopology(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// wildcard deny intention
	// ingress-gateway on node edge - upstream: api
	// ingress -> api gateway config entry (but no intention)

	// api and api-proxy on node foo - transparent proxy
	// api -> web exact intention

	// web and web-proxy on node bar - upstream: redis
	// web and web-proxy on node baz - transparent proxy
	// web -> redis exact intention

	// redis and redis-proxy on node zip

	registerTestTopologyEntries(t, codec, "")

	var (
		ingress = structs.NewServiceName("ingress", structs.DefaultEnterpriseMetaInDefaultPartition())
		api     = structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition())
		web     = structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
		redis   = structs.NewServiceName("redis", structs.DefaultEnterpriseMetaInDefaultPartition())
	)

	t.Run("ingress", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "ingress",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)
			require.False(r, out.QueryMeta.ResultsFilteredByACLs)
			require.Equal(r, "http", out.ServiceTopology.MetricsProtocol)

			// foo/api, foo/api-proxy
			require.Len(r, out.ServiceTopology.Upstreams, 2)
			require.Len(r, out.ServiceTopology.Downstreams, 0)

			expectUp := map[string]structs.IntentionDecisionSummary{
				api.String(): {
					DefaultAllow:   true,
					Allowed:        false,
					HasPermissions: false,
					ExternalSource: "nomad",

					// From wildcard deny
					HasExact: false,
				},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)

			expectUpstreamSources := map[string]string{
				api.String(): structs.TopologySourceRegistration,
			}
			require.Equal(r, expectUpstreamSources, out.ServiceTopology.UpstreamSources)
			require.Empty(r, out.ServiceTopology.DownstreamSources)

			// The ingress gateway has an explicit upstream
			require.False(r, out.ServiceTopology.TransparentProxy)
		})
	})

	t.Run("api", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "api",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)
			require.False(r, out.QueryMeta.ResultsFilteredByACLs)
			require.Equal(r, "http", out.ServiceTopology.MetricsProtocol)

			// edge/ingress
			require.Len(r, out.ServiceTopology.Downstreams, 1)

			expectDown := map[string]structs.IntentionDecisionSummary{
				ingress.String(): {
					DefaultAllow:   true,
					Allowed:        false,
					HasPermissions: false,
					ExternalSource: "nomad",

					// From wildcard deny
					HasExact: false,
				},
			}
			require.Equal(r, expectDown, out.ServiceTopology.DownstreamDecisions)

			expectDownstreamSources := map[string]string{
				ingress.String(): structs.TopologySourceRegistration,
			}
			require.Equal(r, expectDownstreamSources, out.ServiceTopology.DownstreamSources)

			// bar/web, bar/web-proxy, baz/web, baz/web-proxy
			require.Len(r, out.ServiceTopology.Upstreams, 4)

			expectUp := map[string]structs.IntentionDecisionSummary{
				web.String(): {
					DefaultAllow:   true,
					Allowed:        true,
					HasPermissions: false,
					HasExact:       true,
				},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)

			expectUpstreamSources := map[string]string{
				web.String(): structs.TopologySourceSpecificIntention,
			}
			require.Equal(r, expectUpstreamSources, out.ServiceTopology.UpstreamSources)

			// The only instance of api's proxy is in transparent mode
			require.True(r, out.ServiceTopology.TransparentProxy)
		})
	})

	t.Run("web", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)
			require.False(r, out.QueryMeta.ResultsFilteredByACLs)
			require.Equal(r, "http", out.ServiceTopology.MetricsProtocol)

			// foo/api, foo/api-proxy
			require.Len(r, out.ServiceTopology.Downstreams, 2)

			expectDown := map[string]structs.IntentionDecisionSummary{
				api.String(): {
					DefaultAllow:   true,
					Allowed:        true,
					HasPermissions: false,
					HasExact:       true,
				},
			}
			require.Equal(r, expectDown, out.ServiceTopology.DownstreamDecisions)

			expectDownstreamSources := map[string]string{
				api.String(): structs.TopologySourceSpecificIntention,
			}
			require.Equal(r, expectDownstreamSources, out.ServiceTopology.DownstreamSources)

			// zip/redis, zip/redis-proxy
			require.Len(r, out.ServiceTopology.Upstreams, 2)

			expectUp := map[string]structs.IntentionDecisionSummary{
				redis.String(): {
					DefaultAllow:   true,
					Allowed:        false,
					HasPermissions: true,
					HasExact:       true,
				},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)

			expectUpstreamSources := map[string]string{
				// We prefer from-registration over intention source when there is a mix
				redis.String(): structs.TopologySourceRegistration,
			}
			require.Equal(r, expectUpstreamSources, out.ServiceTopology.UpstreamSources)

			// Not all instances of web are in transparent mode
			require.False(r, out.ServiceTopology.TransparentProxy)
		})
	})

	t.Run("redis", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "redis",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)
			require.False(r, out.QueryMeta.ResultsFilteredByACLs)
			require.Equal(r, "http", out.ServiceTopology.MetricsProtocol)

			require.Len(r, out.ServiceTopology.Upstreams, 0)

			// bar/web, bar/web-proxy, baz/web, baz/web-proxy
			require.Len(r, out.ServiceTopology.Downstreams, 4)

			expectDown := map[string]structs.IntentionDecisionSummary{
				web.String(): {
					DefaultAllow:   true,
					Allowed:        false,
					HasPermissions: true,
					HasExact:       true,
				},
			}
			require.Equal(r, expectDown, out.ServiceTopology.DownstreamDecisions)

			expectDownstreamSources := map[string]string{
				web.String(): structs.TopologySourceRegistration,
			}
			require.Equal(r, expectDownstreamSources, out.ServiceTopology.DownstreamSources)
			require.Empty(r, out.ServiceTopology.UpstreamSources)

			// No proxies are in transparent mode
			require.False(r, out.ServiceTopology.TransparentProxy)
		})
	})
}

func TestInternal_ServiceTopology_RoutingConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// dashboard -> routing-config -> { counting, counting-v2 }
	registerTestRoutingConfigTopologyEntries(t, codec)

	t.Run("dashboard", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "dashboard",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)
			require.False(r, out.QueryMeta.ResultsFilteredByACLs)
			require.Equal(r, "http", out.ServiceTopology.MetricsProtocol)

			require.Empty(r, out.ServiceTopology.Downstreams)
			require.Empty(r, out.ServiceTopology.DownstreamDecisions)
			require.Empty(r, out.ServiceTopology.DownstreamSources)

			// routing-config will not appear as an Upstream service
			// but will be present in UpstreamSources as a k-v pair.
			require.Empty(r, out.ServiceTopology.Upstreams)

			sn := structs.NewServiceName("routing-config", structs.DefaultEnterpriseMetaInDefaultPartition()).String()

			expectUp := map[string]structs.IntentionDecisionSummary{
				sn: {DefaultAllow: true, Allowed: true},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)

			expectUpstreamSources := map[string]string{
				sn: structs.TopologySourceRoutingConfig,
			}
			require.Equal(r, expectUpstreamSources, out.ServiceTopology.UpstreamSources)

			require.False(r, out.ServiceTopology.TransparentProxy)
		})
	})
}

func TestInternal_ServiceTopology_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = TestDefaultInitialManagementToken
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// wildcard deny intention
	// ingress-gateway on node edge - upstream: api
	// ingress -> api gateway config entry (but no intention)

	// api and api-proxy on node foo - transparent proxy
	// api -> web exact intention

	// web and web-proxy on node bar - upstream: redis
	// web and web-proxy on node baz - transparent proxy
	// web -> redis exact intention

	// redis and redis-proxy on node zip
	registerTestTopologyEntries(t, codec, TestDefaultInitialManagementToken)

	// Token grants read to: foo/api, foo/api-proxy, bar/web, baz/web
	userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `
node_prefix "" { policy = "read" }
service_prefix "api" { policy = "read" }
service "web" { policy = "read" }
`)
	require.NoError(t, err)

	t.Run("api can't read web", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "api",
			QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
		}
		var out structs.IndexedServiceTopology
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))

		require.True(t, out.FilteredByACLs)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs)
		require.Equal(t, "http", out.ServiceTopology.MetricsProtocol)

		// The web-proxy upstream gets filtered out from both bar and baz
		require.Len(t, out.ServiceTopology.Upstreams, 2)
		require.Equal(t, "web", out.ServiceTopology.Upstreams[0].Service.Service)
		require.Equal(t, "web", out.ServiceTopology.Upstreams[1].Service.Service)

		require.Len(t, out.ServiceTopology.Downstreams, 0)
	})

	t.Run("web can't read redis", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "web",
			QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
		}
		var out structs.IndexedServiceTopology
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))

		require.True(t, out.FilteredByACLs)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs)
		require.Equal(t, "http", out.ServiceTopology.MetricsProtocol)

		// The redis upstream gets filtered out but the api and proxy downstream are returned
		require.Len(t, out.ServiceTopology.Upstreams, 0)
		require.Len(t, out.ServiceTopology.Downstreams, 2)
	})

	t.Run("redis can't read self", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "redis",
			QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
		}
		var out structs.IndexedServiceTopology
		err := msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out)

		// Can't read self, fails fast
		require.True(t, acl.IsErrPermissionDenied(err))
	})
}

func TestInternal_IntentionUpstreams(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Services:
	// api and api-proxy on node foo
	// web and web-proxy on node foo
	//
	// Intentions
	// * -> * (deny) intention
	// web -> api (allow)
	registerIntentionUpstreamEntries(t, codec, "")

	t.Run("web", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web",
			}
			var out structs.IndexedServiceList
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.IntentionUpstreams", &args, &out))

			// foo/api
			require.Len(r, out.Services, 1)

			expectUp := structs.ServiceList{
				structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition()),
			}
			require.Equal(r, expectUp, out.Services)
		})
	})
}

func TestInternal_IntentionUpstreamsDestination(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Services:
	// api and api-proxy on node foo
	// web and web-proxy on node foo
	//
	// Intentions
	// * -> * (deny) intention
	// web -> api (allow)
	registerIntentionUpstreamEntries(t, codec, "")

	t.Run("api.example.com", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "web",
			}
			var out structs.IndexedServiceList
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.IntentionUpstreamsDestination", &args, &out))

			// foo/api
			require.Len(r, out.Services, 1)

			expectUp := structs.ServiceList{
				structs.NewServiceName("api.example.com", structs.DefaultEnterpriseMetaInDefaultPartition()),
			}
			require.Equal(r, expectUp, out.Services)
		})
	})
}

func TestInternal_IntentionUpstreams_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)

	{ // ensure it's default deny to start
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
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
		}, &out))
		require.True(t, out)
	}

	run := func(t *testing.T, dataPrefix string, expectServices int) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := &structs.ServiceSpecificRequest{
					Datacenter:  "dc1",
					ServiceName: "web",
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.IndexedServiceList
				errCh := channelCallRPC(s1, "Internal.IntentionUpstreams", args, &out, func() error {
					if len(out.Services) != expectServices {
						return fmt.Errorf("expected %d services got %d", expectServices, len(out.Services))
					}
					return nil
				})
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out string
				return channelCallRPC(s1, "Intention.Apply", &structs.IntentionRequest{
					Datacenter: "dc1",
					Op:         structs.IntentionOpCreate,
					Intention: &structs.Intention{
						SourceName:      fmt.Sprintf(dataPrefix+"-src-%d", i),
						DestinationName: fmt.Sprintf(dataPrefix+"-dst-%d", i),
						Action:          structs.IntentionActionAllow,
					},
				}, &out, nil)
			},
		)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other", 0)
	})

	// Services:
	// api and api-proxy on node foo
	// web and web-proxy on node foo
	//
	// Intentions
	// * -> * (deny) intention
	// web -> api (allow)
	registerIntentionUpstreamEntries(t, codec, "")

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other", 1)
	})
}

func TestInternal_IntentionUpstreams_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = TestDefaultInitialManagementToken
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// Services:
	// api and api-proxy on node foo
	// web and web-proxy on node foo
	//
	// Intentions
	// * -> * (deny) intention
	// web -> api (allow)
	registerIntentionUpstreamEntries(t, codec, TestDefaultInitialManagementToken)

	t.Run("valid token", func(t *testing.T) {
		// Token grants read to read api service
		userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `
service_prefix "api" { policy = "read" }
`)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web",
				QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
			}
			var out structs.IndexedServiceList
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.IntentionUpstreams", &args, &out))

			// foo/api
			require.Len(r, out.Services, 1)

			expectUp := structs.ServiceList{
				structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition()),
			}
			require.Equal(r, expectUp, out.Services)
		})
	})

	t.Run("invalid token filters results", func(t *testing.T) {
		// Token grants read to read an unrelated service, mongo
		userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `
service_prefix "mongo" { policy = "read" }
`)
		require.NoError(t, err)

		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:   "dc1",
				ServiceName:  "web",
				QueryOptions: structs.QueryOptions{Token: userToken.SecretID},
			}
			var out structs.IndexedServiceList
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.IntentionUpstreams", &args, &out))

			// Token can't read api service
			require.Empty(r, out.Services)
		})
	})
}

func TestInternal_CatalogOverview(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.MetricsReportingInterval = 100 * time.Millisecond
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	retry.Run(t, func(r *retry.R) {
		var out structs.CatalogSummary
		if err := msgpackrpc.CallWithCodec(codec, "Internal.CatalogOverview", &arg, &out); err != nil {
			r.Fatalf("err: %v", err)
		}

		expected := structs.CatalogSummary{
			Nodes: []structs.HealthSummary{
				{
					Total:          1,
					Passing:        1,
					EnterpriseMeta: *structs.NodeEnterpriseMetaInDefaultPartition(),
				},
			},
			Services: []structs.HealthSummary{
				{
					Name:           "consul",
					Total:          1,
					Passing:        1,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			Checks: []structs.HealthSummary{
				{
					Name:           "Serf Health Status",
					Total:          1,
					Passing:        1,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		}
		require.Equal(r, expected, out)
	})
}

func TestInternal_CatalogOverview_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = TestDefaultInitialManagementToken
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	codec := rpcClient(t, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.CatalogSummary
	err := msgpackrpc.CallWithCodec(codec, "Internal.CatalogOverview", &arg, &out)
	require.True(t, acl.IsErrPermissionDenied(err))

	opReadToken, err := upsertTestTokenWithPolicyRules(
		codec, TestDefaultInitialManagementToken, "dc1", `operator = "read"`)
	require.NoError(t, err)

	arg.Token = opReadToken.SecretID
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.CatalogOverview", &arg, &out))
}

func TestInternal_PeeredUpstreams(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	orig := virtualIPVersionCheckInterval
	virtualIPVersionCheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { virtualIPVersionCheckInterval = orig })

	t.Parallel()
	_, s1 := testServerWithConfig(t)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Services
	//   api        local
	//   web        peer: peer-a
	//   web-proxy  peer: peer-a
	//   web        peer: peer-b
	//   web-proxy  peer: peer-b
	registerLocalAndRemoteServicesVIPEnabled(t, s1.fsm.State())

	codec := rpcClient(t, s1)

	args := structs.PartitionSpecificRequest{
		Datacenter:     "dc1",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	var out structs.IndexedPeeredServiceList
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.PeeredUpstreams", &args, &out))

	require.Len(t, out.Services, 2)
	expect := []structs.PeeredServiceName{
		{Peer: "peer-a", ServiceName: structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())},
		{Peer: "peer-b", ServiceName: structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())},
	}
	require.Equal(t, expect, out.Services)
}

func TestInternal_ServiceGatewayService_Terminating(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	db := structs.NodeService{
		ID:      "db2",
		Service: "db",
	}

	redis := structs.NodeService{
		ID:      "redis",
		Service: "redis",
	}

	// Register gateway and two service instances that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "10.1.2.2",
			Service: &structs.NodeService{
				ID:      "terminating-gateway-01",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
				Address: "198.18.1.3",
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway-01",
			},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service:    &db,
			Check: &structs.HealthCheck{
				Name:      "db2-passing",
				Status:    api.HealthPassing,
				ServiceID: "db2",
			},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	// Register terminating-gateway config entry, linking it to db and redis (dne)
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "db",
				},
				{
					Name:     "redis",
					CAFile:   "/etc/certs/ca.pem",
					CertFile: "/etc/certs/cert.pem",
					KeyFile:  "/etc/certs/key.pem",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	var out structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceKind: structs.ServiceKindTerminatingGateway,
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceGateways", &req, &out))

	for _, n := range out.Nodes {
		n.Node.RaftIndex = structs.RaftIndex{}
		n.Service.RaftIndex = structs.RaftIndex{}
		for _, m := range n.Checks {
			m.RaftIndex = structs.RaftIndex{}
		}
	}

	expect := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				Node:       "foo",
				RaftIndex:  structs.RaftIndex{},
				Address:    "10.1.2.2",
				Datacenter: "dc1",
				Partition:  acl.DefaultPartitionName,
			},
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				ID:      "terminating-gateway-01",
				Service: "terminating-gateway",
				TaggedAddresses: map[string]structs.ServiceAddress{
					"consul-virtual:" + db.CompoundServiceName().String():    {Address: "240.0.0.1"},
					"consul-virtual:" + redis.CompoundServiceName().String(): {Address: "240.0.0.2"},
				},
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				Tags:           []string{},
				Meta:           map[string]string{},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				RaftIndex:      structs.RaftIndex{},
				Address:        "198.18.1.3",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Name:           "terminating connect",
					Node:           "foo",
					CheckID:        "terminating connect",
					Status:         api.HealthPassing,
					ServiceID:      "terminating-gateway-01",
					ServiceName:    "terminating-gateway",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}

	assert.Equal(t, expect, out.Nodes)
}

func TestInternal_ServiceGatewayService_Terminating_ACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Create the ACL.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
  	service "db" { policy = "read" }
	service "terminating-gateway" { policy = "read" }
	node_prefix "" { policy = "read" }`)
	require.NoError(t, err)

	// Register gateway and two service instances that will be associated with it
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
		{
			arg := structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					ID:      "terminating-gateway2",
					Service: "terminating-gateway2",
					Kind:    structs.ServiceKindTerminatingGateway,
					Port:    444,
				},
				Check: &structs.HealthCheck{
					Name:      "terminating connect",
					Status:    api.HealthPassing,
					ServiceID: "terminating-gateway2",
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}
			var out struct{}
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
		}

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				ID:      "db",
				Service: "db",
			},
			Check: &structs.HealthCheck{
				Name:      "db-warning",
				Status:    api.HealthWarning,
				ServiceID: "db",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

		arg = structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				ID:      "api",
				Service: "api",
			},
			Check: &structs.HealthCheck{
				Name:      "api-passing",
				Status:    api.HealthPassing,
				ServiceID: "api",
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}

	// Register terminating-gateway config entry, linking it to db and api
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{Name: "db"},
				{Name: "api"},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	// Register terminating-gateway config entry, linking it to db and api
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway2",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{Name: "db"},
				{Name: "api"},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:           structs.ConfigEntryUpsert,
			Datacenter:   "dc1",
			Entry:        args,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &out))
		require.True(t, out)
	}

	var out structs.IndexedCheckServiceNodes

	// Not passing a token with service:read on Gateway leads to PermissionDenied
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceKind: structs.ServiceKindTerminatingGateway,
	}
	err = msgpackrpc.CallWithCodec(codec, "Internal.ServiceGateways", &req, &out)
	require.Error(t, err, acl.ErrPermissionDenied)

	// Passing a token without service:read on api leads to it getting filtered out
	req = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "db",
		ServiceKind:  structs.ServiceKindTerminatingGateway,
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceGateways", &req, &out))

	nodes := out.Nodes
	require.Len(t, nodes, 1)
	require.Equal(t, "foo", nodes[0].Node.Node)
	require.Equal(t, structs.ServiceKindTerminatingGateway, nodes[0].Service.Kind)
	require.Equal(t, "terminating-gateway", nodes[0].Service.Service)
	require.Equal(t, "terminating-gateway", nodes[0].Service.ID)
	require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
}

func TestInternal_ServiceGatewayService_Terminating_Destination(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	google := structs.NodeService{
		ID:      "google",
		Service: "google",
	}

	// Register service-default with conflicting destination address
	{
		arg := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name:           "google",
				Destination:    &structs.DestinationConfig{Addresses: []string{"www.google.com"}, Port: 443},
				EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			},
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &arg, &configOutput))
		require.True(t, configOutput)
	}

	// Register terminating-gateway config entry, linking it to google.com
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Kind:    structs.ServiceKindTerminatingGateway,
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "terminating connect",
				Status:    api.HealthPassing,
				ServiceID: "terminating-gateway",
			},
		}
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))
	}
	{
		args := &structs.TerminatingGatewayConfigEntry{
			Name: "terminating-gateway",
			Kind: structs.TerminatingGateway,
			Services: []structs.LinkedService{
				{
					Name: "google",
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	var out structs.IndexedCheckServiceNodes
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "google",
		ServiceKind: structs.ServiceKindTerminatingGateway,
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceGateways", &req, &out))

	nodes := out.Nodes

	for _, n := range nodes {
		n.Node.RaftIndex = structs.RaftIndex{}
		n.Service.RaftIndex = structs.RaftIndex{}
		for _, m := range n.Checks {
			m.RaftIndex = structs.RaftIndex{}
		}
	}

	expect := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				Node:       "foo",
				RaftIndex:  structs.RaftIndex{},
				Address:    "127.0.0.1",
				Datacenter: "dc1",
				Partition:  acl.DefaultPartitionName,
			},
			Service: &structs.NodeService{
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "terminating-gateway",
				Service:        "terminating-gateway",
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				Tags:           []string{},
				Meta:           map[string]string{},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				TaggedAddresses: map[string]structs.ServiceAddress{
					"consul-virtual:" + google.CompoundServiceName().String(): {Address: "240.0.0.1"},
				},
				RaftIndex: structs.RaftIndex{},
				Address:   "",
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Name:           "terminating connect",
					Node:           "foo",
					CheckID:        "terminating connect",
					Status:         api.HealthPassing,
					ServiceID:      "terminating-gateway",
					ServiceName:    "terminating-gateway",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}

	assert.Len(t, nodes, 1)
	assert.Equal(t, expect, nodes)
}

func TestInternal_ExportedPeeredServices_ACLEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServerWithConfig(t, testServerACLConfig)
	codec := rpcClient(t, s)

	require.NoError(t, s.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(),
			Name: "peer-1",
		},
	}))
	require.NoError(t, s.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(),
			Name: "peer-2",
		},
	}))
	require.NoError(t, s.fsm.State().EnsureConfigEntry(1, &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "web",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-1"},
				},
			},
			{
				Name: "db",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-2"},
				},
			},
			{
				Name: "api",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-1"},
				},
			},
		},
	}))

	type testcase struct {
		name      string
		token     string
		expect    map[string]structs.ServiceList
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		var out *structs.IndexedExportedServiceList
		req := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: tc.token},
		}
		err := msgpackrpc.CallWithCodec(codec, "Internal.ExportedPeeredServices", &req, &out)

		if tc.expectErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectErr)
			require.Nil(t, out)
			return
		}

		require.NoError(t, err)

		require.Len(t, out.Services, len(tc.expect))
		for k, v := range tc.expect {
			require.ElementsMatch(t, v, out.Services[k])
		}
	}
	tcs := []testcase{
		{
			name: "can read all",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				`
			service_prefix "" {
				policy = "read"
			}
			`),
			expect: map[string]structs.ServiceList{
				"peer-1": {
					structs.NewServiceName("api", nil),
					structs.NewServiceName("web", nil),
				},
				"peer-2": {
					structs.NewServiceName("db", nil),
				},
			},
		},
		{
			name: "filtered",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				`
			service "web" { policy = "read" }
			service "api" { policy = "read" }
			service "db"  { policy = "deny" }	
			`),
			expect: map[string]structs.ServiceList{
				"peer-1": {
					structs.NewServiceName("api", nil),
					structs.NewServiceName("web", nil),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func tokenWithRules(t *testing.T, codec rpc.ClientCodec, mgmtToken, rules string) string {
	t.Helper()

	var tok *structs.ACLToken
	var err error
	retry.Run(t, func(r *retry.R) {
		tok, err = upsertTestTokenWithPolicyRules(codec, mgmtToken, "dc1", rules)
		require.NoError(r, err)
	})
	return tok.SecretID
}

func TestInternal_PeeredUpstreams_ACLEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServerWithConfig(t, testServerACLConfig)
	codec := rpcClient(t, s)

	type testcase struct {
		name      string
		token     string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		var out *structs.IndexedPeeredServiceList

		req := structs.PartitionSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: tc.token},
		}
		err := msgpackrpc.CallWithCodec(codec, "Internal.PeeredUpstreams", &req, &out)

		if tc.expectErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectErr)
			require.Nil(t, out)
		} else {
			require.NoError(t, err)
		}
	}
	tcs := []testcase{
		{
			name: "can write all",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken, `
			service_prefix "" {
				policy = "write"
			}
			`),
		},
		{
			name:      "can't write",
			token:     tokenWithRules(t, codec, TestDefaultInitialManagementToken, ``),
			expectErr: "lacks permission 'service:write' on \"any service\"",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestInternal_ExportedServicesForPeer_ACLEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	_, s := testServerWithConfig(t, testServerACLConfig)
	codec := rpcClient(t, s)

	require.NoError(t, s.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(),
			Name: "peer-1",
		},
	}))
	require.NoError(t, s.fsm.State().PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(),
			Name: "peer-2",
		},
	}))
	require.NoError(t, s.fsm.State().EnsureConfigEntry(1, &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "web",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-1"},
				},
			},
			{
				Name: "db",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-2"},
				},
			},
			{
				Name: "api",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-1"},
				},
			},
		},
	}))

	type testcase struct {
		name      string
		token     string
		expect    structs.ServiceList
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		var out *structs.IndexedServiceList
		req := structs.ServiceDumpRequest{
			Datacenter:   "dc1",
			PeerName:     "peer-1",
			QueryOptions: structs.QueryOptions{Token: tc.token},
		}
		err := msgpackrpc.CallWithCodec(codec, "Internal.ExportedServicesForPeer", &req, &out)

		if tc.expectErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectErr)
			require.Nil(t, out)
			return
		}

		require.NoError(t, err)
		require.Equal(t, tc.expect, out.Services)
	}
	tcs := []testcase{
		{
			name: "can read all",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				`
			service_prefix "" {
				policy = "read"
			}
			`),
			expect: structs.ServiceList{
				structs.NewServiceName("api", nil),
				structs.NewServiceName("web", nil),
			},
		},
		{
			name: "filtered",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				`
			service "web" { policy = "read" }
			service "api" { policy = "deny" }
			`),
			expect: structs.ServiceList{
				structs.NewServiceName("web", nil),
			},
		},
		{
			name: "no service rules filters all results",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				``),
			expect: structs.ServiceList{},
		},
		{
			name: "no service rules but mesh write shows all results",
			token: tokenWithRules(t, codec, TestDefaultInitialManagementToken,
				`mesh = "write"`),
			expect: structs.ServiceList{
				structs.NewServiceName("api", nil),
				structs.NewServiceName("web", nil),
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func testUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}
