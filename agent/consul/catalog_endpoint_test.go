package consul

import (
	"fmt"
	"net/rpc"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_Register(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
		Check: &structs.HealthCheck{
			CheckID:   types.CheckID("db-check"),
			ServiceID: "db",
		},
	}
	var out struct{}

	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_RegisterService_InvalidAddress(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	for _, addr := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run("addr "+addr, func(t *testing.T) {
			arg := structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					Service: "db",
					Address: addr,
					Port:    8000,
				},
			}
			var out struct{}

			err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
			if err == nil || err.Error() != "Invalid service address" {
				t.Fatalf("got error %v want 'Invalid service address'", err)
			}
		})
	}
}

func TestCatalog_RegisterService_SkipNodeUpdate(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Register a node
	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatal(err)
	}

	// Update it with a blank address, should fail.
	arg.Address = ""
	arg.Service = &structs.NodeService{
		Service: "db",
		Port:    8000,
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err == nil || err.Error() != "Must provide address if SkipNodeUpdate is not set" {
		t.Fatalf("got error %v want 'Must provide address...'", err)
	}

	// Set SkipNodeUpdate, should succeed
	arg.SkipNodeUpdate = true
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCatalog_Register_NodeID(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		ID:         "nope",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
		Check: &structs.HealthCheck{
			CheckID:   types.CheckID("db-check"),
			ServiceID: "db",
		},
	}
	var out struct{}

	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err == nil || !strings.Contains(err.Error(), "Bad node ID") {
		t.Fatalf("err: %v", err)
	}

	arg.ID = types.NodeID("adf4238a-882b-9ddc-4a9d-5b6758e4159e")
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	argR := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	var outR struct{}

	// This should fail since we are writing to the "db" service, which isn't
	// allowed.
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work, though.
	argR.Service.Service = "foo"
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try the special case for the "consul" service that allows it no matter
	// what with pre-version 8 ACL enforcement.
	argR.Service.Service = "consul"
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the exception goes away when we turn on version 8 ACL
	// enforcement.
	s1.config.ACLEnforceVersion8 = true
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Register a db service using the root token.
	argR.Service.Service = "db"
	argR.Service.ID = "my-id"
	argR.Token = "root"
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Prove that we are properly looking up the node services and passing
	// that to the ACL helper. We can vet the helper independently in its
	// own unit test after this. This is trying to register over the db
	// service we created above, which is a check that depends on looking
	// at the existing registration data with that service ID. This is a new
	// check for version 8.
	argR.Service.Service = "foo"
	argR.Service.ID = "my-id"
	argR.Token = id
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ForwardLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Use the follower as the client
	var codec rpc.ClientCodec
	if !s1.IsLeader() {
		codec = codec1
	} else {
		codec = codec2
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ForwardDC(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	arg := structs.RegisterRequest{
		Datacenter: "dc2", // Should forward through s1
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"master"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequestProxy(t)

	// Register
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)
}

// DEPRECATED (ProxyDestination) - remove this whole test case when removing
// ProxyDestination
func TestCatalog_Register_DeprecatedConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequestProxy(t)
	args.Service.ProxyDestination = "legacy"
	args.Service.Proxy = structs.ConnectProxyConfig{}

	// Register
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(args.Service.ProxyDestination, v.ServiceProxy.DestinationServiceName)
}

// Test an invalid ConnectProxy. We don't need to exhaustively test because
// this is all tested in structs on the Validate method.
func TestCatalog_Register_ConnectProxy_invalid(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequestProxy(t)
	args.Service.Proxy.DestinationServiceName = ""

	// Register
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	assert.NotNil(err)
	assert.Contains(err.Error(), "DestinationServiceName")
}

// Test that write is required for the proxy destination to register a proxy.
func TestCatalog_Register_ConnectProxy_ACLProxyDestination(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var token string
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &token))

	// Register should fail because we don't have permission on the destination
	args := structs.TestRegisterRequestProxy(t)
	args.Service.Service = "foo"
	args.Service.Proxy.DestinationServiceName = "bar"
	args.WriteRequest.Token = token
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	assert.True(acl.IsErrPermissionDenied(err))

	// Register should fail with the right destination but wrong name
	args = structs.TestRegisterRequestProxy(t)
	args.Service.Service = "bar"
	args.Service.Proxy.DestinationServiceName = "foo"
	args.WriteRequest.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	assert.True(acl.IsErrPermissionDenied(err))

	// Register should work with the right destination
	args = structs.TestRegisterRequestProxy(t)
	args.Service.Service = "foo"
	args.Service.Proxy.DestinationServiceName = "foo"
	args.WriteRequest.Token = token
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
}

