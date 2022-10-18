package consul

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestCatalog_Register(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			Tags:    []string{"primary"},
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			Tags:    []string{"primary"},
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
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "write"
}
node "foo" {
	policy = "write"
}
`
	id := createToken(t, codec, rules)

	argR := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
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

	// Try the former special case for the "consul" service.
	argR.Service.Service = "consul"
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

func createTokenFull(t *testing.T, cc rpc.ClientCodec, policyRules string) *structs.ACLToken {
	t.Helper()
	return createTokenWithPolicyNameFull(t, cc, "the-policy", policyRules, "root")
}

func createToken(t *testing.T, cc rpc.ClientCodec, policyRules string) string {
	return createTokenFull(t, cc, policyRules).SecretID
}

func createTokenWithPolicyNameFull(t *testing.T, cc rpc.ClientCodec, policyName string, policyRules string, token string) *structs.ACLToken {
	t.Helper()

	reqPolicy := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Name:  policyName,
			Rules: policyRules,
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}
	err := msgpackrpc.CallWithCodec(cc, "ACL.PolicySet", &reqPolicy, &structs.ACLPolicy{})
	require.NoError(t, err)

	secretId, err := uuid.GenerateUUID()
	require.NoError(t, err)

	reqToken := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			SecretID: secretId,
			Policies: []structs.ACLTokenPolicyLink{{Name: policyName}},
		},
		WriteRequest: structs.WriteRequest{Token: token},
	}

	resp := &structs.ACLToken{}

	err = msgpackrpc.CallWithCodec(cc, "ACL.TokenSet", &reqToken, &resp)
	require.NoError(t, err)
	return resp
}

func createTokenWithPolicyName(t *testing.T, cc rpc.ClientCodec, policyName string, policyRules string, token string) string {
	return createTokenWithPolicyNameFull(t, cc, policyName, policyRules, token).SecretID
}

func TestCatalog_Register_ForwardLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			Tags:    []string{"primary"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ForwardDC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			Tags:    []string{"primary"},
			Port:    8000,
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_Register_ConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequestProxy(t)

	// Register
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(t, structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(t, args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)
}

// Test an invalid ConnectProxy. We don't need to exhaustively test because
// this is all tested in structs on the Validate method.
func TestCatalog_Register_ConnectProxy_invalid(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

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
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "DestinationServiceName")
}

// Test that write is required for the proxy destination to register a proxy.
func TestCatalog_Register_ConnectProxy_ACLDestinationServiceName(t *testing.T) {
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	rules := `
service "foo" {
	policy = "write"
}
node "foo" {
	policy = "write"
}
`
	token := createToken(t, codec, rules)

	// Register should fail because we don't have permission on the destination
	args := structs.TestRegisterRequestProxy(t)
	args.Service.Service = "foo"
	args.Service.Proxy.DestinationServiceName = "bar"
	args.WriteRequest.Token = token
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	assert.True(t, acl.IsErrPermissionDenied(err))

	// Register should fail with the right destination but wrong name
	args = structs.TestRegisterRequestProxy(t)
	args.Service.Service = "bar"
	args.Service.Proxy.DestinationServiceName = "foo"
	args.WriteRequest.Token = token
	err = msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out)
	assert.True(t, acl.IsErrPermissionDenied(err))

	// Register should work with the right destination
	args = structs.TestRegisterRequestProxy(t)
	args.Service.Service = "foo"
	args.Service.Proxy.DestinationServiceName = "foo"
	args.WriteRequest.Token = token
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
}

func TestCatalog_Register_ConnectNative(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true

	// Register
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(t, structs.ServiceKindTypical, v.ServiceKind)
	assert.True(t, v.ServiceConnect.Native)
}

func TestCatalog_Deregister(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	rules := `
node "node" {
	policy = "write"
}

