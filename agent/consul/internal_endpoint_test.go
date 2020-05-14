package consul

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
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
			Tags:    []string{"master"},
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
	if !lib.StrContains(nodes[0].Services[0].Tags, "master") {
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
			Tags:    []string{"master"},
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
			Tags:    []string{"slave"},
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
			if !lib.StrContains(node.Services[0].Tags, "master") {
				t.Fatalf("Bad: %v", nodes[0])
			}
			if node.Checks[0].Status != api.HealthPassing {
				t.Fatalf("Bad: %v", nodes[0])
			}

		case "bar":
			foundBar = true
			if !lib.StrContains(node.Services[0].Tags, "slave") {
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
			Tags:    []string{"master"},
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
			Tags:    []string{"slave"},
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
		QueryOptions: structs.QueryOptions{Filter: "master in Services.Tags"},
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

	doRequest := func(t *testing.T, filter string) structs.CheckServiceNodes {
		t.Helper()
		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Filter: filter},
		}

		var out structs.IndexedCheckServiceNodes
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.ServiceDump", &args, &out))
		return out.Nodes
	}

	// Run the tests against the test server
	t.Run("No Filter", func(t *testing.T) {
		nodes := doRequest(t, "")
		// redis (3), web (3), critical (1), warning (1) and consul (1)
		require.Len(t, nodes, 9)
	})

	t.Run("Filter Node foo and service version 1", func(t *testing.T) {
		nodes := doRequest(t, "Node.Node == foo and Service.Meta.version == 1")
		require.Len(t, nodes, 1)
		require.Equal(t, "redis", nodes[0].Service.Service)
		require.Equal(t, "redisV1", nodes[0].Service.ID)
	})

	t.Run("Filter service web", func(t *testing.T) {
		nodes := doRequest(t, "Service.Service == web")
		require.Len(t, nodes, 3)
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

		var out structs.IndexedCheckServiceNodes
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

func TestInternal_TerminatingGatewayServices(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	{
		var out struct{}

		// Register a service "api"
		args := structs.TestRegisterRequest(t)
		args.Service.Service = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "db"
		args = structs.TestRegisterRequest(t)
		args.Service.Service = "db"
		args.Check = &structs.HealthCheck{
			Name:      "db",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "redis"
		args = structs.TestRegisterRequest(t)
		args.Service.Service = "redis"
		args.Check = &structs.HealthCheck{
			Name:      "redis",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a gateway
		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "gateway",
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "gateway",
				Status:    api.HealthPassing,
				ServiceID: "gateway",
			},
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		entryArgs := &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "gateway",
				Services: []structs.LinkedService{
					{
						Name:     "api",
						CAFile:   "api/ca.crt",
						CertFile: "api/client.crt",
						KeyFile:  "api/client.key",
						SNI:      "my-domain",
					},
					{
						Name: "db",
					},
					{
						Name:     "*",
						CAFile:   "ca.crt",
						CertFile: "client.crt",
						KeyFile:  "client.key",
						SNI:      "my-alt-domain",
					},
				},
			},
		}
		var entryResp bool
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))
	}

	retry.Run(t, func(r *retry.R) {
		// List should return all three services
		req := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "gateway",
		}
		var resp structs.IndexedGatewayServices
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 3)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceID("api", nil),
				Gateway:     structs.NewServiceID("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
			},
			{
				Service:     structs.NewServiceID("db", nil),
				Gateway:     structs.NewServiceID("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "",
				CertFile:    "",
				KeyFile:     "",
			},
			{
				Service:      structs.NewServiceID("redis", nil),
				Gateway:      structs.NewServiceID("gateway", nil),
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				CAFile:       "ca.crt",
				CertFile:     "client.crt",
				KeyFile:      "client.key",
				SNI:          "my-alt-domain",
				FromWildcard: true,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)
	})
}

func TestInternal_GatewayServices_BothGateways(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	{
		var out struct{}

		// Register a service "api"
		args := structs.TestRegisterRequest(t)
		args.Service.Service = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a terminating gateway
		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "gateway",
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "gateway",
				Status:    api.HealthPassing,
				ServiceID: "gateway",
			},
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		entryArgs := &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "gateway",
				Services: []structs.LinkedService{
					{
						Name: "api",
					},
				},
			},
		}
		var entryResp bool
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))

		// Register a service "db"
		args = structs.TestRegisterRequest(t)
		args.Service.Service = "db"
		args.Check = &structs.HealthCheck{
			Name:      "db",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register an ingress gateway
		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "ingress",
				Port:    444,
			},
			Check: &structs.HealthCheck{
				Name:      "ingress",
				Status:    api.HealthPassing,
				ServiceID: "ingress",
			},
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		entryArgs = &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress",
				Listeners: []structs.IngressListener{
					{
						Port: 8888,
						Services: []structs.IngressService{
							{Name: "db"},
						},
					},
				},
			},
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))
	}

	retry.Run(t, func(r *retry.R) {
		req := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "gateway",
		}
		var resp structs.IndexedGatewayServices
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 1)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceID("api", nil),
				Gateway:     structs.NewServiceID("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)

		req.ServiceName = "ingress"
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 1)

		expect = structs.GatewayServices{
			{
				Service:     structs.NewServiceID("db", nil),
				Gateway:     structs.NewServiceID("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "tcp",
				Port:        8888,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)
	})

	// Test a non-gateway service being requested
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "api",
	}
	var resp structs.IndexedGatewayServices
	err := msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp)
	assert.NoError(t, err)
	assert.Empty(t, resp.Services)
	// Ensure that the index is not zero so that a blocking query still gets the
	// latest GatewayServices index
	assert.NotEqual(t, 0, resp.Index)
}

