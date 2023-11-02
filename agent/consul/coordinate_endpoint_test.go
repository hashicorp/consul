// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec, ""); err != nil {
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
	_, c, err := state.Coordinate(nil, "node1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.Equal(t, lib.CoordinateSet{}, c)

	_, c, err = state.Coordinate(nil, "node2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.Equal(t, lib.CoordinateSet{}, c)

	// Send another update for the second node. It should take precedence
	// since there will be two updates in the same batch.
	arg2.Coord = generateRandomCoordinate()
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait a while and the updates should get picked up.
	time.Sleep(3 * s1.config.CoordinateUpdatePeriod)
	_, c, err = state.Coordinate(nil, "node1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected := lib.CoordinateSet{
		"": arg1.Coord,
	}
	require.Equal(t, expected, c)

	_, c, err = state.Coordinate(nil, "node2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected = lib.CoordinateSet{
		"": arg2.Coord,
	}
	require.Equal(t, expected, c)

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
		_, c, err = state.Coordinate(nil, fmt.Sprintf("bogusnode%d", i), nil)
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec, "root"); err != nil {
		t.Fatal(err)
	}

	// Send an update for the first node.
	// don't have version 8 ACLs enforced yet.
	req := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	id := createToken(t, codec, `node "node1" { policy = "write" }`)

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
	require.Equal(t, out[0].Coordinates[0].Coord, c)
}

func TestCoordinate_ListNodes(t *testing.T) {
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

	// Register some nodes.
	nodes := []string{"foo", "bar", "baz"}
	if err := registerNodes(nodes, codec, ""); err != nil {
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
		require.Equal(r, arg2.Coord, resp.Coordinates[0].Coord) // bar
		require.Equal(r, arg3.Coord, resp.Coordinates[1].Coord) // baz
		require.Equal(r, arg1.Coord, resp.Coordinates[2].Coord) // foo
	})
}

func TestCoordinate_ListNodes_ACLFilter(t *testing.T) {
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

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

	// Wait for all the coordinate updates to apply.
	retry.Run(t, func(r *retry.R) {
		arg := structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: "root"},
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
	// coordinates we can do the rest of the requests without the loop.
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

	id := createToken(t, codec, ` node "foo" { policy = "read" } `)

	// With the token, it should now go through.
	arg.Token = id
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.ListNodes", &arg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Coordinates) != 1 || resp.Coordinates[0].Node != "foo" {
		t.Fatalf("bad: %#v", resp.Coordinates)
	}
	if !resp.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}
}

func TestCoordinate_Node(t *testing.T) {
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

	// Register some nodes.
	nodes := []string{"foo", "bar"}
	if err := registerNodes(nodes, codec, ""); err != nil {
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
		require.Equal(r, arg1.Coord, resp.Coordinates[0].Coord) // foo
	})
}

func TestCoordinate_Node_ACLDeny(t *testing.T) {
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

	testrpc.WaitForLeader(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	// Register some nodes.
	nodes := []string{"node1", "node2"}
	if err := registerNodes(nodes, codec, "root"); err != nil {
		t.Fatal(err)
	}

	coord := generateRandomCoordinate()
	req := structs.CoordinateUpdateRequest{
		Datacenter:   "dc1",
		Node:         "node1",
		Coord:        coord,
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Coordinate.Update", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try a read for the first node. This should fail without a token.
	arg := structs.NodeSpecificRequest{
		Node:       "node1",
		Datacenter: "dc1",
	}
	resp := structs.IndexedCoordinates{}
	err := msgpackrpc.CallWithCodec(codec, "Coordinate.Node", &arg, &resp)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	id := createToken(t, codec, `node "node1" { policy = "read" } `)

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

func registerNodes(nodes []string, codec rpc.ClientCodec, token string) error {
	for _, node := range nodes {
		req := structs.RegisterRequest{
			Datacenter:   "dc1",
			Node:         node,
			Address:      "127.0.0.1",
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply struct{}
		if err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &req, &reply); err != nil {
			return err
		}
	}

	return nil
}
