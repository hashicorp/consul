package consul

import (
	"fmt"
	"math"
	"sort"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/coordinate"
)

// computeDistance returns the distance between the two network coordinates in
// seconds. If either of the coordinates is nil then this will return positive
// infinity.
func computeDistance(a *coordinate.Coordinate, b *coordinate.Coordinate) float64 {
	if a == nil || b == nil {
		return math.Inf(1.0)
	}

	return a.DistanceTo(b).Seconds()
}

// nodeSorter takes a list of nodes and a parallel vector of distances and
// implements sort.Interface, keeping both structures coherent and sorting by
// distance.
type nodeSorter struct {
	Nodes structs.Nodes
	Vec   []float64
}

// newNodeSorter returns a new sorter for the given source coordinate and set of
// nodes.
func (s *Server) newNodeSorter(c *coordinate.Coordinate, nodes structs.Nodes) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(nodes))
	for i, node := range nodes {
		_, coord, err := state.CoordinateGet(node.Node)
		if err != nil {
			return nil, err
		}
		vec[i] = computeDistance(c, coord)
	}
	return &nodeSorter{nodes, vec}, nil
}

// See sort.Interface.
func (n *nodeSorter) Len() int {
	return len(n.Nodes)
}

// See sort.Interface.
func (n *nodeSorter) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
	n.Vec[i], n.Vec[j] = n.Vec[j], n.Vec[i]
}

// See sort.Interface.
func (n *nodeSorter) Less(i, j int) bool {
	return n.Vec[i] < n.Vec[j]
}

// serviceNodeSorter takes a list of service nodes and a parallel vector of
// distances and implements sort.Interface, keeping both structures coherent and
// sorting by distance.
type serviceNodeSorter struct {
	Nodes structs.ServiceNodes
	Vec   []float64
}

// newServiceNodeSorter returns a new sorter for the given source coordinate and
// set of service nodes.
func (s *Server) newServiceNodeSorter(c *coordinate.Coordinate, nodes structs.ServiceNodes) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(nodes))
	for i, node := range nodes {
		_, coord, err := state.CoordinateGet(node.Node)
		if err != nil {
			return nil, err
		}
		vec[i] = computeDistance(c, coord)
	}
	return &serviceNodeSorter{nodes, vec}, nil
}

// See sort.Interface.
func (n *serviceNodeSorter) Len() int {
	return len(n.Nodes)
}

// See sort.Interface.
func (n *serviceNodeSorter) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
	n.Vec[i], n.Vec[j] = n.Vec[j], n.Vec[i]
}

// See sort.Interface.
func (n *serviceNodeSorter) Less(i, j int) bool {
	return n.Vec[i] < n.Vec[j]
}

// newSorterByDistanceFrom returns a sorter for the given type.
func (s *Server) newSorterByDistanceFrom(c *coordinate.Coordinate, subj interface{}) (sort.Interface, error) {
	switch v := subj.(type) {
	case structs.Nodes:
		return s.newNodeSorter(c, v)
	case structs.ServiceNodes:
		return s.newServiceNodeSorter(c, v)
	default:
		panic(fmt.Errorf("Unhandled type passed to newSorterByDistanceFrom: %#v", subj))
	}
}

// sortByDistanceFrom is used to sort results from our service catalog based on the
// distance (RTT) from the given source node.
func (s *Server) sortByDistanceFrom(source structs.QuerySource, subj interface{}) error {
	// We can't compare coordinates across DCs.
	if source.Datacenter != s.config.Datacenter {
		return nil
	}

	// There won't always be a coordinate for the source node. If there's not
	// one then we can bail out because there's no meaning for the sort.
	state := s.fsm.State()
	_, coord, err := state.CoordinateGet(source.Node)
	if err != nil {
		return err
	}
	if coord == nil {
		return nil
	}

	// Do the Dew!
	sorter, err := s.newSorterByDistanceFrom(coord, subj)
	if err != nil {
		return err
	}
	sort.Stable(sorter)
	return nil
}