func TestCatalog_Register_ConnectNative(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true

	// Register
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(structs.ServiceKindTypical, v.ServiceKind)
	assert.True(v.ServiceConnect.Native)
}

func TestCatalog_Deregister(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out struct{}

	err := msgpackrpc.CallWithCodec(codec, "Catalog.Deregister", &arg, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Deregister", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Deregister_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
node "node" {
	policy = "write"
}

service "service" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a node, node check, service, and service check.
	argR := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "node",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			Port:    8000,
		},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{
				Node:    "node",
				CheckID: "node-check",
			},
			&structs.HealthCheck{
				Node:      "node",
				CheckID:   "service-check",
				ServiceID: "service",
			},
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	var outR struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR); err != nil {
		t.Fatalf("err: %v", err)
	}

	// First pass with version 8 ACL enforcement disabled, we should be able
	// to deregister everything even without a token.
	var err error
	var out struct{}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "service-check"}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "node-check"}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			ServiceID:  "service"}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node"}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Turn on version 8 ACL enforcement and put the catalog entry back.
	s1.config.ACLEnforceVersion8 = true
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &argR, &outR); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Second pass with version 8 ACL enforcement enabled, these should all
	// get rejected.
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "service-check"}, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "node-check"}, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			ServiceID:  "service"}, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node"}, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Third pass these should all go through with the token set.
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "service-check",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "node-check",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			ServiceID:  "service",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try a few error cases.
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			ServiceID:  "nope",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err == nil || !strings.Contains(err.Error(), "Unknown service") {
		t.Fatalf("err: %v", err)
	}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Deregister",
		&structs.DeregisterRequest{
			Datacenter: "dc1",
			Node:       "node",
			CheckID:    "nope",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err == nil || !strings.Contains(err.Error(), "Unknown check") {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_ListDatacenters(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var out []string
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The DCs should come out sorted by default.
	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != "dc1" {
		t.Fatalf("bad: %v", out)
	}
	if out[1] != "dc2" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListDatacenters_DistanceSort(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDC(t, "acdc")
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)
	joinWAN(t, s3, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	var out []string
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// It's super hard to force the Serfs into a known configuration of
	// coordinates, so the best we can do is make sure that the sorting
	// function is getting called (it's tested extensively in rtt_test.go).
	// Since this is relative to dc1, it will be listed first (proving we
	// went into the sort fn).
	if len(out) != 3 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != "dc1" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListNodes(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedNodes
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	retry.Run(t, func(r *retry.R) {
		msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		if got, want := len(out.Nodes), 2; got != want {
			r.Fatalf("got %d nodes want %d", got, want)
		}
	})

	// Server node is auto added from Serf
	if out.Nodes[1].Node != s1.config.NodeName {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListNodes_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a new node with the right meta k/v pair
	node := &structs.Node{Node: "foo", Address: "127.0.0.1", Meta: map[string]string{"somekey": "somevalue"}}
	if err := s1.fsm.State().EnsureNode(1, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Filter by a specific meta k/v pair
	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
		NodeMetaFilters: map[string]string{
			"somekey": "somevalue",
		},
	}
	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		if got, want := len(out.Nodes), 1; got != want {
			r.Fatalf("got %d nodes want %d", got, want)
		}
	})

	// Verify that only the correct node was returned
	if out.Nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
	if v, ok := out.Nodes[0].Meta["somekey"]; !ok || v != "somevalue" {
		t.Fatalf("bad: %v", out)
	}

	// Now filter on a nonexistent meta k/v pair
	args = structs.DCSpecificRequest{
		Datacenter: "dc1",
		NodeMetaFilters: map[string]string{
			"somekey": "invalid",
		},
	}
	out = structs.IndexedNodes{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Should get an empty list of nodes back
	retry.Run(t, func(r *retry.R) {
		msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		if len(out.Nodes) != 0 {
			r.Fatal(nil)
		}
	})
}

func TestCatalog_ListNodes_StaleRead(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join
	joinLAN(t, s2, s1)

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	testrpc.WaitForTestAgent(t, s2.RPC, "dc1")

	// Use the follower as the client
	var codec rpc.ClientCodec
	if !s1.IsLeader() {
		codec = codec1

		// Inject fake data on the follower!
		if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
			t.Fatalf("err: %v", err)
		}
	} else {
		codec = codec2

		// Inject fake data on the follower!
		if err := s2.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	args := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{AllowStale: true},
	}
	var out structs.IndexedNodes
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	found := false
	for _, n := range out.Nodes {
		if n.Node == "foo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("failed to find foo in %#v", out.Nodes)
	}

	if out.QueryMeta.LastContact == 0 {
		t.Fatalf("should have a last contact time")
	}
	if !out.QueryMeta.KnownLeader {
		t.Fatalf("should have known leader")
	}
}

func TestCatalog_ListNodes_ConsistentRead_Fail(t *testing.T) {
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

	// Try to join and wait for all servers to get promoted to voters.
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s2)
	servers := []*Server{s1, s2, s3}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Use the leader as the client, kill the followers.
	var codec rpc.ClientCodec
	for _, s := range servers {
		if s.IsLeader() {
			codec = rpcClient(t, s)
			defer codec.Close()
		} else {
			s.Shutdown()
		}
	}
	if codec == nil {
		t.Fatalf("no leader")
	}

	args := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{RequireConsistent: true},
	}
	var out structs.IndexedNodes
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
	if err == nil || !strings.HasPrefix(err.Error(), "leadership lost") {
		t.Fatalf("err: %v", err)
	}
	if out.QueryMeta.LastContact != 0 {
		t.Fatalf("should not have a last contact time")
	}
	if out.QueryMeta.KnownLeader {
		t.Fatalf("should have no known leader")
	}
}

func TestCatalog_ListNodes_ConsistentRead(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec1 := rpcClient(t, s1)
	defer codec1.Close()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	// Try to join
	joinLAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Use the leader as the client, kill the follower
	var codec rpc.ClientCodec
	if s1.IsLeader() {
		codec = codec1
	} else {
		codec = codec2
	}

	args := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{RequireConsistent: true},
	}
	var out structs.IndexedNodes
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.QueryMeta.LastContact != 0 {
		t.Fatalf("should not have a last contact time")
	}
	if !out.QueryMeta.KnownLeader {
		t.Fatalf("should have known leader")
	}
}

