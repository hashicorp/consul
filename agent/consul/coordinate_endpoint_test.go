package consul

import (
	"fmt"
	"math"
	"math/rand"
	"net/rpc"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/serf/coordinate"
	"github.com/pascaldekloe/goe/verify"
)

// generateRandomCoordinate creates a random coordinate. This mucks with the
// underlying structure directly, so it's not really useful for any particular
// position in the network, but it's a good payload to send through to make
// sure things come out the other side or get stored correctly.
func generateRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(config)
	for i := range coord.Vec {
		coord.Vec[i] = rand.NormFloat64()
	}
	coord.Error = rand.NormFloat64()
	coord.Adjustment = rand.NormFloat64()
	return coord
}

func TestCoordinate_Update(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.CoordinateUpdatePeriod = 500 * time.Millisecond
		c.CoordinateUpdateBatchSize = 5
		c.CoordinateUpdateMaxBatches = 2
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec); err != nil {
		t.Fatal(err)
	}

	// Send an update for the first node.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Send an update for the second node.
	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node2",
		Coord:      generateRandomCoordinate(),
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the updates did not yet apply because the update period
	// hasn't expired.
	state := s1.fsm.State()
	_, c, err := state.Coordinate("node1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", c, lib.CoordinateSet{})

	_, c, err = state.Coordinate("node2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", c, lib.CoordinateSet{})

	// Send another update for the second node. It should take precedence
	// since there will be two updates in the same batch.
	arg2.Coord = generateRandomCoordinate()
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait a while and the updates should get picked up.
	time.Sleep(3 * s1.config.CoordinateUpdatePeriod)
	_, c, err = state.Coordinate("node1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected := lib.CoordinateSet{
		"": arg1.Coord,
	}
	verify.Values(t, "", c, expected)

	_, c, err = state.Coordinate("node2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected = lib.CoordinateSet{
		"": arg2.Coord,
	}
	verify.Values(t, "", c, expected)

	// Register a bunch of additional nodes.
	spamLen := s1.config.CoordinateUpdateBatchSize*s1.config.CoordinateUpdateMaxBatches + 1
	for i := 0; i < spamLen; i++ {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("bogusnode%d", i),
			Address:    "127.0.0.1",
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Now spam some coordinate updates and make sure it starts throwing
	// them away if they exceed the batch allowance. Note we have to make
	// unique names since these are held in map by node name.
	for i := 0; i < spamLen; i++ {
		arg1.Node = fmt.Sprintf("bogusnode%d", i)
		arg1.Coord = generateRandomCoordinate()
		if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg1, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait a little while for the batch routine to run, then make sure
	// exactly one of the updates got dropped (we won't know which one).
	time.Sleep(3 * s1.config.CoordinateUpdatePeriod)
	numDropped := 0
	for i := 0; i < spamLen; i++ {
		_, c, err = state.Coordinate(fmt.Sprintf("bogusnode%d", i), nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(c) == 0 {
			numDropped++
		}
	}
	if numDropped != 1 {
		t.Fatalf("wrong number of coordinates dropped, %d != 1", numDropped)
	}

	// Send a coordinate with a NaN to make sure that we don't absorb that
	// into the database.
	arg2.Coord.Vec[0] = math.NaN()
	err = msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out)
	if err == nil || !strings.Contains(err.Error(), "invalid coordinate") {
		t.Fatalf("should have failed with an error, got %v", err)
	}

	// Finally, send a coordinate with the wrong dimensionality to make sure
	// there are no panics, and that it gets rejected.
	arg2.Coord.Vec = make([]float64, 2*len(arg2.Coord.Vec))
	err = msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out)
	if err == nil || !strings.Contains(err.Error(), "incompatible coordinate") {
		t.Fatalf("should have failed with an error, got %v", err)
	}
}

