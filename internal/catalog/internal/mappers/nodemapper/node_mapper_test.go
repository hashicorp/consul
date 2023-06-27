package nodemapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestNodeMapper_NodeIDFromWorkload(t *testing.T) {
	mapper := New()

	data := &pbcatalog.Workload{
		NodeName: "test-node",
		// the other fields should be irrelevant
	}

	workload := resourcetest.Resource(types.WorkloadType, "test-workload").
		WithData(t, data).Build()

	actual := mapper.NodeIDFromWorkload(workload, data)
	expected := &pbresource.ID{
		Type:    types.NodeType,
		Tenancy: workload.Id.Tenancy,
		Name:    "test-node",
	}

	prototest.AssertDeepEqual(t, expected, actual)
}

func requireWorkloadsTracked(t *testing.T, mapper *NodeMapper, node *pbresource.Resource, workloads ...*pbresource.ID) {
	t.Helper()
	reqs, err := mapper.MapNodeToWorkloads(
		context.Background(),
		controller.Runtime{},
		node)

	require.NoError(t, err)
	require.Len(t, reqs, len(workloads))
	for _, workload := range workloads {
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: workload})
	}
}

func TestNodeMapper_WorkloadTracking(t *testing.T) {
	mapper := New()

	node1 := resourcetest.Resource(types.NodeType, "node1").
		WithData(t, &pbcatalog.Node{Addresses: []*pbcatalog.NodeAddress{{Host: "198.18.0.1"}}}).
		Build()

	node2 := resourcetest.Resource(types.NodeType, "node2").
		WithData(t, &pbcatalog.Node{Addresses: []*pbcatalog.NodeAddress{{Host: "198.18.0.2"}}}).
		Build()

	tenant := &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}

	workload1 := &pbresource.ID{Type: types.WorkloadType, Tenancy: tenant, Name: "workload1"}
	workload2 := &pbresource.ID{Type: types.WorkloadType, Tenancy: tenant, Name: "workload2"}
	workload3 := &pbresource.ID{Type: types.WorkloadType, Tenancy: tenant, Name: "workload3"}
	workload4 := &pbresource.ID{Type: types.WorkloadType, Tenancy: tenant, Name: "workload4"}
	workload5 := &pbresource.ID{Type: types.WorkloadType, Tenancy: tenant, Name: "workload5"}

	// No Workloads have been tracked so the mapper should return empty lists
	requireWorkloadsTracked(t, mapper, node1)
	requireWorkloadsTracked(t, mapper, node2)
	// As nothing is tracked these should be pretty much no-ops
	mapper.UntrackWorkload(workload1)
	mapper.UntrackWorkload(workload2)
	mapper.UntrackWorkload(workload2)
	mapper.UntrackWorkload(workload3)
	mapper.UntrackWorkload(workload4)
	mapper.UntrackWorkload(workload5)

	// Now track some workloads
	mapper.TrackWorkload(workload1, node1.Id)
	mapper.TrackWorkload(workload2, node1.Id)
	mapper.TrackWorkload(workload3, node2.Id)
	mapper.TrackWorkload(workload4, node2.Id)

	// Mapping should now return 2 workload requests for each node
	requireWorkloadsTracked(t, mapper, node1, workload1, workload2)
	requireWorkloadsTracked(t, mapper, node2, workload3, workload4)

	// Track the same workloads again, this should end up being mostly a no-op
	mapper.TrackWorkload(workload1, node1.Id)
	mapper.TrackWorkload(workload2, node1.Id)
	mapper.TrackWorkload(workload3, node2.Id)
	mapper.TrackWorkload(workload4, node2.Id)

	// Mappings should be unchanged from the initial workload tracking
	requireWorkloadsTracked(t, mapper, node1, workload1, workload2)
	requireWorkloadsTracked(t, mapper, node2, workload3, workload4)

	// Change the workload association for workload2
	mapper.TrackWorkload(workload2, node2.Id)

	// Node1 should now track just the single workload and node2 should track 3
	requireWorkloadsTracked(t, mapper, node1, workload1)
	requireWorkloadsTracked(t, mapper, node2, workload2, workload3, workload4)

	// Untrack the workloads - this is done in very specific ordering to ensure all
	// the workload tracking removal paths get hit. This does assume that the ordering
	// of requests is stable between removals.

	// remove the one and only workload from a node
	mapper.UntrackWorkload(workload1)
	requireWorkloadsTracked(t, mapper, node1)

	// track an additional workload
	mapper.TrackWorkload(workload5, node2.Id)
	reqs, err := mapper.MapNodeToWorkloads(context.Background(), controller.Runtime{}, node2)
	require.NoError(t, err)
	require.Len(t, reqs, 4)

	first := reqs[0].ID
	second := reqs[1].ID
	third := reqs[2].ID
	fourth := reqs[3].ID

	// remove from the middle of the request list
	mapper.UntrackWorkload(second)
	requireWorkloadsTracked(t, mapper, node2, first, third, fourth)

	// remove from the end of the list
	mapper.UntrackWorkload(fourth)
	requireWorkloadsTracked(t, mapper, node2, first, third)

	// remove from the beginning of the list
	mapper.UntrackWorkload(first)
	requireWorkloadsTracked(t, mapper, node2, third)

	// remove the last element
	mapper.UntrackWorkload(third)
	requireWorkloadsTracked(t, mapper, node2)
}
