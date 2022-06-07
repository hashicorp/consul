package peering

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

// healthSnapshot represents a normalized view of a set of CheckServiceNodes
// meant for easy comparison to aid in differential synchronization
type healthSnapshot struct {
	Nodes map[types.NodeID]*nodeSnapshot
}

type nodeSnapshot struct {
	Node     *structs.Node
	Services map[structs.ServiceID]*serviceSnapshot
}

type serviceSnapshot struct {
	Service *structs.NodeService
	Checks  map[types.CheckID]*structs.HealthCheck
}

func newHealthSnapshot(all []structs.CheckServiceNode, partition, peerName string) *healthSnapshot {
	// For all nodes, services, and checks we override the peer name and
	// partition to be the local partition and local name for the peer.
	for _, instance := range all {
		// For all nodes, services, and checks we override the peer name and partition to be
		// the local partition and local name for the peer.
		instance.Node.PeerName = peerName
		instance.Node.OverridePartition(partition)

		instance.Service.PeerName = peerName
		instance.Service.OverridePartition(partition)

		for _, chk := range instance.Checks {
			chk.PeerName = peerName
			chk.OverridePartition(partition)
		}
	}

	snap := &healthSnapshot{
		Nodes: make(map[types.NodeID]*nodeSnapshot),
	}

	for _, instance := range all {
		if instance.Node.ID == "" {
			panic("TODO(peering): data should always have a node ID")
		}
		nodeSnap, ok := snap.Nodes[instance.Node.ID]
		if !ok {
			nodeSnap = &nodeSnapshot{
				Node:     instance.Node,
				Services: make(map[structs.ServiceID]*serviceSnapshot),
			}
			snap.Nodes[instance.Node.ID] = nodeSnap
		}

		if instance.Service.ID == "" {
			panic("TODO(peering): data should always have a service ID")
		}
		sid := instance.Service.CompoundServiceID()

		svcSnap, ok := nodeSnap.Services[sid]
		if !ok {
			svcSnap = &serviceSnapshot{
				Service: instance.Service,
				Checks:  make(map[types.CheckID]*structs.HealthCheck),
			}
			nodeSnap.Services[sid] = svcSnap
		}

		for _, c := range instance.Checks {
			if c.CheckID == "" {
				panic("TODO(peering): data should always have a check ID")
			}
			svcSnap.Checks[c.CheckID] = c
		}
	}

	return snap
}
