package consul

import (
	"fmt"
	"sort"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// nodeSorter takes a list of nodes and a parallel vector of distances and
// implements sort.Interface, keeping both structures coherent and sorting by
// distance.
type nodeSorter struct {
	Nodes structs.Nodes
	Vec   []float64
}

// newNodeSorter returns a new sorter for the given source coordinate and set of
// nodes.
func (s *Server) newNodeSorter(cs lib.CoordinateSet, nodes structs.Nodes) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(nodes))
	for i, node := range nodes {
		_, other, err := state.Coordinate(node.Node, nil)
		if err != nil {
			return nil, err
		}
		c1, c2 := cs.Intersect(other)
		vec[i] = lib.ComputeDistance(c1, c2)
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
func (s *Server) newServiceNodeSorter(cs lib.CoordinateSet, nodes structs.ServiceNodes) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(nodes))
	for i, node := range nodes {
		_, other, err := state.Coordinate(node.Node, nil)
		if err != nil {
			return nil, err
		}
		c1, c2 := cs.Intersect(other)
		vec[i] = lib.ComputeDistance(c1, c2)
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

// serviceNodeSorter takes a list of health checks and a parallel vector of
// distances and implements sort.Interface, keeping both structures coherent and
// sorting by distance.
type healthCheckSorter struct {
	Checks structs.HealthChecks
	Vec    []float64
}

// newHealthCheckSorter returns a new sorter for the given source coordinate and
// set of health checks with nodes.
func (s *Server) newHealthCheckSorter(cs lib.CoordinateSet, checks structs.HealthChecks) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(checks))
	for i, check := range checks {
		_, other, err := state.Coordinate(check.Node, nil)
		if err != nil {
			return nil, err
		}
		c1, c2 := cs.Intersect(other)
		vec[i] = lib.ComputeDistance(c1, c2)
	}
	return &healthCheckSorter{checks, vec}, nil
}

// See sort.Interface.
func (n *healthCheckSorter) Len() int {
	return len(n.Checks)
}

// See sort.Interface.
func (n *healthCheckSorter) Swap(i, j int) {
	n.Checks[i], n.Checks[j] = n.Checks[j], n.Checks[i]
	n.Vec[i], n.Vec[j] = n.Vec[j], n.Vec[i]
}

// See sort.Interface.
func (n *healthCheckSorter) Less(i, j int) bool {
	return n.Vec[i] < n.Vec[j]
}

// checkServiceNodeSorter takes a list of service nodes and a parallel vector of
// distances and implements sort.Interface, keeping both structures coherent and
// sorting by distance.
type checkServiceNodeSorter struct {
	Nodes structs.CheckServiceNodes
	Vec   []float64
}

// newCheckServiceNodeSorter returns a new sorter for the given source coordinate
// and set of nodes with health checks.
func (s *Server) newCheckServiceNodeSorter(cs lib.CoordinateSet, nodes structs.CheckServiceNodes) (sort.Interface, error) {
	state := s.fsm.State()
	vec := make([]float64, len(nodes))
	for i, node := range nodes {
		_, other, err := state.Coordinate(node.Node.Node, nil)
		if err != nil {
			return nil, err
		}
		c1, c2 := cs.Intersect(other)
		vec[i] = lib.ComputeDistance(c1, c2)
	}
	return &checkServiceNodeSorter{nodes, vec}, nil
}

// See sort.Interface.
func (n *checkServiceNodeSorter) Len() int {
	return len(n.Nodes)
}

// See sort.Interface.
func (n *checkServiceNodeSorter) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
	n.Vec[i], n.Vec[j] = n.Vec[j], n.Vec[i]
}

// See sort.Interface.
func (n *checkServiceNodeSorter) Less(i, j int) bool {
	return n.Vec[i] < n.Vec[j]
}

// newSorterByDistanceFrom returns a sorter for the given type.
func (s *Server) newSorterByDistanceFrom(cs lib.CoordinateSet, subj interface{}) (sort.Interface, error) {
	switch v := subj.(type) {
	case structs.Nodes:
		return s.newNodeSorter(cs, v)
	case structs.ServiceNodes:
		return s.newServiceNodeSorter(cs, v)
	case structs.HealthChecks:
		return s.newHealthCheckSorter(cs, v)
	case structs.CheckServiceNodes:
		return s.newCheckServiceNodeSorter(cs, v)
	default:
		panic(fmt.Errorf("Unhandled type passed to newSorterByDistanceFrom: %#v", subj))
	}
}

// sortNodesByDistanceFrom is used to sort results from our service catalog based
// on the round trip time from the given source node. Nodes with missing coordinates
// will get stable sorted at the end of the list.
//
// If coordinates are disabled this will be a no-op.
func (s *Server) sortNodesByDistanceFrom(source structs.QuerySource, subj interface{}) error {
	// We can't sort if there's no source node.
	if source.Node == "" {
		return nil
	}

	// We can't compare coordinates across DCs.
	if source.Datacenter != s.config.Datacenter {
		return nil
	}

	// There won't always be coordinates for the source node. If there are
	// none then we can bail out because there's no meaning for the sort.
	state := s.fsm.State()
	_, cs, err := state.Coordinate(source.Node, nil)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		return nil
	}

	// Do the sort!
	sorter, err := s.newSorterByDistanceFrom(cs, subj)
	if err != nil {
		return err
	}
	sort.Stable(sorter)
	return nil
}
