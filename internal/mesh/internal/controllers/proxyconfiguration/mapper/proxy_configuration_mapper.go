// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/common"
	"github.com/hashicorp/consul/internal/resource/mappers/selectiontracker"
	"github.com/hashicorp/consul/lib/stringslice"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Mapper struct {
	workloadSelectionTracker *selectiontracker.WorkloadSelectionTracker
}

func New() *Mapper {
	return &Mapper{
		workloadSelectionTracker: selectiontracker.New(),
	}
}

// MapProxyConfiguration is responsible for mapping ProxyConfiguration resources to the corresponding ComputedProxyConfiguration
// resource which are name-aligned with the workload.
func (m *Mapper) MapProxyConfiguration(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var proxyConfig pbmesh.ProxyConfiguration
	err := res.Data.UnmarshalTo(&proxyConfig)
	if err != nil {
		return nil, err
	}

	// First, we return any existing workloads that this proxy configuration selects.
	// The number of selected workloads may change in the future, but for this even we
	// only need to care about triggering reconcile requests for the current ones.
	requests, err := common.MapSelector(ctx, rt.Client, pbmesh.ComputedProxyConfigurationType,
		proxyConfig.GetWorkloads(), res.Id.Tenancy)
	if err != nil {
		return nil, err
	}

	// Then generate requests for any previously selected workloads.
	prevSelector := m.workloadSelectionTracker.GetSelector(res.GetId())

	if !(stringslice.Equal(prevSelector.GetNames(), proxyConfig.GetWorkloads().GetNames()) &&
		stringslice.Equal(prevSelector.GetPrefixes(), proxyConfig.GetWorkloads().GetPrefixes())) {
		// the selector is different, so we need to map those selectors as well.
		requestsForPrevSelector, err := common.MapSelector(ctx, rt.Client, pbmesh.ComputedProxyConfigurationType,
			prevSelector, res.Id.Tenancy)
		if err != nil {
			return nil, err
		}
		requests = append(requests, requestsForPrevSelector...)
	}

	// Second, we track this proxy configuration's selector and ID in the tracker.
	m.workloadSelectionTracker.TrackIDForSelector(res.Id, proxyConfig.GetWorkloads())

	return requests, nil
}

func (m *Mapper) ProxyConfigurationsForWorkload(name string) []*pbresource.ID {
	return m.workloadSelectionTracker.GetIDsForName(name)
}

func (m *Mapper) UntrackProxyConfiguration(id *pbresource.ID) {
	m.workloadSelectionTracker.UntrackID(id)
}
