package consul

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternal_NodeInfo(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
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
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedNodeDump
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	if err := msgpackrpc.CallWithCodec(codec, "Internal.NodeInfo", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes := out2.Dump
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
}

func TestInternal_NodeDump(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
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
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg = structs.RegisterRequest{
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
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

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
}

func TestInternal_NodeDump_Filter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.RegisterRequest{
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
			Tags:    []string{"replica"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    api.HealthWarning,
			ServiceID: "db",
		},
	}

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out))

	var out2 structs.IndexedNodeDump
	req := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Filter: "primary in Services.Tags"},
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.NodeDump", &req, &out2))

	nodes := out2.Dump
	require.Len(t, nodes, 1)
	require.Equal(t, "foo", nodes[0].Node)
}

func TestInternal_KeyringOperation(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &opt, &reply); err != nil {
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

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestInternal_NodeDump_FilterACL(t *testing.T) {
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
	if err := msgpackrpc.CallWithCodec(codec, "Health.NodeChecks", &opt, &reply); err != nil {
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

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestInternal_EventFire_Token(t *testing.T) {
	t.Parallel()
	dir, srv := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDownPolicy = "deny"
		c.ACLDefaultPolicy = "deny"
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

func TestInternal_GatewayServiceDump_Terminating(t *testing.T) {
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "baz",
					CheckID:        types.CheckID("db2-passing"),
					Name:           "db2-passing",
					Status:         "passing",
					ServiceID:      "db2",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "terminating-gateway",
			},
		},
		{
			Node: &structs.Node{
				Node:       "bar",
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "bar",
					CheckID:        types.CheckID("db-warning"),
					Name:           "db-warning",
					Status:         "warning",
					ServiceID:      "db",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				},
			},
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceName("terminating-gateway", nil),
				Service:     structs.NewServiceName("db", nil),
				GatewayKind: "terminating-gateway",
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
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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
}

func TestInternal_GatewayServiceDump_Ingress(t *testing.T) {
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "bar",
					CheckID:        types.CheckID("db-warning"),
					Name:           "db-warning",
					Status:         "warning",
					ServiceID:      "db",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			},
			Checks: structs.HealthChecks{
				{
					Node:           "baz",
					CheckID:        types.CheckID("db2-passing"),
					Name:           "db2-passing",
					Status:         "passing",
					ServiceID:      "db2",
					ServiceName:    "db",
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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

func TestInternal_GatewayIntentions(t *testing.T) {
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
	dir1, s1 := testServerWithConfig(t, testServerACLConfig(nil))
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken(TestDefaultMasterToken))

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
			WriteRequest: structs.WriteRequest{Token: TestDefaultMasterToken},
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
			WriteRequest: structs.WriteRequest{Token: TestDefaultMasterToken},
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
				WriteRequest: structs.WriteRequest{Token: TestDefaultMasterToken},
			}
			req.Intention.SourceName = "api"
			req.Intention.DestinationName = v

			var reply string
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))

			req = structs.IntentionRequest{
				Datacenter:   "dc1",
				Op:           structs.IntentionOpCreate,
				Intention:    structs.TestIntention(t),
				WriteRequest: structs.WriteRequest{Token: TestDefaultMasterToken},
			}
			req.Intention.SourceName = v
			req.Intention.DestinationName = "api"
			assert.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &reply))
		}
	}

	userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `
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
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// api and api-proxy on node foo - upstream: web
	// web and web-proxy on node bar - upstream: redis
	// web and web-proxy on node baz - upstream: redis
	// redis and redis-proxy on node zip
	registerTestTopologyEntries(t, codec, "")

	var (
		api   = structs.NewServiceName("api", structs.DefaultEnterpriseMeta())
		web   = structs.NewServiceName("web", structs.DefaultEnterpriseMeta())
		redis = structs.NewServiceName("redis", structs.DefaultEnterpriseMeta())
	)

	t.Run("api", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			args := structs.ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "api",
			}
			var out structs.IndexedServiceTopology
			require.NoError(r, msgpackrpc.CallWithCodec(codec, "Internal.ServiceTopology", &args, &out))
			require.False(r, out.FilteredByACLs)

			// bar/web, bar/web-proxy, baz/web, baz/web-proxy
			require.Len(r, out.ServiceTopology.Upstreams, 4)
			require.Len(r, out.ServiceTopology.Downstreams, 0)

			expectUp := map[string]structs.IntentionDecisionSummary{
				web.String(): {
					Allowed:        false,
					HasPermissions: false,
					ExternalSource: "nomad",
				},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)
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

			// foo/api, foo/api-proxy
			require.Len(r, out.ServiceTopology.Downstreams, 2)

			expectDown := map[string]structs.IntentionDecisionSummary{
				api.String(): {
					Allowed:        false,
					HasPermissions: false,
					ExternalSource: "nomad",
				},
			}
			require.Equal(r, expectDown, out.ServiceTopology.DownstreamDecisions)

			// zip/redis, zip/redis-proxy
			require.Len(r, out.ServiceTopology.Upstreams, 2)

			expectUp := map[string]structs.IntentionDecisionSummary{
				redis.String(): {
					Allowed:        false,
					HasPermissions: true,
				},
			}
			require.Equal(r, expectUp, out.ServiceTopology.UpstreamDecisions)
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

			require.Len(r, out.ServiceTopology.Upstreams, 0)

			// bar/web, bar/web-proxy, baz/web, baz/web-proxy
			require.Len(r, out.ServiceTopology.Downstreams, 4)

			expectDown := map[string]structs.IntentionDecisionSummary{
				web.String(): {
					Allowed:        false,
					HasPermissions: true,
				},
			}
			require.Equal(r, expectDown, out.ServiceTopology.DownstreamDecisions)
		})
	})
}

func TestInternal_ServiceTopology_ACL(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = TestDefaultMasterToken
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	codec := rpcClient(t, s1)
	defer codec.Close()

	// api and api-proxy on node foo - upstream: web
	// web and web-proxy on node bar - upstream: redis
	// web and web-proxy on node baz - upstream: redis
	// redis and redis-proxy on node zip
	registerTestTopologyEntries(t, codec, TestDefaultMasterToken)

	// Token grants read to: foo/api, foo/api-proxy, bar/web, baz/web
	userToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultMasterToken, "dc1", `
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
