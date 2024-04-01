// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Mapper struct {
	workloadSelectionMapper *workloadselectionmapper.Mapper[*pbmesh.Destinations]

	serviceRefMapper *bimapper.Mapper
}

func New() *Mapper {
	return &Mapper{
		workloadSelectionMapper: workloadselectionmapper.New[*pbmesh.Destinations](pbmesh.ComputedExplicitDestinationsType),
		serviceRefMapper:        bimapper.New(pbmesh.ComputedExplicitDestinationsType, pbcatalog.ServiceType),
	}
}

func (m *Mapper) MapService(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	serviceRef := resource.Reference(res.GetId(), "")

	compDestinations := m.serviceRefMapper.ItemIDsForLink(serviceRef)

	return controller.MakeRequests(pbmesh.ComputedExplicitDestinationsType, compDestinations), nil
}

func (m *Mapper) MapDestinations(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return m.workloadSelectionMapper.MapToComputedType(ctx, rt, res)
}

func (m *Mapper) MapComputedRoute(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	serviceID := resource.ReplaceType(pbcatalog.ServiceType, res.GetId())
	serviceRef := resource.Reference(serviceID, "")

	compDestinations := m.serviceRefMapper.ItemIDsForLink(serviceRef)

	return controller.MakeRequests(pbmesh.ComputedExplicitDestinationsType, compDestinations), nil
}

func (m *Mapper) TrackDestinations(id *pbresource.ID, destinations []*types.DecodedDestinations) {
	var links []resource.ReferenceOrID
	for _, dst := range destinations {
		for _, d := range dst.GetData().GetDestinations() {
			links = append(links, d.DestinationRef)
		}
	}

	m.serviceRefMapper.TrackItem(id, links)
}

func (m *Mapper) UntrackComputedExplicitDestinations(id *pbresource.ID) {
	m.serviceRefMapper.UntrackItem(id)
}

func (m *Mapper) UntrackDestinations(id *pbresource.ID) {
	m.workloadSelectionMapper.UntrackID(id)
}

func (m *Mapper) DestinationsForWorkload(id *pbresource.ID) []*pbresource.ID {
	return m.workloadSelectionMapper.IDsForWorkload(id)
}