func TestCoordinate_Update_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec); err != nil {
		t.Fatal(err)
	}

	// Send an update for the first node. This should go through since we
	// don't have version 8 ACLs enforced yet.
	req := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now turn on version 8 enforcement and try again.
	s1.config.ACLEnforceVersion8 = true
	err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Create an ACL that can write to the node.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTypeClient,
			Rules: `
node "node1" {
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

	// With the token, it should now go through.
	req.Token = id
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// But it should be blocked for the other node.
	req.Node = "node2"
	err = msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestCoordinate_ListDatacenters(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// It's super hard to force the Serfs into a known configuration of
	// coordinates, so the best we can do is make sure our own DC shows
	// up in the list with the proper coordinates. The guts of the algorithm
	// are extensively tested in rtt_test.go using a mock database.
	var out []structs.DatacenterMap
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListDatacenters", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 1 ||
		out[0].Datacenter != "dc1" ||
		len(out[0].Coordinates) != 1 ||
		out[0].Coordinates[0].Node != s1.config.NodeName {
		t.Fatalf("bad: %v", out)
	}
	c, err := s1.serfWAN.GetCoordinate()
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	verify.Values(t, "", c, out[0].Coordinates[0].Coord)
}

func TestCoordinate_ListNodes(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"foo", "bar", "baz"}
	if err := registerNodes(nodes, codec); err != nil {
		t.Fatal(err)
	}

	// Send coordinate updates for a few nodes.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      generateRandomCoordinate(),
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg3 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "baz",
		Coord:      generateRandomCoordinate(),
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg3, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Now query back for all the nodes.
	retry.Run(t, func(r *retry.R) {
		arg := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		resp := structs.IndexedCoordinates{}
		if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListNodes", &arg, &resp); err != nil {
			r.Fatalf("err: %v", err)
		}
		if len(resp.Coordinates) != 3 ||
			resp.Coordinates[0].Node != "bar" ||
			resp.Coordinates[1].Node != "baz" ||
			resp.Coordinates[2].Node != "foo" {
			r.Fatalf("bad: %v", resp.Coordinates)
		}
		verify.Values(t, "", resp.Coordinates[0].Coord, arg2.Coord) // bar
		verify.Values(t, "", resp.Coordinates[1].Coord, arg3.Coord) // baz
		verify.Values(t, "", resp.Coordinates[2].Coord, arg1.Coord) // foo
	})
}

func TestCoordinate_ListNodes_ACLFilter(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"foo", "bar", "baz"}
	for _, node := range nodes {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       node,
			Address:    "127.0.0.1",
			WriteRequest: structs.WriteRequest{
				Token: "root",
			},
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Send coordinate updates for a few nodes.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      generateRandomCoordinate(),
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      generateRandomCoordinate(),
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg3 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "baz",
		Coord:      generateRandomCoordinate(),
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg3, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Wait for all the coordinate updates to apply. Since we aren't
	// enforcing version 8 ACLs, this should also allow us to read
	// everything back without a token.
	retry.Run(t, func(r *retry.R) {
		arg := structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		resp := structs.IndexedCoordinates{}
		if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListNodes", &arg, &resp); err != nil {
			r.Fatalf("err: %v", err)
		}
		if got, want := len(resp.Coordinates), 3; got != want {
			r.Fatalf("got %d coordinates want %d", got, want)
		}
	})

	// Now that we've waited for the batch processing to ingest the
	// coordinates we can do the rest of the requests without the loop. We
	// will start by turning on version 8 ACL support which should block
	// everything.
	s1.config.ACLEnforceVersion8 = true
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	resp := structs.IndexedCoordinates{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListNodes", &arg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Coordinates) != 0 {
		t.Fatalf("bad: %#v", resp.Coordinates)
	}

	// Create an ACL that can read one of the nodes.
	var id string
	{
		req := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name: "User token",
				Type: structs.ACLTypeClient,
				Rules: `
node "foo" {
	policy = "read"
}
`,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &req, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// With the token, it should now go through.
	arg.Token = id
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListNodes", &arg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Coordinates) != 1 || resp.Coordinates[0].Node != "foo" {
		t.Fatalf("bad: %#v", resp.Coordinates)
	}
}

func TestCoordinate_Node(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"foo", "bar"}
	if err := registerNodes(nodes, codec); err != nil {
		t.Fatal(err)
	}

	// Send coordinate updates for each node.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      generateRandomCoordinate(),
	}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now query back for a specific node (make sure we only get coordinates for foo).
	retry.Run(t, func(r *retry.R) {
		arg := structs.NodeSpecificRequest{
			Node:       "foo",
			Datacenter: "dc1",
		}
		resp := structs.IndexedCoordinates{}
		if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp); err != nil {
			r.Fatalf("err: %v", err)
		}
		if len(resp.Coordinates) != 1 ||
			resp.Coordinates[0].Node != "foo" {
			r.Fatalf("bad: %v", resp.Coordinates)
		}
		verify.Values(t, "", resp.Coordinates[0].Coord, arg1.Coord) // foo
	})
}

func TestCoordinate_Node_ACLDeny(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		c.ACLEnforceVersion8 = false
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec); err != nil {
		t.Fatal(err)
	}

	coord := generateRandomCoordinate()
	req := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      coord,
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try a read for the first node. This should go through since we
	// don't have version 8 ACLs enforced yet.
	arg := structs.NodeSpecificRequest{
		Node:       "node1",
		Datacenter: "dc1",
	}
	resp := structs.IndexedCoordinates{}
	retry.Run(t, func(r *retry.R) {
		if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp); err != nil {
			r.Fatalf("err: %v", err)
		}
		if len(resp.Coordinates) != 1 ||
			resp.Coordinates[0].Node != "node1" {
			r.Fatalf("bad: %v", resp.Coordinates)
		}
		verify.Values(t, "", resp.Coordinates[0].Coord, coord)
	})

	// Now turn on version 8 enforcement and try again.
	s1.config.ACLEnforceVersion8 = true
	err := msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Create an ACL that can read from the node.
	aclReq := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTypeClient,
			Rules: `
node "node1" {
	policy = "read"
}
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &aclReq, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// With the token, it should now go through.
	arg.Token = id
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// But it should be blocked for the other node.
	arg.Node = "node2"
	err = msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func registerNodes(nodes []string, codec rpc.ClientCodec) error {
	for _, node := range nodes {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       node,
			Address:    "127.0.0.1",
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
			return err
		}
	}

	return nil
}
