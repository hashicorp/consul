// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package selectiontracker

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	workloadData = &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "198.18.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {
				Port:     8080,
				Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
	}
)

func TestRemoveIDFromTreeAtPaths(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)

	tree := radix.New[[]controller.Request]()

	toRemove := rtest.Resource(types.ServiceEndpointsType, "blah", registry).ID()
	other1 := rtest.Resource(types.ServiceEndpointsType, "other1", registry).ID()
	other2 := rtest.Resource(types.ServiceEndpointsType, "other1", registry).ID()

	// we are trying to create a tree such that removal of the toRemove id causes a
	// few things to happen.
	//
	// * All the slice modification conditions are executed
	//   - removal from beginning of the list
	//   - removal from the end of the list
	//   - removal of only element in the list
	//   - removal from middle of the list
	// * Paths without matching ids are ignored

	notMatching := []controller.Request{
		{ID: other1},
		{ID: other2},
	}

	matchAtBeginning := []controller.Request{
		{ID: toRemove},
		{ID: other1},
		{ID: other2},
	}

	matchAtEnd := []controller.Request{
		{ID: other1},
		{ID: other2},
		{ID: toRemove},
	}

	matchInMiddle := []controller.Request{
		{ID: other1},
		{ID: toRemove},
		{ID: other2},
	}

	matchOnly := []controller.Request{
		{ID: toRemove},
	}

	tree.Insert("no-match", notMatching)
	tree.Insert("match-beginning", matchAtBeginning)
	tree.Insert("match-end", matchAtEnd)
	tree.Insert("match-middle", matchInMiddle)
	tree.Insert("match-only", matchOnly)

	removeIDFromTreeAtPaths(tree, toRemove, []string{
		"no-match",
		"match-beginning",
		"match-end",
		"match-middle",
		"match-only",
	})

	reqs, found := tree.Get("no-match")
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	reqs, found = tree.Get("match-beginning")
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	reqs, found = tree.Get("match-end")
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	reqs, found = tree.Get("match-middle")
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	// The last tracked request should cause removal from the tree
	_, found = tree.Get("match-only")
	require.False(t, found)
}

type selectionTrackerSuite struct {
	suite.Suite

	rt       controller.Runtime
	registry resource.Registry
	tracker  *WorkloadSelectionTracker

	workloadAPI1 *pbresource.Resource
	workloadWeb1 *pbresource.Resource
	endpointsFoo *pbresource.ID
	endpointsBar *pbresource.ID
}

func (suite *selectionTrackerSuite) SetupTest() {
	suite.tracker = New()
	suite.registry = resource.NewRegistry()
	types.Register(suite.registry)

	suite.workloadAPI1 = rtest.Resource(types.WorkloadType, "api-1", suite.registry).WithData(suite.T(), workloadData).Build()
	suite.workloadWeb1 = rtest.Resource(types.WorkloadType, "web-1", suite.registry).WithData(suite.T(), workloadData).Build()
	suite.endpointsFoo = rtest.Resource(types.ServiceEndpointsType, "foo", suite.registry).ID()
	suite.endpointsBar = rtest.Resource(types.ServiceEndpointsType, "bar", suite.registry).ID()
}

func (suite *selectionTrackerSuite) requireMappedIDs(workload *pbresource.Resource, ids ...*pbresource.ID) {
	suite.T().Helper()

	reqs, err := suite.tracker.MapWorkload(context.Background(), suite.rt, workload)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), reqs, len(ids))
	for _, id := range ids {
		prototest.AssertContainsElement(suite.T(), reqs, controller.Request{ID: id})
	}
}

func (suite *selectionTrackerSuite) TestMapWorkload_Empty() {
	// If we aren't tracking anything than the default mapping behavior
	// should be to return an empty list of requests.
	suite.requireMappedIDs(suite.workloadAPI1)
}

func (suite *selectionTrackerSuite) TestUntrackID_Empty() {
	// this test has no assertions but mainly is here to prove that things
	// dont explode if this is attempted.
	suite.tracker.UntrackID(suite.endpointsFoo)
}

