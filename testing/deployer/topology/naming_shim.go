// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

// Deprecated: SortedWorkloads
func (n *Node) SortedServices() []*Workload {
	return n.SortedWorkloads()
}

// Deprecated: mapifyWorkloads
func mapifyServices(services []*Workload) map[ServiceID]*Workload {
	return mapifyWorkloads(services)
}

// Deprecated: WorkloadByID
func (c *Cluster) ServiceByID(nid NodeID, sid ServiceID) *Workload {
	return c.WorkloadByID(nid, sid)
}

// Deprecated: WorkloadsByID
func (c *Cluster) ServicesByID(sid ServiceID) []*Workload {
	return c.WorkloadsByID(sid)
}

// Deprecated: WorkloadByID
func (n *Node) ServiceByID(sid ServiceID) *Workload {
	return n.WorkloadByID(sid)
}

// Deprecated: Workload
type Service = Workload

// Deprecated: ID
type ServiceID = ID

// Deprecated: NewID
func NewServiceID(name, namespace, partition string) ID {
	return NewID(name, namespace, partition)
}

// Deprecated:
type Destination = Upstream
