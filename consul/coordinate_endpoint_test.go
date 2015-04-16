package consul

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/serf/coordinate"
)

// getRandomCoordinate generates a random coordinate.
func getRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	// Randomly apply updates between n clients
	n := 5
	clients := make([]*coordinate.Client, n)
	for i := 0; i < n; i++ {
		clients[i] = coordinate.NewClient(config)
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
	config := coordinate.DefaultConfig()
	dist, err := a.DistanceTo(b, config)
	if err != nil {
		panic(err)
	}
	return dist < 0.1
}

func TestCoordinate(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	arg := structs.CoordinateUpdateRequest{
		NodeSpecificRequest: structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       "node1",
		},
		Op:    structs.CoordinateSet,
		Coord: getRandomCoordinate(),
	}

	var out struct{}
	if err := client.Call("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	state := s1.fsm.State()
	_, d, err := state.CoordinateGet("node1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !coordinatesEqual(d.Coord, arg.Coord) {
		t.Fatalf("should be equal\n%v\n%v", d.Coord, arg.Coord)
	}

	// Get via RPC
	var out2 *structs.Coordinate
	arg2 := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "node1",
	}
	if err := client.Call("Coordinate.Get", &arg2, &out2); err != nil {
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
	if err := client.Call("Coordinate.Get", &arg2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !coordinatesEqual(out2.Coord, arg.Coord) {
		t.Fatalf("should be equal\n%v\n%v", out2.Coord, arg.Coord)
	}
}
