// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselectionmapper

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/common"
	"github.com/hashicorp/consul/internal/resource/mappers/selectiontracker"
	"github.com/hashicorp/consul/lib/stringslice"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// WorkloadSelecting denotes a resource type that uses workload selectors.
type WorkloadSelecting interface {
	proto.Message
	GetWorkloads() *pbcatalog.WorkloadSelector
}

type Mapper[T WorkloadSelecting] struct {
	workloadSelectionTracker *selectiontracker.WorkloadSelectionTracker
	computedType             *pbresource.Type
}

func New[T WorkloadSelecting](computedType *pbresource.Type) *Mapper[T] {
	if computedType == nil {
		panic("computed type is required")
	}
	return &Mapper[T]{
		workloadSelectionTracker: selectiontracker.New(),
		computedType:             computedType,
	}
}

// MapToComputedType is responsible for mapping types with workload selectors to the corresponding computed type
// resources which are name-aligned with the workload. This function will also track workload selectors with the ids
// from the workload-selectable types in the mapper.
func (m *Mapper[T]) MapToComputedType(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var zero T
	data := zero.ProtoReflect().New().Interface().(T)
	// check that there is data to unmarshall
	if res.Data != nil {
		if err := res.Data.UnmarshalTo(data); err != nil {
			return nil, err
		}
	}

	// First, we return any existing workloads that this proxy configuration selects.
	// The number of selected workloads may change in the future, but for this even we
	// only need to care about triggering reconcile requests for the current ones.
	requests, err := common.MapSelector(ctx, rt.Client, m.computedType,
		data.GetWorkloads(), res.Id.Tenancy)
	if err != nil {
		return nil, err
	}

	// Then generate requests for any previously selected workloads.
	prevSelector := m.workloadSelectionTracker.GetSelector(res.GetId())

	if !(stringslice.Equal(prevSelector.GetNames(), data.GetWorkloads().GetNames()) &&
		stringslice.Equal(prevSelector.GetPrefixes(), data.GetWorkloads().GetPrefixes())) {
		// the selector is different, so we need to map those selectors as well.
		requestsForPrevSelector, err := common.MapSelector(ctx, rt.Client, m.computedType,
			prevSelector, res.Id.Tenancy)
		if err != nil {
			return nil, err
		}
		requests = append(requests, requestsForPrevSelector...)
	}

	// Second, we track this proxy configuration's selector and ID in the tracker.
	m.workloadSelectionTracker.TrackIDForSelector(res.Id, data.GetWorkloads())

	return requests, nil
}

// IDsForWorkload returns IDs of workload-selecting types that we're tracking for the
// given workload name.
func (m *Mapper[T]) IDsForWorkload(name string) []*pbresource.ID {
	return m.workloadSelectionTracker.GetIDsForName(name)
}

// UntrackID removes tracking for the workload-selecting resource with the given ID.
func (m *Mapper[T]) UntrackID(id *pbresource.ID) {
	m.workloadSelectionTracker.UntrackID(id)
}
