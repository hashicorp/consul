// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodemapper

import (
	"context"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type NodeMapper struct {
	b *bimapper.Mapper
}

func New() *NodeMapper {
	return &NodeMapper{
		b: bimapper.New(pbcatalog.WorkloadType, pbcatalog.NodeType),
	}
}

// NodeIDFromWorkload will create a resource ID referencing the Node type with the same tenancy as
// the workload and with the name populated from the workloads NodeName field.
func (m *NodeMapper) NodeIDFromWorkload(workload *pbresource.Resource, workloadData *pbcatalog.Workload) *pbresource.ID {
	return &pbresource.ID{
		Type:    pbcatalog.NodeType,
		Tenancy: workload.Id.Tenancy,
		Name:    workloadData.NodeName,
	}
}

// MapNodeToWorkloads will take a Node resource and return controller requests
// for all Workloads associated with the Node.
func (m *NodeMapper) MapNodeToWorkloads(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	ids := m.b.ItemIDsForLink(res.Id)
	return controller.MakeRequests(pbcatalog.WorkloadType, ids), nil
}

// TrackWorkload instructs the NodeMapper to associate the given workload
// ID with the given node ID.
func (m *NodeMapper) TrackWorkload(workloadID *pbresource.ID, nodeID *pbresource.ID) {
	m.b.TrackItem(workloadID, []resource.ReferenceOrID{
		nodeID,
	})
}

// UntrackWorkload will cause the node mapper to forget about the specified
// workload if it is currently tracking it.
func (m *NodeMapper) UntrackWorkload(workloadID *pbresource.ID) {
	m.b.UntrackItem(workloadID)
}
