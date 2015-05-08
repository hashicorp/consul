package consul

import (
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/serf/coordinate"
)

func init() {
	// Shorten updatePeriod so we don't have to wait as long
	updatePeriod = time.Duration(100) * time.Millisecond
}

// getRandomCoordinate generates a random coordinate.
func getRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	// Randomly apply updates between n clients
	n := 5
	clients := make([]*coordinate.Client, n)
	for i := 0; i < n; i++ {
		clients[i], _ = coordinate.NewClient(config)
	}

	for i := 0; i < n*100; i++ {
		k1 := rand.Intn(n)
		k2 := rand.Intn(n)
		if k1 == k2 {
			continue
		}
		clients[k1].Update(clients[k2].GetCoordinate(), time.Duration(rand.Int63())*time.Microsecond)
	}
	return clients[rand.Intn(n)].GetCoordinate()
}

func coordinatesEqual(a, b *coordinate.Coordinate) bool {
	return reflect.DeepEqual(a, b)
}

func TestCoordinateUpdate(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Op:         structs.CoordinateSet,
		Coord:      getRandomCoordinate(),
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node2",
		Op:         structs.CoordinateSet,
		Coord:      getRandomCoordinate(),
	}

	updateLastSent = time.Now()

	var out struct{}
	if err := client.Call("Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	state := s1.fsm.State()
	_, d, err := state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d != nil {
		t.Fatalf("should be nil because the update should be batched")
	}

	// Wait a while and send another update; this time the updates should be sent
	time.Sleep(time.Duration(2) * updatePeriod)
	if err := client.Call("Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	_, d, err = state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	if !coordinatesEqual(d.Coord, arg1.Coord) {
		t.Fatalf("should be equal\n%v\n%v", d.Coord, arg1.Coord)
	}

	_, d, err = state.CoordinateGet("node2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should return a coordinate but it's nil")
	}
	if !coordinatesEqual(d.Coord, arg2.Coord) {
		t.Fatalf("should be equal\n%v\n%v", d.Coord, arg2.Coord)
	}
}

func TestCoordinateGetLAN(t *testing.T) {
	updatePeriod = time.Duration(0) // to make updates instant

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Op:         structs.CoordinateSet,
		Coord:      getRandomCoordinate(),
	}

	var out struct{}
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Get via RPC
	var out2 *structs.IndexedCoordinate
	arg2 := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "node1",
	}
	if err := client.Call("Coordinate.GetLAN", &arg2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !coordinatesEqual(out2.Coord, arg.Coord) {
		t.Fatalf("should be equal\n%v\n%v", out2.Coord, arg.Coord)
	}

	// Now let's override the original coordinate; Coordinate.Get should return
	// the latest coordinate
	arg.Coord = getRandomCoordinate()
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := client.Call("Coordinate.GetLAN", &arg2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !coordinatesEqual(out2.Coord, arg.Coord) {
		t.Fatalf("should be equal\n%v\n%v", out2.Coord, arg.Coord)
	}
}