service "service" {
	policy = "write"
}
`
	id := createToken(t, codec, rules)

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

	// We should be not be able to deregister everything without a token.
	var err error
	var out struct{}
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

	// Second pass these should all go through with the token set.
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
			Node:       "nope",
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
			Node:       "nope",
			CheckID:    "nope",
			WriteRequest: structs.WriteRequest{
				Token: id,
			}}, &out)
	if err == nil || !strings.Contains(err.Error(), "Unknown check") {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalog_ListDatacenters(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	require.False(t, out.QueryMeta.NotModified)

	t.Run("with option AllowNotModifiedResponse", func(t *testing.T) {
		args.QueryOptions = structs.QueryOptions{
			MinQueryIndex:            out.QueryMeta.Index,
			MaxQueryTime:             20 * time.Millisecond,
			AllowNotModifiedResponse: true,
		}
		err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out)
		require.NoError(t, err)

		require.Equal(t, out.Index, out.QueryMeta.Index)
		require.Len(t, out.Nodes, 0)
		require.True(t, out.QueryMeta.NotModified, "NotModified should be true")
	})
}

func TestCatalog_ListNodes_NodeMetaFilter(t *testing.T) {
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

func TestCatalog_RPC_Filter(t *testing.T) {
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

	// Run the tests against the test server

	t.Run("ListNodes", func(t *testing.T) {
		args := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Filter: "Meta.os == linux"},
		}

		out := new(structs.IndexedNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, out))
		require.Len(t, out.Nodes, 2)
		require.Condition(t, func() bool {
			return (out.Nodes[0].Node == "foo" && out.Nodes[1].Node == "baz") ||
				(out.Nodes[0].Node == "baz" && out.Nodes[1].Node == "foo")
		})

		args.Filter = "Meta.os == linux and Meta.env == qa"
		out = new(structs.IndexedNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, out))
		require.Len(t, out.Nodes, 1)
		require.Equal(t, "baz", out.Nodes[0].Node)
	})

	t.Run("ServiceNodes", func(t *testing.T) {
		args := structs.ServiceSpecificRequest{
			Datacenter:   "dc1",
			ServiceName:  "redis",
			QueryOptions: structs.QueryOptions{Filter: "ServiceMeta.version == 1"},
		}

		out := new(structs.IndexedServiceNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
		require.Len(t, out.ServiceNodes, 2)
		require.Condition(t, func() bool {
			return (out.ServiceNodes[0].Node == "foo" && out.ServiceNodes[1].Node == "bar") ||
				(out.ServiceNodes[0].Node == "bar" && out.ServiceNodes[1].Node == "foo")
		})

		args.Filter = "ServiceMeta.version == 2"
		out = new(structs.IndexedServiceNodes)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
		require.Len(t, out.ServiceNodes, 1)
		require.Equal(t, "foo", out.ServiceNodes[0].Node)
	})

	t.Run("NodeServices", func(t *testing.T) {
		args := structs.NodeSpecificRequest{
			Datacenter:   "dc1",
			Node:         "baz",
			QueryOptions: structs.QueryOptions{Filter: "Service == web"},
		}

		out := new(structs.IndexedNodeServices)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &out))
		require.Len(t, out.NodeServices.Services, 2)

		args.Filter = "Service == web and Meta.version == 2"
		out = new(structs.IndexedNodeServices)
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &out))
		require.Len(t, out.NodeServices.Services, 1)
	})
}

func TestCatalog_ListNodes_StaleRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	retry.Run(t, func(r *retry.R) {
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}

		found := false
		for _, n := range out.Nodes {
			if n.Node == "foo" {
				found = true
			}
		}
		if !found {
			r.Fatalf("failed to find foo in %#v", out.Nodes)
		}
		if out.QueryMeta.LastContact == 0 {
			r.Fatalf("should have a last contact time")
		}
		if !out.QueryMeta.KnownLeader {
			r.Fatalf("should have known leader")
		}
	})
}

func TestCatalog_ListNodes_ConsistentRead_Fail(t *testing.T) {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	token := func(policy string) string {
		rules := fmt.Sprintf(
			`node "%s" { policy = "%s" }`,
			s1.config.NodeName,
			policy,
		)
		return createTokenWithPolicyName(t, codec, policy, rules, "root")
	}

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	t.Run("deny", func(t *testing.T) {
		args.Token = token("deny")

		var reply structs.IndexedNodes
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 0 {
			t.Fatalf("bad: %v", reply.Nodes)
		}
		if !reply.QueryMeta.ResultsFilteredByACLs {
			t.Fatal("ResultsFilteredByACLs should be true")
		}
	})

	t.Run("allow", func(t *testing.T) {
		args.Token = token("read")

		var reply structs.IndexedNodes
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListNodes", &args, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(reply.Nodes) != 1 {
			t.Fatalf("bad: %v", reply.Nodes)
		}
		if reply.QueryMeta.ResultsFilteredByACLs {
			t.Fatal("ResultsFilteredByACLs should not true")
		}
	})
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	require.False(t, out.QueryMeta.NotModified)
	require.False(t, out.QueryMeta.ResultsFilteredByACLs)

	t.Run("with option AllowNotModifiedResponse", func(t *testing.T) {
		args.QueryOptions = structs.QueryOptions{
			MinQueryIndex:            out.QueryMeta.Index,
			MaxQueryTime:             20 * time.Millisecond,
			AllowNotModifiedResponse: true,
		}
		err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out)
		require.NoError(t, err)

		require.Equal(t, out.Index, out.QueryMeta.Index)
		require.Len(t, out.Services, 0)
		require.True(t, out.QueryMeta.NotModified, "NotModified should be true")
	})
}

func TestCatalog_ListServices_NodeMetaFilter(t *testing.T) {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			t.Errorf("err: %v", err)
			return
		}
		if err := s1.fsm.State().EnsureService(idx+2, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "127.0.0.1", Port: 5000}); err != nil {
			t.Errorf("err: %v", err)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1" // Enable ACLs!
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
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err.Error() != structs.ErrNoLeader.Error() {
		t.Fatalf("expected %v but got err: %v and %v", structs.ErrNoLeader, err, out)
	}

	// Try to join
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) { r.Check(wantRaft([]*Server{s1, s2})) })
	waitForLeader(s1, s2)

	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		out = structs.IndexedServices{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
		// Should find the services
		if len(out.Services) != 1 {
			r.Fatalf("bad: %#v", out.Services)
		}
		if !out.KnownLeader {
			r.Fatalf("should have a leader: %v", out)
		}
	})

	s1.Leave()
	s1.Shutdown()

	testrpc.WaitUntilNoLeader(t, s2.RPC, "dc1")

	args.AllowStale = false
	// Since the leader is now down, non-stale query should fail now
	out = structs.IndexedServices{}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err.Error() != structs.ErrLeaderNotTracked.Error() {
		t.Fatalf("expected %v but got err: %v and %v", structs.ErrNoLeader, err, out)
	}
	if out.KnownLeader {
		t.Fatalf("should not have a leader anymore: %#v", out)
	}

	// With stale, request should still work
	args.AllowStale = true
	retry.Run(t, func(r *retry.R) {
		out = structs.IndexedServices{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.ListServices", &args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
		if out.KnownLeader || len(out.Services) != 1 {
			r.Fatalf("got %t nodes want %d", out.KnownLeader, len(out.Services))
		}
	})
}

func TestCatalog_ListServiceNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTags: []string{"replica"},
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

func TestCatalog_ListServiceNodes_ByAddress(t *testing.T) {
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

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
	}
	var out structs.IndexedServiceNodes
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))

	fooAddress := "10.1.2.3"
	fooPort := 1111
	fooTaggedAddresses := map[string]structs.ServiceAddress{
		"lan": {
			Address: "10.1.2.3",
			Port:    fooPort,
		},
		"wan": {
			Address: "198.18.1.2",
			Port:    fooPort,
		},
	}
	barAddress := "10.1.2.3"
	barPort := 2222
	barTaggedAddresses := map[string]structs.ServiceAddress{
		"lan": {
			Address: "10.1.2.3",
			Port:    barPort,
		},
		"wan": {
			Address: "198.18.2.3",
			Port:    barPort,
		},
	}
	bazAddress := "192.168.1.35"
	bazPort := 2222
	bazTaggedAddresses := map[string]structs.ServiceAddress{
		"lan": {
			Address: "192.168.1.35",
			Port:    barPort,
		},
		"wan": {
			Address: "198.18.2.4",
			Port:    barPort,
		},
	}

	// Just add a node
	require.NoError(t, s1.fsm.State().EnsureNode(1, &structs.Node{Node: "node", Address: "127.0.0.1"}))
	require.NoError(t, s1.fsm.State().EnsureService(2, "node", &structs.NodeService{ID: "foo", Service: "db", Address: fooAddress, TaggedAddresses: fooTaggedAddresses, Port: fooPort}))
	require.NoError(t, s1.fsm.State().EnsureService(2, "node", &structs.NodeService{ID: "bar", Service: "db", Address: barAddress, TaggedAddresses: barTaggedAddresses, Port: barPort}))
	require.NoError(t, s1.fsm.State().EnsureService(2, "node", &structs.NodeService{ID: "baz", Service: "db", Address: bazAddress, TaggedAddresses: bazTaggedAddresses, Port: bazPort}))
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
	require.Len(t, out.ServiceNodes, 3)

	// Try with an address that would match foo & bar
	args.ServiceAddress = "10.1.2.3"
	out = structs.IndexedServiceNodes{}

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
	require.Len(t, out.ServiceNodes, 2)
	for _, sn := range out.ServiceNodes {
		require.True(t, sn.ServiceID == "foo" || sn.ServiceID == "bar")
	}

	// Try with an address that would match just bar
	args.ServiceAddress = "198.18.2.3"
	out = structs.IndexedServiceNodes{}

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &args, &out))
	require.Len(t, out.ServiceNodes, 1)
	require.Equal(t, "bar", out.ServiceNodes[0].ServiceID)
}

// TestCatalog_ListServiceNodes_ServiceTags_V1_2_3Compat asserts the compatibility between <=v1.2.3 agents and >=v1.3.0 servers
// see https://github.com/hashicorp/consul/issues/4922
func TestCatalog_ListServiceNodes_ServiceTags_V1_2_3Compat(t *testing.T) {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	// Register the service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
		TagFilter:   false,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(t, structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(t, args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)
}

func TestCatalog_ServiceNodes_Gateway(t *testing.T) {
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

		// Register a proxy for api
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "api-proxy"
		args.Service.Proxy.DestinationServiceName = "api"
		args.Check = &structs.HealthCheck{
			Name:      "api-proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a service "web"
		args = structs.TestRegisterRequest(t)
		args.Check = &structs.HealthCheck{
			Name:      "web",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy for web
		args = structs.TestRegisterRequestProxy(t)
		args.Check = &structs.HealthCheck{
			Name:      "web-proxy",
			Status:    api.HealthPassing,
			ServiceID: args.Service.Service,
		}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a gateway for web
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
				ServiceID: args.Service.Service,
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
						Name: "web",
					},
				},
			},
		}
		var entryResp bool
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &entryArgs, &entryResp))
	}

	retry.Run(t, func(r *retry.R) {
		// List should return both the terminating-gateway and the connect-proxy associated with web
		req := structs.ServiceSpecificRequest{
			Connect:     true,
			Datacenter:  "dc1",
			ServiceName: "web",
		}
		var resp structs.IndexedServiceNodes
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
		assert.Len(r, resp.ServiceNodes, 2)

		// Check sidecar
		assert.Equal(r, structs.ServiceKindConnectProxy, resp.ServiceNodes[0].ServiceKind)
		assert.Equal(r, "foo", resp.ServiceNodes[0].Node)
		assert.Equal(r, "web-proxy", resp.ServiceNodes[0].ServiceName)
		assert.Equal(r, "web-proxy", resp.ServiceNodes[0].ServiceID)
		assert.Equal(r, "web", resp.ServiceNodes[0].ServiceProxy.DestinationServiceName)
		assert.Equal(r, 2222, resp.ServiceNodes[0].ServicePort)

		// Check gateway
		assert.Equal(r, structs.ServiceKindTerminatingGateway, resp.ServiceNodes[1].ServiceKind)
		assert.Equal(r, "foo", resp.ServiceNodes[1].Node)
		assert.Equal(r, "gateway", resp.ServiceNodes[1].ServiceName)
		assert.Equal(r, "gateway", resp.ServiceNodes[1].ServiceID)
		assert.Equal(r, 443, resp.ServiceNodes[1].ServicePort)
	})
}

func TestCatalog_ListServiceNodes_ConnectDestination(t *testing.T) {
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

	// Register the proxy service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// Register the service
	{
		dst := args.Service.Proxy.DestinationServiceName
		args := structs.TestRegisterRequest(t)
		args.Service.Service = dst
		var out struct{}
		assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))
	}

	// List
	req := structs.ServiceSpecificRequest{
		Connect:     true,
		Datacenter:  "dc1",
		ServiceName: args.Service.Proxy.DestinationServiceName,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(t, structs.ServiceKindConnectProxy, v.ServiceKind)
	assert.Equal(t, args.Service.Proxy.DestinationServiceName, v.ServiceProxy.DestinationServiceName)

	// List by non-Connect
	req = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Proxy.DestinationServiceName,
	}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v = resp.ServiceNodes[0]
	assert.Equal(t, args.Service.Proxy.DestinationServiceName, v.ServiceName)
	assert.Equal(t, "", v.ServiceProxy.DestinationServiceName)
}

// Test that calling ServiceNodes with Connect: true will return
// Connect native services.
func TestCatalog_ListServiceNodes_ConnectDestinationNative(t *testing.T) {
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

	// Register the native service
	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true
	var out struct{}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Connect:     true,
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	var resp structs.IndexedServiceNodes
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	require.Equal(t, args.Service.Service, v.ServiceName)

	// List by non-Connect
	req = structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
	}
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(t, resp.ServiceNodes, 1)
	v = resp.ServiceNodes[0]
	require.Equal(t, args.Service.Service, v.ServiceName)
}

func TestCatalog_ListServiceNodes_ConnectProxy_ACL(t *testing.T) {
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	rules := `
service "foo-proxy" {
	policy = "write"
}
service "foo" {
	policy = "write"
}
node "foo" {
	policy = "read"
}
`
	token := createToken(t, codec, rules)

	{
		// Register a proxy
		args := structs.TestRegisterRequestProxy(t)
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "bar"
		args.WriteRequest.Token = "root"
		var out struct{}
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "foo-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.WriteRequest.Token = "root"
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))

		// Register a proxy
		args = structs.TestRegisterRequestProxy(t)
		args.Service.Service = "another-proxy"
		args.Service.Proxy.DestinationServiceName = "foo"
		args.WriteRequest.Token = "root"
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", &args, &out))
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
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(t, resp.ServiceNodes, 0)

	// List w/ token. This should work since we're requesting "foo", but should
	// also only contain the proxies with names that adhere to our ACL.
	req = structs.ServiceSpecificRequest{
		Connect:      true,
		Datacenter:   "dc1",
		ServiceName:  "foo",
		QueryOptions: structs.QueryOptions{Token: token},
	}
	resp = structs.IndexedServiceNodes{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	require.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	require.Equal(t, "foo-proxy", v.ServiceName)
	require.True(t, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
}

func TestCatalog_ListServiceNodes_ConnectNative(t *testing.T) {
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

	// Register the service
	args := structs.TestRegisterRequest(t)
	args.Service.Connect.Native = true
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: args.Service.Service,
		TagFilter:   false,
	}
	var resp structs.IndexedServiceNodes
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.ServiceNodes", &req, &resp))
	assert.Len(t, resp.ServiceNodes, 1)
	v := resp.ServiceNodes[0]
	assert.Equal(t, args.Service.Connect.Native, v.ServiceConnect.Native)
}

func TestCatalog_NodeServices(t *testing.T) {
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
	if !stringslice.Contains(services["db"].Tags, "primary") || services["db"].Port != 5000 {
		t.Fatalf("bad: %v", out)
	}
	if len(services["web"].Tags) != 0 || services["web"].Port != 80 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalog_NodeServices_ConnectProxy(t *testing.T) {
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

	// Register the service
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       args.Node,
	}
	var resp structs.IndexedNodeServices
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &req, &resp))

	assert.Len(t, resp.NodeServices.Services, 1)
	v := resp.NodeServices.Services[args.Service.Service]
	assert.Equal(t, structs.ServiceKindConnectProxy, v.Kind)
	assert.Equal(t, args.Service.Proxy.DestinationServiceName, v.Proxy.DestinationServiceName)
}

func TestCatalog_NodeServices_ConnectNative(t *testing.T) {
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

	// Register the service
	args := structs.TestRegisterRequest(t)
	var out struct{}
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.Register", args, &out))

	// List
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       args.Node,
	}
	var resp structs.IndexedNodeServices
	assert.Nil(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &req, &resp))

	assert.Len(t, resp.NodeServices.Services, 1)
	v := resp.NodeServices.Services[args.Service.Service]
	assert.Equal(t, args.Service.Connect.Native, v.Connect.Native)
}

// Used to check for a regression against a known bug
func TestCatalog_Register_FailedCase1(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})

	codec = rpcClient(t, srv)
	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken("root"))

	rules := `
