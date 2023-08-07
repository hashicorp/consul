// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodemapper

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type NodeMapper struct {
	lock             sync.Mutex
	nodesToWorkloads map[string][]controller.Request
	workloadsToNodes map[string]string
}

func New() *NodeMapper {
	return &NodeMapper{
		workloadsToNodes: make(map[string]string),
		nodesToWorkloads: make(map[string][]controller.Request),
	}
}

// NodeIDFromWorkload will create a resource ID referencing the Node type with the same tenancy as
// the workload and with the name populated from the workloads NodeName field.
func (m *NodeMapper) NodeIDFromWorkload(workload *pbresource.Resource, workloadData *pbcatalog.Workload) *pbresource.ID {
	return &pbresource.ID{
		Type:    types.NodeType,
		Tenancy: workload.Id.Tenancy,
		Name:    workloadData.NodeName,
	}
}

// MapNodeToWorkloads will take a Node resource and return controller requests
// for all Workloads associated with the Node.
func (m *NodeMapper) MapNodeToWorkloads(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.nodesToWorkloads[res.Id.Name], nil
}

// TrackWorkload instructs the NodeMapper to associate the given workload
// ID with the given node ID.
func (m *NodeMapper) TrackWorkload(workloadID *pbresource.ID, nodeID *pbresource.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if previousNode, found := m.workloadsToNodes[workloadID.Name]; found && previousNode == nodeID.Name {
		return
	} else if found {
		// the node association is being changed
		m.untrackWorkloadFromNode(workloadID, previousNode)
	}

	// Now set up the latest tracking
	m.nodesToWorkloads[nodeID.Name] = append(m.nodesToWorkloads[nodeID.Name], controller.Request{ID: workloadID})
	m.workloadsToNodes[workloadID.Name] = nodeID.Name
}

// UntrackWorkload will cause the node mapper to forget about the specified
// workload if it is currently tracking it.
func (m *NodeMapper) UntrackWorkload(workloadID *pbresource.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	node, found := m.workloadsToNodes[workloadID.Name]
	if !found {
		return
	}
	m.untrackWorkloadFromNode(workloadID, node)
}

// untrackWorkloadFromNode will disassociate the specified workload and node.
// This method will clean up unnecessary tracking entries if the node name
// is no longer associated with any workloads.
func (m *NodeMapper) untrackWorkloadFromNode(workloadID *pbresource.ID, node string) {
	foundIdx := -1
	for idx, req := range m.nodesToWorkloads[node] {
		if resource.EqualID(req.ID, workloadID) {
			foundIdx = idx
			break
		}
	}

	if foundIdx != -1 {
		workloads := m.nodesToWorkloads[node]
		l := len(workloads)

		if l == 1 {
			delete(m.nodesToWorkloads, node)
		} else if foundIdx == l-1 {
			m.nodesToWorkloads[node] = workloads[:foundIdx]
		} else if foundIdx == 0 {
			m.nodesToWorkloads[node] = workloads[1:]
		} else {
			m.nodesToWorkloads[node] = append(workloads[:foundIdx], workloads[foundIdx+1:]...)
		}
	}

	delete(m.workloadsToNodes, workloadID.Name)
}
