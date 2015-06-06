package consul

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
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
// equal (no floating point fuzz is considered).
func verifyCoordinatesEqual(t *testing.T, a, b *coordinate.Coordinate) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("coordinates are not equal: %v != %v", a, b)
	}
}

func TestCoordinate_Update(t *testing.T) {
	name := fmt.Sprintf("Node %d", getPort())
	dir1, config1 := testServerConfig(t, name)
	defer os.RemoveAll(dir1)

	config1.CoordinateUpdatePeriod = 1 * time.Second
	config1.CoordinateUpdateMaxBatchSize = 5
	s1, err := NewServer(config1)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Shutdown()

	client := rpcClient(t, s1)
	defer client.Close()
	testutil.WaitForLeader(t, client.Call, "dc1")

	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Op:         structs.CoordinateUpdate,
		Coord:      generateRandomCoordinate(),
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node2",
		Op:         structs.CoordinateUpdate,
		Coord:      generateRandomCoordinate(),
	}

	// Send an update for the first node.
	var out struct{}
	if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the update did not yet apply because the batching thresholds
	// haven't yet been met.
	state := s1.fsm.State()
	_, d, err := state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("should be nil because the update should be batched")
	}

	// Wait a while and send another update. This time both updates should
	// be applied.
	time.Sleep(2 * s1.config.CoordinateUpdatePeriod)
	if err := client.Call("Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait a little while so the flush goroutine can run, then make sure
	// both coordinates made it in.
	time.Sleep(100 * time.Millisecond)

	_, d, err = state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	verifyCoordinatesEqual(t, d.Coord, arg1.Coord)

	_, d, err = state.CoordinateGet("node2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	verifyCoordinatesEqual(t, d.Coord, arg2.Coord)

	// Now try spamming coordinates and make sure it flushes when the batch
	// size is hit.
	for i := 0; i < (s1.config.CoordinateUpdateMaxBatchSize + 1); i++ {
		arg1.Coord = generateRandomCoordinate()
		if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Wait a little while so the flush goroutine can run, then make sure
	// the last coordinate update made it in.
	time.Sleep(100 * time.Millisecond)

	_, d, err = state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	verifyCoordinatesEqual(t, d.Coord, arg1.Coord)
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
		Op:         structs.CoordinateUpdate,
		Coord:      generateRandomCoordinate(),
	}

	// Send an initial update, waiting a little while for the flush goroutine
	// to run.
	var out struct{}
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

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
	time.Sleep(100 * time.Millisecond)

	// Now re-query and make sure the results are fresh.
	if err := client.Call("Coordinate.Get", &arg2, &coord); err != nil {
		t.Fatalf("err: %v", err)
	}
	verifyCoordinatesEqual(t, coord.Coord, arg.Coord)
}