func TestCatalog_ListNodes_DistanceSort(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "aaa", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(2, &structs.Node{Node: "foo", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(3, &structs.Node{Node: "bar", Address: "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureNode(4, &structs.Node{Node: "baz", Address: "127.0.0.4"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Set all but one of the nodes to known coordinates.
	updates := structs.Coordinates{
		{Node: "foo", Coord: lib.GenerateCoordinate(2 * time.Millisecond)},
		{Node: "bar", Coord: lib.GenerateCoordinate(5 * time.Millisecond)},
		{Node: "baz", Coord: lib.GenerateCoordinate(1 * time.Millisecond)},
	}
	if err := s1.fsm.State().CoordinateBatchUpdate(5, updates); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query with no given source node, should get the natural order from
	// the index.
	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		if got, want := len(out.Nodes), 5; got != want {
			r.Fatalf("got %d nodes want %d", got, want)
		}
	})

	if out.Nodes[0].Node != "aaa" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[2].Node != "baz" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[3].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[4].Node != s1.config.NodeName {
		t.Fatalf("bad: %v", out)
	}

	// Query relative to foo, note that there's no known coordinate for the
	// default-added Serf node nor "aaa" so they will go at the end.
	args = structs.DCSpecificRequest{
		Datacenter: "dc1",
		Source:     structs.QuerySource{Datacenter: "dc1", Node: "foo"},
	}
	retry.Run(t, func(r *retry.R) {
		msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		if got, want := len(out.Nodes), 5; got != want {
			r.Fatalf("got %d nodes want %d", got, want)
		}
	})

	if out.Nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[1].Node != "baz" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[2].Node != "bar" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[3].Node != "aaa" {
		t.Fatalf("bad: %v", out)
	}
	if out.Nodes[4].Node != s1.config.NodeName {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListNodes_ACLFilter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// We scope the reply in each of these since msgpack won't clear out an
	// existing slice if the incoming one is nil, so it's best to start
	// clean each time.

	// Prior to version 8, the node policy should be ignored.
	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	{
		reply := structs.IndexedNodes{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply.Nodes)
		}
	}

	// Now turn on version 8 enforcement and try again.
	s1.config.ACLEnforceVersion8 = true
	{
		reply := structs.IndexedNodes{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 {
			t.Fatalf("bad: %v", reply.Nodes)
		}
	}

	// Create an ACL that can read the node.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: fmt.Sprintf(`
node "%s" {
	policy = "read"
}
`, s1.config.NodeName),
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now try with the token and it will go through.
	args.Token = id
	{
		reply := structs.IndexedNodes{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply.Nodes)
		}
	}
}

func Benchmark_Catalog_ListNodes(t *testing.B) {
	dir1, s1 := testServer(nil)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(nil, s1)
	defer codec.Close()

	// Just add a node
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	for i := 0; i < t.N; i++ {
		var out structs.IndexedNodes
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestCatalog_ListServices(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
	for _, s := range out.Services {
		if s == nil {
			t.Fatalf("bad: %v", s)
		}
	}
	// Consul service should auto-register
	if _, ok := out.Services["consul"]; !ok {
		t.Fatalf("bad: %v", out)
	}
	if len(out.Services["db"]) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["db"][0] != "primary" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListServices_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Add a new node with the right meta k/v pair
	node := &structs.Node{Node: "foo", Address: "127.0.0.1", Meta: map[string]string{"somekey": "somevalue"}}
	if err := s1.fsm.State().EnsureNode(1, node); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Add a service to the new node
	if err := s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Filter by a specific meta k/v pair
	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
		NodeMetaFilters: map[string]string{
			"somekey": "somevalue",
		},
	}
	var out structs.IndexedServices
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.Services) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["db"] == nil {
		t.Fatalf("bad: %v", out.Services["db"])
	}
	if len(out.Services["db"]) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["db"][0] != "primary" {
		t.Fatalf("bad: %v", out)
	}

	// Now filter on a nonexistent meta k/v pair
	args = structs.DCSpecificRequest{
		Datacenter: "dc1",
		NodeMetaFilters: map[string]string{
			"somekey": "invalid",
		},
	}
	out = structs.IndexedServices{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should get an empty list of nodes back
	if len(out.Services) != 0 {
		t.Fatalf("bad: %v", out.Services)
	}
}

func TestCatalog_ListServices_Blocking(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Run the query
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup a blocking query
	args.MinQueryIndex = out.Index
	args.MaxQueryTime = time.Second

	// Async cause a change
	idx := out.Index
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := s1.fsm.State().EnsureNode(idx+1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := s1.fsm.State().EnsureService(idx+2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Re-run the query
	out = structs.IndexedServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the indexes
	if out.Index != idx+2 {
		t.Fatalf("bad: %v", out)
	}

	// Should find the service
	if len(out.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListServices_Timeout(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedServices

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Run the query
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup a blocking query
	args.MinQueryIndex = out.Index
	args.MaxQueryTime = 100 * time.Millisecond

	// Re-run the query
	start := time.Now()
	out = structs.IndexedServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the indexes, should not change
	if out.Index != args.MinQueryIndex {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListServices_Stale(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1" // Enable ACLs!
		c.ACLsEnabled = true
		c.Bootstrap = false // Disable bootstrap
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	args.AllowStale = true
	var out structs.IndexedServices

	// Inject a node
	if err := s1.fsm.State().EnsureNode(3, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	codec := rpcClient(t, s2)
	defer codec.Close()

	// Run the query, do not wait for leader, never any contact with leader, should fail
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err == nil || err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("expected %v but got err: %v and %v", structs.ErrNoLeader, err, out)
	}

	// Try to join
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) { r.Check(wantRaft([]*Server{s1, s2})) })
	waitForLeader(s1, s2)

	testrpc.WaitForLeader(t, s2.RPC, "dc1")
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should find the services
	if len(out.Services) != 1 {
		t.Fatalf("bad: %#v", out.Services)
	}

	if !out.KnownLeader {
		t.Fatalf("should have a leader: %v", out)
	}

	s1.Leave()
	s1.Shutdown()

	testrpc.WaitUntilNoLeader(t, s2.RPC, "dc1")

	args.AllowStale = false
	// Since the leader is now down, non-stale query should fail now
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err == nil || err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("expected %v but got err: %v and %v", structs.ErrNoLeader, err, out)
	}

	// With stale, request should still work
	args.AllowStale = true
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should find old service
	if len(out.Services) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	if out.KnownLeader {
		t.Fatalf("should not have a leader anymore: %#v", out)
	}
}

func TestCatalog_ListServiceNodes(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTags: []string{"slave"},
		TagFilter:   false,
	}
	var out structs.IndexedServiceNodes
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out.ServiceNodes) != 1 {
		t.Fatalf("bad: %v", out)
	}

	// Try with a filter
	args.TagFilter = true
	out = structs.IndexedServiceNodes{}

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.ServiceNodes) != 0 {
		t.Fatalf("bad: %v", out)
	}
}

// TestCatalog_ListServiceNodes_ServiceTags_V1_2_3Compat asserts the compatibility between <=v1.2.3 agents and >=v1.3.0 servers
// see https://github.com/hashicorp/consul/issues/4922
func TestCatalog_ListServiceNodes_ServiceTags_V1_2_3Compat(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"})
	require.NoError(t, err)

	// register two service instances with different tags
	err = s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db1", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000})
	require.NoError(t, err)
	err = s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"secondary"}, Address: "127.0.0.1", Port: 5001})
	require.NoError(t, err)

	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	// make a request with the <=1.2.3 ServiceTag tag field (vs ServiceTags)
	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTag:  "primary",
		TagFilter:   true,
	}
	var out structs.IndexedServiceNodes
	err = msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	require.NoError(t, err)

	// nodes should be filtered, even when using the old ServiceTag field
	require.Equal(t, 1, len(out.ServiceNodes))
	require.Equal(t, "db1", out.ServiceNodes[0].ServiceID)

	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	// test with the other tag
	args = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTag:  "secondary",
		TagFilter:   true,
	}
	out = structs.IndexedServiceNodes{}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	require.NoError(t, err)
	require.Equal(t, 1, len(out.ServiceNodes))
	require.Equal(t, "db2", out.ServiceNodes[0].ServiceID)

	// no tag, both instances
	args = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
	}
	out = structs.IndexedServiceNodes{}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	require.NoError(t, err)
	require.Equal(t, 2, len(out.ServiceNodes))

	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	// when both ServiceTag and ServiceTags fields are populated, use ServiceTag
	args = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTag:  "primary",
		ServiceTags: []string{"secondary"},
		TagFilter:   true,
	}
	out = structs.IndexedServiceNodes{}
	err = msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	require.NoError(t, err)
	require.Equal(t, "db1", out.ServiceNodes[0].ServiceID)
}

func TestCatalog_ListServiceNodes_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add 2 nodes with specific meta maps
	node := &structs.Node{Node: "foo", Address: "127.0.0.1", Meta: map[string]string{"somekey": "somevalue", "common": "1"}}
	if err := s1.fsm.State().EnsureNode(1, node); err != nil {
		t.Fatalf("err: %v", err)
	}
	node2 := &structs.Node{Node: "bar", Address: "127.0.0.2", Meta: map[string]string{"common": "1"}}
	if err := s1.fsm.State().EnsureNode(2, node2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(3, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary", "v2"}, Address: "127.0.0.1", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(4, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"secondary", "v2"}, Address: "127.0.0.2", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []struct {
		filters  map[string]string
		tags     []string
		services structs.ServiceNodes
	}{
		// Basic meta filter
		{
			filters:  map[string]string{"somekey": "somevalue"},
			services: structs.ServiceNodes{&structs.ServiceNode{Node: "foo", ServiceID: "db"}},
		},
		// Basic meta filter, tag
		{
			filters:  map[string]string{"somekey": "somevalue"},
			tags:     []string{"primary"},
			services: structs.ServiceNodes{&structs.ServiceNode{Node: "foo", ServiceID: "db"}},
		},
		// Common meta filter
		{
			filters: map[string]string{"common": "1"},
			services: structs.ServiceNodes{
				&structs.ServiceNode{Node: "bar", ServiceID: "db2"},
				&structs.ServiceNode{Node: "foo", ServiceID: "db"},
			},
		},
		// Common meta filter, tag
		{
			filters: map[string]string{"common": "1"},
			tags:    []string{"secondary"},
			services: structs.ServiceNodes{
				&structs.ServiceNode{Node: "bar", ServiceID: "db2"},
			},
		},
		// Invalid meta filter
		{
			filters:  map[string]string{"invalid": "nope"},
			services: structs.ServiceNodes{},
		},
		// Multiple filter values
		{
			filters:  map[string]string{"somekey": "somevalue", "common": "1"},
			services: structs.ServiceNodes{&structs.ServiceNode{Node: "foo", ServiceID: "db"}},
		},
		// Multiple filter values, tag
		{
			filters:  map[string]string{"somekey": "somevalue", "common": "1"},
			tags:     []string{"primary"},
			services: structs.ServiceNodes{&structs.ServiceNode{Node: "foo", ServiceID: "db"}},
		},
		// Common meta filter, single tag
		{
			filters: map[string]string{"common": "1"},
			tags:    []string{"v2"},
			services: structs.ServiceNodes{
				&structs.ServiceNode{Node: "bar", ServiceID: "db2"},
				&structs.ServiceNode{Node: "foo", ServiceID: "db"},
			},
		},
		// Common meta filter, multiple tags
		{
			filters:  map[string]string{"common": "1"},
			tags:     []string{"v2", "primary"},
			services: structs.ServiceNodes{&structs.ServiceNode{Node: "foo", ServiceID: "db"}},
		},
	}

	for _, tc := range cases {
		args := structs.ServiceSpecificRequest{
			Datacenter:      "dc1",
			NodeMetaFilters: tc.filters,
			ServiceName:     "db",
			ServiceTags:     tc.tags,
			TagFilter:       len(tc.tags) > 0,
		}
		var out structs.IndexedServiceNodes
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
		require.Len(t, out.ServiceNodes, len(tc.services))

		for i, serviceNode := range out.ServiceNodes {
			if serviceNode.Node != tc.services[i].Node || serviceNode.ServiceID != tc.services[i].ServiceID {
				t.Fatalf("bad: %v, %v filters: %v", serviceNode, tc.services[i], tc.filters)
			}
		}
	}
}

func TestCatalog_ListServiceNodes_DistanceSort(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
	}
	var out structs.IndexedServiceNodes
	err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add a few nodes for the associated services.
	s1.fsm.State().EnsureNode(1, &structs.Node{Node: "aaa", Address: "127.0.0.1"})
	s1.fsm.State().EnsureService(2, "aaa", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000})
	s1.fsm.State().EnsureNode(3, &structs.Node{Node: "foo", Address: "127.0.0.2"})
	s1.fsm.State().EnsureService(4, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.2", Port: 5000})
	s1.fsm.State().EnsureNode(5, &structs.Node{Node: "bar", Address: "127.0.0.3"})
	s1.fsm.State().EnsureService(6, "bar", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.3", Port: 5000})
	s1.fsm.State().EnsureNode(7, &structs.Node{Node: "baz", Address: "127.0.0.4"})
	s1.fsm.State().EnsureService(8, "baz", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.4", Port: 5000})

	// Set all but one of the nodes to known coordinates.
	updates := structs.Coordinates{
		{Node: "foo", Coord: lib.GenerateCoordinate(2 * time.Millisecond)},
		{Node: "bar", Coord: lib.GenerateCoordinate(5 * time.Millisecond)},
		{Node: "baz", Coord: lib.GenerateCoordinate(1 * time.Millisecond)},
	}
	if err := s1.fsm.State().CoordinateBatchUpdate(9, updates); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query with no given source node, should get the natural order from
	// the index.
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.ServiceNodes) != 4 {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[0].Node != "aaa" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[1].Node != "bar" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[2].Node != "baz" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[3].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}

	// Query relative to foo, note that there's no known coordinate for "aaa"
	// so it will go at the end.
	args = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		Source:      structs.QuerySource{Datacenter: "dc1", Node: "foo"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.ServiceNodes) != 4 {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[0].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[1].Node != "baz" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[2].Node != "bar" {
		t.Fatalf("bad: %v", out)
	}
	if out.ServiceNodes[3].Node != "aaa" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_ListServiceNodes_ConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register the service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
		TagFilter:   false,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)
}

func TestCatalog_ListServiceNodes_ConnectDestination(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register the proxy service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// Register the service
	{
		dst := args.Service.Proxy.DestinationServiceName
		args := structs.TestRegisterRequest(t)
		args.Service.Service = dst
		var out struct{}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))
	}

	// List
	req := structs.ServiceSpecificRequest{
		Connect:     true,
		Datacenter:  "dc1",
		ServiceName: args.Service.Proxy.DestinationServiceName,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)

	// List by non-Connect
	req = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Proxy.DestinationServiceName,
	}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v = resp.ServiceNodes[0]
	assert.Equal(args.Service.Proxy.DestinationServiceName, v.ServiceName)
	assert.Equal("", v.ServiceProxy.DestinationServiceName)
}

// Test that calling ServiceNodes with Connect: true will return
// Connect native services.
func TestCatalog_ListServiceNodes_ConnectDestinationNative(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register the native service
	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true
	var out struct{}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Connect:     true,
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	require.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	require.Equal(args.Service.Service, v.ServiceName)

	// List by non-Connect
	req = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(resp.ServiceNodes, 1)
	v = resp.ServiceNodes[0]
	require.Equal(args.Service.Service, v.ServiceName)
}

func TestCatalog_ListServiceNodes_ConnectProxy_ACL(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var token string
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &token))

	{
		// Register a proxy
		args := structs.TestRegisterRequestProxy(t)
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "bar"
		args.WriteRequest.Token = "root"
		var out struct{}
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.WriteRequest.Token = "root"
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "another-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.WriteRequest.Token = "root"
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
	}

	// List w/ token. This should disallow because we don't have permission
	// to read "bar"
	req := structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "bar",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 0)

	// List w/ token. This should work since we're requesting "foo", but should
	// also only contain the proxies with names that adhere to our ACL.
	req = structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal("foo-proxy", v.ServiceName)
}

func TestCatalog_ListServiceNodes_ConnectNative(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register the service
	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
		TagFilter:   false,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(args.Service.Connect.Native, v.ServiceConnect.Native)
}

func TestCatalog_NodeServices(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	args := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out structs.IndexedNodeServices
	err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Just add a node
	if err := s1.fsm.State().EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s1.fsm.State().EnsureService(3, "foo", &structs.NodeService{ID: "web", Service: "web", Tags: nil, Address: "127.0.0.1", Port: 80}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.NodeServices.Node.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
	if len(out.NodeServices.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
	services := out.NodeServices.Services
	if !lib.StrContains(services["db"].Tags, "primary") || services["db"].Port != 5000 {
		t.Fatalf("bad: %v", out)
	}
	if len(services["web"].Tags) != 0 || services["web"].Port != 80 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_NodeServices_ConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register the service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       args.Node,
	}
	var resp structs.IndexedNodeServices
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &req, &resp))

	assert.Len(resp.NodeServices.Services, 1)
	v := resp.NodeServices.Services[args.Service.Service]
	assert.Equal(structs.ServiceKindConnectProxy, v.Kind)
	assert.Equal(args.Service.Proxy.DestinationServiceName, v.Proxy.DestinationServiceName)
}

func TestCatalog_NodeServices_ConnectNative(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register the service
	args := structs.TestRegisterRequest(t)
	var out struct{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       args.Node,
	}
	var resp structs.IndexedNodeServices
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &req, &resp))

	assert.Len(resp.NodeServices.Services, 1)
	v := resp.NodeServices.Services[args.Service.Service]
	assert.Equal(args.Service.Connect.Native, v.Connect.Native)
}

// Used to check for a regression against a known bug
func TestCatalog_Register_FailedCase1(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			Service: "web",
			Tags:    nil,
			Port:    8000,
		},
	}
	var out struct{}

	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check we can get this back
	query := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}
	var out2 structs.IndexedServiceNodes
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", query, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the output
	if len(out2.ServiceNodes) != 1 {
		t.Fatalf("Bad: %v", out2)
	}
}

func testACLFilterServer(t *testing.T) (dir, token string, srv *Server, codec rpc.ClientCodec) {
	dir, srv = testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})

	codec = rpcClient(t, srv)
	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	// Create a new token
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a service
	regArg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.config.NodeName,
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "foo",
			Service: "foo",
		},
		Check: &structs.HealthCheck{
			CheckID:   "service:foo",
			Name:      "service:foo",
			ServiceID: "foo",
			Status:    api.HealthPassing,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Register a service which should be denied
	regArg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.config.NodeName,
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "bar",
			Service: "bar",
		},
		Check: &structs.HealthCheck{
			CheckID:   "service:bar",
			Name:      "service:bar",
			ServiceID: "bar",
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil); err != nil {
		t.Fatalf("err: %s", err)
	}
	return
}

func TestCatalog_ListServices_FilterACL(t *testing.T) {
	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()
	testrpc.WaitForTestAgent(t, srv.RPC, "dc1")

	opt := structs.DCSpecificRequest{
		Datacenter:   "dc1",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := reply.Services["foo"]; !ok {
		t.Fatalf("bad: %#v", reply.Services)
	}
	if _, ok := reply.Services["bar"]; ok {
		t.Fatalf("bad: %#v", reply.Services)
	}
}

func TestCatalog_ServiceNodes_FilterACL(t *testing.T) {
	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()

	opt := structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedServiceNodes{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, sn := range reply.ServiceNodes {
		if sn.ServiceID == "foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bad: %#v", reply.ServiceNodes)
	}

	// Filters services we can't access
	opt = structs.ServiceSpecificRequest{
		Datacenter:   "dc1",
		ServiceName:  "bar",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply = structs.IndexedServiceNodes{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, sn := range reply.ServiceNodes {
		if sn.ServiceID == "bar" {
			t.Fatalf("bad: %#v", reply.ServiceNodes)
		}
	}

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestCatalog_NodeServices_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Prior to version 8, the node policy should be ignored.
	args := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       s1.config.NodeName,
	}
	reply := structs.IndexedNodeServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.NodeServices == nil {
		t.Fatalf("should not be nil")
	}

	// Now turn on version 8 enforcement and try again.
	s1.config.ACLEnforceVersion8 = true
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.NodeServices != nil {
		t.Fatalf("should not nil")
	}

	// Create an ACL that can read the node.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: fmt.Sprintf(`
node "%s" {
	policy = "read"
}
`, s1.config.NodeName),
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now try with the token and it will go through.
	args.Token = id
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.NodeServices == nil {
		t.Fatalf("should not be nil")
	}

	// Make sure an unknown node doesn't cause trouble.
	args.Node = "nope"
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply.NodeServices != nil {
		t.Fatalf("should not nil")
	}
}

func TestCatalog_NodeServices_FilterACL(t *testing.T) {
	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()
	testrpc.WaitForTestAgent(t, srv.RPC, "dc1")

	opt := structs.NodeSpecificRequest{
		Datacenter:   "dc1",
		Node:         srv.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: token},
	}
	reply := structs.IndexedNodeServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &opt, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, svc := range reply.NodeServices.Services {
		if svc.ID == "bar" {
			t.Fatalf("bad: %#v", reply.NodeServices.Services)
		}
		if svc.ID == "foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bad: %#v", reply.NodeServices)
	}
}