service "foo" {
	policy = "write"
}
node_prefix "" {
	policy = "read"
}
`
	token = createToken(t, codec, rules)

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
			Status:    api.HealthPassing,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &regArg, nil); err != nil {
		t.Fatalf("err: %s", err)
	}
	return
}

func TestCatalog_ListServices_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()
	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken("root"))

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
	if !reply.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}
}

func TestCatalog_ServiceNodes_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// We've already proven that we call the ACL filtering function so we
	// test node filtering down in acl.go for node cases. This also proves
	// that we respect the version 8 ACL flag, since the test server sets
	// that to false (the regression value of *not* changing this is better
	// for now until we change the sense of the version 8 ACL flag).
}

func TestCatalog_NodeServices_ACL(t *testing.T) {
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

	token := func(policy string) string {
		rules := fmt.Sprintf(`
				node "%s" { policy = "%s" }
				service "consul" { policy = "%s" }
			`,
			s1.config.NodeName,
			policy,
			policy,
		)
		return createTokenWithPolicyName(t, codec, policy, rules, "root")
	}

	args := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       s1.config.NodeName,
	}

	t.Run("deny", func(t *testing.T) {

		args.Token = token("deny")

		var reply structs.IndexedNodeServices
		err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply)
		require.NoError(t, err)
		require.Nil(t, reply.NodeServices)
		require.True(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("allow", func(t *testing.T) {

		args.Token = token("read")

		var reply structs.IndexedNodeServices
		err := msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &args, &reply)
		require.NoError(t, err)
		require.NotNil(t, reply.NodeServices)
		require.False(t, reply.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})
}

func TestCatalog_NodeServices_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir, token, srv, codec := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer codec.Close()
	testrpc.WaitForTestAgent(t, srv.RPC, "dc1", testrpc.WithToken("root"))

	opt := structs.NodeSpecificRequest{
		Datacenter:   "dc1",
		Node:         srv.config.NodeName,
		QueryOptions: structs.QueryOptions{Token: token},
	}

	var reply structs.IndexedNodeServices
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.NodeServices", &opt, &reply))

	require.NotNil(t, reply.NodeServices)
	require.Len(t, reply.NodeServices.Services, 1)

	svc, ok := reply.NodeServices.Services["foo"]
	require.True(t, ok)
	require.Equal(t, "foo", svc.ID)
}

func TestCatalog_GatewayServices_TerminatingGateway(t *testing.T) {
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
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 3)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "",
				CertFile:    "",
				KeyFile:     "",
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:      structs.NewServiceName("redis", nil),
				Gateway:      structs.NewServiceName("gateway", nil),
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

func TestCatalog_GatewayServices_BothGateways(t *testing.T) {
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
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 1)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				ServiceKind: structs.GatewayServiceKindService,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)

		req.ServiceName = "ingress"
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 1)

		expect = structs.GatewayServices{
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
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
	err := msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp)
	assert.NoError(t, err)
	assert.Empty(t, resp.Services)
	// Ensure that the index is not zero so that a blocking query still gets the
	// latest GatewayServices index
	assert.NotEqual(t, 0, resp.Index)
}

func TestCatalog_GatewayServices_ACLFiltering(t *testing.T) {
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
		err := msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp)
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
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 0)
		assert.True(r, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
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
		assert.Nil(r, msgpackrpc.CallWithCodec(codec, "Catalog.GatewayServices", &req, &resp))
		assert.Len(r, resp.Services, 2)
		assert.True(r, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db_replica", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				ServiceKind: structs.GatewayServiceKindUnknown,
			},
		}

		// Ignore raft index for equality
		for _, s := range resp.Services {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, resp.Services)
	})
}

func TestVetRegisterWithACL(t *testing.T) {
	appendAuthz := func(t *testing.T, defaultAuthz acl.Authorizer, rules string) acl.Authorizer {
		policy, err := acl.NewPolicyFromSource(rules, acl.SyntaxCurrent, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(defaultAuthz, []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		return authz
	}

	t.Run("With an 'allow all' authorizer the update should be allowed", func(t *testing.T) {
		args := &structs.RegisterRequest{
			Node:    "nope",
			Address: "127.0.0.1",
		}

		// With an "allow all" authorizer the update should be allowed.
		require.NoError(t, vetRegisterWithACL(resolver.Result{Authorizer: acl.ManageAll()}, args, nil))
	})

	var perms acl.Authorizer = acl.DenyAll()
	var resolvedPerms resolver.Result

	args := &structs.RegisterRequest{
		Node:    "nope",
		Address: "127.0.0.1",
	}

	// Create a basic node policy.
	perms = appendAuthz(t, perms, `
	node "node" {
	  policy = "write"
	} `)
	resolvedPerms = resolver.Result{Authorizer: perms}

	// With that policy, the update should now be blocked for node reasons.
	err := vetRegisterWithACL(resolvedPerms, args, nil)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Now use a permitted node name.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, nil))

	// Build some node info that matches what we have now.
	ns := &structs.NodeServices{
		Node: &structs.Node{
			Node:    "node",
			Address: "127.0.0.1",
		},
		Services: make(map[string]*structs.NodeService),
	}

	// Try to register a service, which should be blocked.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			ID:      "my-id",
		},
	}
	err = vetRegisterWithACL(resolver.Result{Authorizer: perms}, args, ns)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Chain on a basic service policy.
	perms = appendAuthz(t, perms, `
	service "service" {
	  policy = "write"
	} `)
	resolvedPerms = resolver.Result{Authorizer: perms}

	// With the service ACL, the update should go through.
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Add an existing service that they are clobbering and aren't allowed
	// to write to.
	ns = &structs.NodeServices{
		Node: &structs.Node{
			Node:    "node",
			Address: "127.0.0.1",
		},
		Services: map[string]*structs.NodeService{
			"my-id": {
				Service: "other",
				ID:      "my-id",
			},
		},
	}
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Chain on a policy that allows them to write to the other service.
	perms = appendAuthz(t, perms, `
	service "other" {
	  policy = "write"
	} `)
	resolvedPerms = resolver.Result{Authorizer: perms}

	// Now it should go through.
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Try creating the node and the service at once by having no existing
	// node record. This should be ok since we have node and service
	// permissions.
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, nil))

	// Add a node-level check to the member, which should be rejected.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			ID:      "my-id",
		},
		Check: &structs.HealthCheck{
			Node: "node",
		},
	}
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	testutil.RequireErrorContains(t, err, "check member must be nil")

	// Move the check into the slice, but give a bad node name.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			ID:      "my-id",
		},
		Checks: []*structs.HealthCheck{
			{
				Node: "nope",
			},
		},
	}
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	testutil.RequireErrorContains(t, err, "doesn't match register request node")

	// Fix the node name, which should now go through.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			ID:      "my-id",
		},
		Checks: []*structs.HealthCheck{
			{
				Node: "node",
			},
		},
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Add a service-level check.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "service",
			ID:      "my-id",
		},
		Checks: []*structs.HealthCheck{
			{
				Node: "node",
			},
			{
				Node:      "node",
				ServiceID: "my-id",
			},
		},
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Try creating everything at once. This should be ok since we have all
	// the permissions we need. It also makes sure that we can register a
	// new node, service, and associated checks.
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, nil))

	// Nil out the service registration, which'll skip the special case
	// and force us to look at the ns data (it will look like we are
	// writing to the "other" service which also has "my-id").
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Checks: []*structs.HealthCheck{
			{
				Node: "node",
			},
			{
				Node:      "node",
				ServiceID: "my-id",
			},
		},
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Chain on a policy that forbids them to write to the other service.
	perms = appendAuthz(t, perms, `
	service "other" {
	  policy = "deny"
	} `)
	resolvedPerms = resolver.Result{Authorizer: perms}

	// This should get rejected.
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Change the existing service data to point to a service name they
	// can write to. This should go through.
	ns = &structs.NodeServices{
		Node: &structs.Node{
			Node:    "node",
			Address: "127.0.0.1",
		},
		Services: map[string]*structs.NodeService{
			"my-id": {
				Service: "service",
				ID:      "my-id",
			},
		},
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Chain on a policy that forbids them to write to the node.
	perms = appendAuthz(t, perms, `
	node "node" {
	  policy = "deny"
	} `)
	resolvedPerms = resolver.Result{Authorizer: perms}

	// This should get rejected because there's a node-level check in here.
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Change the node-level check into a service check, and then it should
	// go through.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.1",
		Checks: []*structs.HealthCheck{
			{
				Node:      "node",
				ServiceID: "my-id",
			},
			{
				Node:      "node",
				ServiceID: "my-id",
			},
		},
	}
	require.NoError(t, vetRegisterWithACL(resolvedPerms, args, ns))

	// Finally, attempt to update the node part of the data and make sure
	// that gets rejected since they no longer have permissions.
	args = &structs.RegisterRequest{
		Node:    "node",
		Address: "127.0.0.2",
		Checks: []*structs.HealthCheck{
			{
				Node:      "node",
				ServiceID: "my-id",
			},
			{
				Node:      "node",
				ServiceID: "my-id",
			},
		},
	}
	err = vetRegisterWithACL(resolvedPerms, args, ns)
	require.True(t, acl.IsErrPermissionDenied(err))
}

func TestVetDeregisterWithACL(t *testing.T) {
	t.Parallel()
	args := &structs.DeregisterRequest{
		Node: "nope",
	}

	// With an "allow all" authorizer the update should be allowed.
	if err := vetDeregisterWithACL(resolver.Result{Authorizer: acl.ManageAll()}, args, nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a basic node policy.
	policy, err := acl.NewPolicyFromSource(`