func (suite *selectionTrackerSuite) TestTrackAndMap_SingleResource_MultipleWorkloadMappings() {
	// This test aims to prove that tracking a resources workload selector and
	// then mapping a workload back to that resource works as expected when the
	// result set is a single resource. This test will ensure that both prefix
	// and exact match criteria are handle correctly and that one resource
	// can be mapped from multiple distinct workloads.

	// associate the foo endpoints with some workloads
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names:    []string{"bar", "api", "web-1"},
		Prefixes: []string{"api-"},
	})

	// Ensure that mappings tracked by prefix work.
	suite.requireMappedIDs(suite.workloadAPI1, suite.endpointsFoo)

	// Ensure that mappings tracked by exact match work.
	suite.requireMappedIDs(suite.workloadWeb1, suite.endpointsFoo)
}

func (suite *selectionTrackerSuite) TestTrackAndMap_MultiResource_SingleWorkloadMapping() {
	// This test aims to prove that multiple resources selecting of a workload
	// will result in multiple requests when mapping that workload.

	// associate the foo endpoints with some workloads
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	// associate the bar endpoints with some workloads
	suite.tracker.TrackIDForSelector(suite.endpointsBar, &pbcatalog.WorkloadSelector{
		Names: []string{"api-1"},
	})

	// now the mapping should return both endpoints resource ids
	suite.requireMappedIDs(suite.workloadAPI1, suite.endpointsFoo, suite.endpointsBar)
}

func (suite *selectionTrackerSuite) TestDuplicateTracking() {
	// This test aims to prove that tracking some ID multiple times doesn't
	// result in multiple requests for the same ID

	// associate the foo endpoints with some workloads 3 times without changing
	// the selection criteria. The second two times should be no-ops
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	// regardless of the number of times tracked we should only see a single request
	suite.requireMappedIDs(suite.workloadAPI1, suite.endpointsFoo)
}

func (suite *selectionTrackerSuite) TestModifyTracking() {
	// This test aims to prove that modifying selection criteria for a resource
	// works as expected. Adding new criteria results in all being tracked.
	// Removal of some criteria does't result in removal of all etc. More or
	// less we want to ensure that updating selection criteria leaves the
	// tracker in a consistent/expected state.

	// track the web-1 workload
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names: []string{"web-1"},
	})

	// ensure that api-1 isn't mapped but web-1 is
	suite.requireMappedIDs(suite.workloadAPI1)
	suite.requireMappedIDs(suite.workloadWeb1, suite.endpointsFoo)

	// now also track the api- prefix
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names:    []string{"web-1"},
		Prefixes: []string{"api-"},
	})

	// ensure that both workloads are mapped appropriately
	suite.requireMappedIDs(suite.workloadAPI1, suite.endpointsFoo)
	suite.requireMappedIDs(suite.workloadWeb1, suite.endpointsFoo)

	// now remove the web tracking
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	// ensure that only api-1 is mapped
	suite.requireMappedIDs(suite.workloadAPI1, suite.endpointsFoo)
	suite.requireMappedIDs(suite.workloadWeb1)
}

func (suite *selectionTrackerSuite) TestRemove() {
	// This test aims to prove that removal of a resource from tracking
	// actually prevents subsequent mapping calls from returning the
	// workload.

	// track the web-1 workload
	suite.tracker.TrackIDForSelector(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names: []string{"web-1"},
	})

	// ensure that api-1 isn't mapped but web-1 is
	suite.requireMappedIDs(suite.workloadWeb1, suite.endpointsFoo)

	// untrack the resource
	suite.tracker.UntrackID(suite.endpointsFoo)

	// ensure that we no longer map the previous workload to the resource
	suite.requireMappedIDs(suite.workloadWeb1)
}

func TestWorkloadSelectionSuite(t *testing.T) {
	suite.Run(t, new(selectionTrackerSuite))
}
