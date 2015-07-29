package consul

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/serf/coordinate"
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

// verifyCoordinatesEqual will compare a and b and fail if they are not exactly
// equal (no floating point fuzz is considered since we are trying to make sure
// we are getting exactly the coordinates we expect, without math on them).
func verifyCoordinatesEqual(t *testing.T, a, b *coordinate.Coordinate) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("coordinates are not equal: %v != %v", a, b)
	}
}

func TestCoordinate_Update(t *testing.T) {
	name := fmt.Sprintf("Node %d", getPort())
	dir1, config1 := testServerConfig(t, name)
	defer os.RemoveAll(dir1)

	config1.CoordinateUpdatePeriod = 500 * time.Millisecond
	config1.CoordinateUpdateBatchSize = 5
	config1.CoordinateUpdateMaxBatches = 2
	s1, err := NewServer(config1)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Shutdown()

	client := rpcClient(t, s1)
	defer client.Close()
	testutil.WaitForLeader(t, client.Call, "dc1")

	// Send an update for the first node.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Send an update for the second node.
	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node2",
		Coord:      generateRandomCoordinate(),
	}
	if err := client.Call("Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the updates did not yet apply because the update period
	// hasn't expired.
	state := s1.fsm.State()
	_, c, err := state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c != nil {
		t.Fatalf("should be nil because the update should be batched")
	}
	_, c, err = state.CoordinateGet("node2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c != nil {
		t.Fatalf("should be nil because the update should be batched")
	}

	// Wait a while and the updates should get picked up.
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)
	_, c, err = state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	verifyCoordinatesEqual(t, c, arg1.Coord)
	_, c, err = state.CoordinateGet("node2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	verifyCoordinatesEqual(t, c, arg2.Coord)

	// Now spam some coordinate updates and make sure it starts throwing
	// them away if they exceed the batch allowance. Note we have to make
	// unique names since these are held in map by node name.
	spamLen := s1.config.CoordinateUpdateBatchSize*s1.config.CoordinateUpdateMaxBatches + 1
	for i := 0; i < spamLen; i++ {
		arg1.Node = fmt.Sprintf("bogusnode%d", i)
		arg1.Coord = generateRandomCoordinate()
		if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait a little while for the batch routine to run, then make sure
	// exactly one of the updates got dropped (we won't know which one).
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)
	numDropped := 0
	for i := 0; i < spamLen; i++ {
		_, c, err = state.CoordinateGet(fmt.Sprintf("bogusnode%d", i))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if c == nil {
			numDropped++
		}
	}
	if numDropped != 1 {
		t.Fatalf("wrong number of coordinates dropped, %d != 1", numDropped)
	}

	// Finally, send a coordinate with the wrong dimensionality to make sure
	// there are no panics, and that it gets rejected.
	arg2.Coord.Vec = make([]float64, 2*len(arg2.Coord.Vec))
	err = client.Call("Coordinate.Update", &arg2, &out)
	if err == nil || !strings.Contains(err.Error(), "rejected bad coordinate") {
		t.Fatalf("should have failed with an error, got %v", err)
	}
}

func TestCoordinate_Get(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	client := rpcClient(t, s1)
	defer client.Close()
	testutil.WaitForLeader(t, client.Call, "dc1")

	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Coord:      generateRandomCoordinate(),
	}

	// Send an initial update, waiting a little while for the batch update
	// to run.
	var out struct{}
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)

	// Query the coordinate via RPC.
	arg2 := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "node1",
	}
	coord := structs.IndexedCoordinate{}
	if err := client.Call("Coordinate.Get", &arg2, &coord); err != nil {
		t.Fatalf("err: %v", err)
	}
	verifyCoordinatesEqual(t, coord.Coord, arg.Coord)

	// Send another coordinate update, waiting after for the flush.
	arg.Coord = generateRandomCoordinate()
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)

	// Now re-query and make sure the results are fresh.
	if err := client.Call("Coordinate.Get", &arg2, &coord); err != nil {
		t.Fatalf("err: %v", err)
	}
	verifyCoordinatesEqual(t, coord.Coord, arg.Coord)
}

func TestCoordinate_ListDatacenters(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	// It's super hard to force the Serfs into a known configuration of
	// coordinates, so the best we can do is make sure our own DC shows
	// up in the list with the proper coordinates. The guts of the algorithm
	// are extensively tested in rtt_test.go using a mock database.
	var out []structs.DatacenterMap
	if err := client.Call("Coordinate.ListDatacenters", struct{}{}, &out); err != nil {
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
	verifyCoordinatesEqual(t, c, out[0].Coordinates[0].Coord)
}

func TestCoordinate_ListNodes(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	client := rpcClient(t, s1)
	defer client.Close()
	testutil.WaitForLeader(t, client.Call, "dc1")

	// Send coordinate updates for a few nodes, waiting a little while for
	// the batch update to run.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      generateRandomCoordinate(),
	}
	var out struct{}
	if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      generateRandomCoordinate(),
	}
	if err := client.Call("Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg3 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "baz",
		Coord:      generateRandomCoordinate(),
	}
	if err := client.Call("Coordinate.Update", &arg3, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)

	// Now query back for all the nodes and make sure they are sorted
	// properly.
	arg := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	resp := structs.IndexedCoordinates{}
	if err := client.Call("Coordinate.ListNodes", &arg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Coordinates) != 3 ||
		resp.Coordinates[0].Node != "bar" ||
		resp.Coordinates[1].Node != "baz" ||
		resp.Coordinates[2].Node != "foo" {
		t.Fatalf("bad: %v", resp.Coordinates)
	}
	verifyCoordinatesEqual(t, resp.Coordinates[0].Coord, arg2.Coord) // bar
	verifyCoordinatesEqual(t, resp.Coordinates[1].Coord, arg3.Coord) // baz
	verifyCoordinatesEqual(t, resp.Coordinates[2].Coord, arg1.Coord) // foo
}