node "node" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	nodePerms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	policy, err = acl.NewPolicyFromSource(`
	service "my-service" {
	  policy = "write"
	}
	`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	servicePerms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, args := range []struct {
		DeregisterRequest structs.DeregisterRequest
		Service           *structs.NodeService
		Check             *structs.HealthCheck
		Perms             acl.Authorizer
		Expected          bool
		Name              string
	}{
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "nope",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "nope",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "right on service but node dergister request",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no rights on node nor service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "no rights on node but rights on service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node nor service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "no rights on node but rights on service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "nope",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node for node check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "nope",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service but no right on node for node check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "node",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "node",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service but not on node for node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "rights on service for service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "right on node for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "rights on service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "node",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "node",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service for node check",
		},
	} {
		t.Run(args.Name, func(t *testing.T) {
			err = vetDeregisterWithACL(resolver.Result{Authorizer: args.Perms}, &args.DeregisterRequest, args.Service, args.Check)
			if !args.Expected {
				if err == nil {
					t.Errorf("expected error with %+v", args.DeregisterRequest)
				}
				if !acl.IsErrPermissionDenied(err) {
					t.Errorf("expected permission denied error with %+v, instead got %+v", args.DeregisterRequest, err)
				}
			} else if err != nil {
				t.Errorf("expected no error with %+v", args.DeregisterRequest)
			}
		})
	}
}