func TestInternal_GatewayServices_ACLFiltering(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLEnforceVersion8 = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	{
		var out struct{}

		// Register a service "api"
		args := structs.TestRegisterRequest(t)
		args.Service.Service = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		args.Token = "root"
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "db"
		args = structs.TestRegisterRequest(t)
		args.Service.Service = "db"
		args.Check = &structs.HealthCheck{
			Name:      "db",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		args.Token = "root"
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "redis"
		args = structs.TestRegisterRequest(t)
		args.Service.Service = "redis"
		args.Check = &structs.HealthCheck{
			Name:      "redis",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		args.Token = "root"
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a gateway
		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "gateway",
				Port:    443,
			},
			Check: &structs.HealthCheck{
				Name:      "gateway",
				Status:    api.HealthPassing,
				ServiceID: "gateway",
			},
		}
		args.Token = "root"
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		entryArgs := &structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "gateway",
				Services: []structs.LinkedService{
					{
						Name:     "api",
						CAFile:   "api/ca.crt",
						CertFile: "api/client.crt",
						KeyFile:  "api/client.key",
					},
					{
						Name: "db",
					},
					{
						Name: "db_replica",
					},
					{
						Name:     "*",
						CAFile:   "ca.crt",
						CertFile: "client.crt",
						KeyFile:  "client.key",
					},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var entryResp bool
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))
	}

	rules := `
service_prefix "db" {
	policy = "read"
}
`
	svcToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", rules)
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		// List should return an empty list, since we do not have read on the gateway
		req := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "gateway",
			QueryOptions: structs.QueryOptions{Token: svcToken.SecretID},
		}
		var resp structs.IndexedGatewayServices
		err := msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp)
		require.True(r, acl.IsErrPermissionDenied(err))
	})

	rules = `
service "gateway" {
	policy = "read"
}
`
	gwToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", rules)
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		// List should return an empty list, since we do not have read on db
		req := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "gateway",
			QueryOptions: structs.QueryOptions{Token: gwToken.SecretID},
		}
		var resp structs.IndexedGatewayServices
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 0)
	})

	rules = `
service_prefix "db" {
	policy = "read"
}
service "gateway" {
	policy = "read"
}
`
	validToken, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", rules)
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		// List should return db entry since we have read on db and gateway
		req := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "gateway",
			QueryOptions: structs.QueryOptions{Token: validToken.SecretID},
		}
		var resp structs.IndexedGatewayServices
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Internal.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 2)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceID("db", nil),
				Gateway:     structs.NewServiceID("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
			},
			{
				Service:     structs.NewServiceID("db_replica", nil),
				Gateway:     structs.NewServiceID("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)
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
				Gateway:     structs.NewServiceID("terminating-gateway", nil),
				Service:     structs.NewServiceID("db", nil),
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
				Gateway:     structs.NewServiceID("terminating-gateway", nil),
				Service:     structs.NewServiceID("db", nil),
				GatewayKind: "terminating-gateway",
			},
		},
		{
			// Only GatewayService should be returned when linked service isn't registered
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceID("terminating-gateway", nil),
				Service:     structs.NewServiceID("redis", nil),
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
		c.ACLEnforceVersion8 = true
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
				Gateway:     structs.NewServiceID("ingress-gateway", nil),
				Service:     structs.NewServiceID("db", nil),
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
				Gateway:     structs.NewServiceID("ingress-gateway", nil),
				Service:     structs.NewServiceID("db", nil),
				GatewayKind: "ingress-gateway",
				Port:        8888,
				Protocol:    "tcp",
			},
		},
		{
			// Only GatewayService should be returned when upstream isn't registered
			GatewayService: &structs.GatewayService{
				Gateway:     structs.NewServiceID("ingress-gateway", nil),
				Service:     structs.NewServiceID("web", nil),
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
		c.ACLEnforceVersion8 = true
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
