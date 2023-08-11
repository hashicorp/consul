// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failovermapper

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Mapper tracks the relationship between a FailoverPolicy an a Service it
// references whether due to name-alignment or from a reference in a
// FailoverDestination leg.
type Mapper struct {
	b *bimapper.Mapper
}

// New creates a new Mapper.
func New() *Mapper {
	return &Mapper{
		b: bimapper.New(types.FailoverPolicyType, types.ServiceType),
	}
}

// TrackFailover extracts all Service references from the provided
// FailoverPolicy and indexes them so that MapService can turn Service events
// into FailoverPolicy events properly.
func (m *Mapper) TrackFailover(failover *resource.DecodedResource[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy]) {
	destRefs := failover.Data.GetUnderlyingDestinationRefs()
	destRefs = append(destRefs, &pbresource.Reference{
		Type:    types.ServiceType,
		Tenancy: failover.Resource.Id.Tenancy,
		Name:    failover.Resource.Id.Name,
	})
	m.trackFailover(failover.Resource.Id, destRefs)
}

func (m *Mapper) trackFailover(failover *pbresource.ID, services []*pbresource.Reference) {
	var servicesAsIDsOrRefs []resource.ReferenceOrID
	for _, s := range services {
		servicesAsIDsOrRefs = append(servicesAsIDsOrRefs, s)
	}
	m.b.TrackItem(failover, servicesAsIDsOrRefs)
}

// UntrackFailover forgets the links inserted by TrackFailover for the provided
// FailoverPolicyID.
func (m *Mapper) UntrackFailover(failoverID *pbresource.ID) {
	m.b.UntrackItem(failoverID)
}

func (m *Mapper) MapService(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return m.b.MapLink(ctx, rt, res)
}

func (m *Mapper) FailoverIDsByService(svcID *pbresource.ID) []*pbresource.ID {
	return m.b.ItemsForLink(svcID)
}