func TestCatalog_VirtualIPForService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.11.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	err := s1.fsm.State().EnsureRegistration(1, &structs.RegisterRequest{
		Node:    "foo",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Connect: structs.ServiceConnect{
				Native: true,
			},
		},
	})
	require.NoError(t, err)

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "api",
	}
	var out string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.VirtualIPForService", &args, &out))
	require.Equal(t, "240.0.0.1", out)
}

func TestCatalog_VirtualIPForService_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.Build = "1.11.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	err := s1.fsm.State().EnsureRegistration(1, &structs.RegisterRequest{
		Node:    "foo",
		Address: "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Connect: structs.ServiceConnect{
				Native: true,
			},
		},
	})
	require.NoError(t, err)

	// Call the endpoint with no token and expect permission denied.
	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "api",
	}
	var out string
	err = msgpackrpc.CallWithCodec(codec, "Catalog.VirtualIPForService", &args, &out)
	require.Contains(t, err.Error(), acl.ErrPermissionDenied.Error())
	require.Equal(t, "", out)

	id := createToken(t, codec, `
	service "api" {
		policy = "read"
	}`)

	// Now try with the token and it will go through.
	args.Token = id
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Catalog.VirtualIPForService", &args, &out))
	require.Equal(t, "240.0.0.1", out)

	// Make sure we still get permission denied for an unknown service.
	args.ServiceName = "nope"
	var out2 string
	err = msgpackrpc.CallWithCodec(codec, "Catalog.VirtualIPForService", &args, &out2)
	require.Contains(t, err.Error(), acl.ErrPermissionDenied.Error())
	require.Equal(t, "", out2)
}
