package consul

import (
	"math/rand"
	"os"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/serf/coordinate"
)

func getRandomCoordinate() *coordinate.Coordinate {
	config := coordinate.DefaultConfig()
	coord := coordinate.NewCoordinate(config)
	for i := 0; i < len(coord.Vec); i++ {
		coord.Vec[i] = rand.Float64()
	}
	return coord
}

func coordinatesEqual(a, b *coordinate.Coordinate) bool {
	config := coordinate.DefaultConfig()
	client := coordinate.NewClient(config)
	dist, err := client.DistanceBetween(a, b)
	if err != nil {
		panic(err)
	}
	return dist < 0.00001
}

func TestCoordinate_Update(t *testing.T) {
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
	if coordinatesEqual(d.Coord, arg.Coord) {
		t.Fatalf("should be equal\n%v\n%v", d.Coord, arg.Coord)
	}
}
