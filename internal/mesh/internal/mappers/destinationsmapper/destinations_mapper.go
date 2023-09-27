package destinationsmapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/common"
	"github.com/hashicorp/consul/internal/resource/mappers/selectiontracker"
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

// MapDestinations is responsible for mapping Destinations resources to the corresponding ComputedDestinations
// resource which are name-aligned with the workload.
func (m *Mapper) MapDestinations(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var destinations pbmesh.Destinations
	err := res.Data.UnmarshalTo(&destinations)
	if err != nil {
		return nil, err
	}

	// First, we return any existing workloads that this destinations resource selects.
	// The number of selected workloads may change in the future, but for this even we
	// only need to care about triggering reconcile requests for the current ones.
	requests, err := common.MapSelector(ctx, rt.Client, pbmesh.ComputedDestinationsType,
		destinations.GetWorkloads(), res.Id.Tenancy)
	if err != nil {
		return nil, err
	}

	// Track this proxy configuration's selector and ID in the tracker.
	m.workloadSelectionTracker.TrackIDForSelector(res.Id, destinations.GetWorkloads())

	return requests, nil
}

func (m *Mapper) DestinationsForWorkload(name string) []*pbresource.ID {
	return m.workloadSelectionTracker.GetIDsForName(name)
}

func (m *Mapper) UntrackDestinations(id *pbresource.ID) {
	m.workloadSelectionTracker.UntrackID(id)
}
