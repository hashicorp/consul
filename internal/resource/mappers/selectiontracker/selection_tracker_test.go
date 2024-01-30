// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package selectiontracker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
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

	tenancyCases = map[string]*pbresource.Tenancy{
		"default": resource.DefaultNamespacedTenancy(),
		"bar ns, default partition, local peer": {
			Partition: "default",
			Namespace: "bar",
		},
		"default ns, baz partition, local peer": {
			Partition: "baz",
			Namespace: "default",
		},
		"bar ns, baz partition, local peer": {
			Partition: "baz",
			Namespace: "bar",
		},
	}
)

func TestRemoveIDFromTreeAtPaths(t *testing.T) {
	tree := radix.New[[]*pbresource.ID]()

	tenancy := resource.DefaultNamespacedTenancy()
	toRemove := rtest.Resource(pbcatalog.ServiceEndpointsType, "blah").WithTenancy(tenancy).ID()
	other1 := rtest.Resource(pbcatalog.ServiceEndpointsType, "other1").WithTenancy(tenancy).ID()
	other2 := rtest.Resource(pbcatalog.ServiceEndpointsType, "other2").WithTenancy(tenancy).ID()

	// we are trying to create a tree such that removal of the toRemove id causes a
	// few things to happen.
	//
	// * All the slice modification conditions are executed
	//   - removal from beginning of the list
	//   - removal from the end of the list
	//   - removal of only element in the list
	//   - removal from middle of the list
	// * Paths without matching ids are ignored

	notMatching := []*pbresource.ID{
		other1,
		other2,
	}

	matchAtBeginning := []*pbresource.ID{
		toRemove,
		other1,
		other2,
	}

	// For this case, we only add one other not matching to test that we don't remove
	// non-matching ones.
	matchAtEnd := []*pbresource.ID{
		other1,
		toRemove,
	}

	matchInMiddle := []*pbresource.ID{
		other1,
		toRemove,
		other2,
	}

	matchOnly := []*pbresource.ID{
		toRemove,
	}

	noMatchKey := treePathFromNameOrPrefix(tenancy, "no-match")
	matchBeginningKey := treePathFromNameOrPrefix(tenancy, "match-beginning")
	matchEndKey := treePathFromNameOrPrefix(tenancy, "match-end")
	matchMiddleKey := treePathFromNameOrPrefix(tenancy, "match-middle")
	matchOnlyKey := treePathFromNameOrPrefix(tenancy, "match-only")

	tree.Insert(noMatchKey, notMatching)
	tree.Insert(matchBeginningKey, matchAtBeginning)
	tree.Insert(matchEndKey, matchAtEnd)
	tree.Insert(matchMiddleKey, matchInMiddle)
	tree.Insert(matchOnlyKey, matchOnly)

	removeIDFromTreeAtPaths(tree, toRemove, []string{
		noMatchKey,
		matchBeginningKey,
		matchEndKey,
		matchMiddleKey,
		matchOnlyKey,
	})

	reqs, found := tree.Get(noMatchKey)
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	reqs, found = tree.Get(matchBeginningKey)
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	reqs, found = tree.Get(matchEndKey)
	require.True(t, found)
	require.Equal(t, []*pbresource.ID{other1}, reqs)

	reqs, found = tree.Get(matchMiddleKey)
	require.True(t, found)
	require.Equal(t, notMatching, reqs)

	// The last tracked request should cause removal from the tree
	_, found = tree.Get(matchOnlyKey)
	require.False(t, found)
}

type selectionTrackerSuite struct {
	suite.Suite

	rt      controller.Runtime
	tracker *WorkloadSelectionTracker

	// The test setup adds resources with various tenancy settings. Because tenancies are stored in a map,
	// it adds "free" order randomization, and so we need to remember the order in which those tenancy were executed.
	executedTenancies []*pbresource.Tenancy

	endpointsFoo []*pbresource.ID
	endpointsBar []*pbresource.ID
	workloadsAPI []*pbresource.Resource
	workloadsWeb []*pbresource.Resource
}

func (suite *selectionTrackerSuite) SetupTest() {
	suite.tracker = New()

	for _, tenancy := range tenancyCases {
		suite.executedTenancies = append(suite.executedTenancies, tenancy)

		endpointsFooID := rtest.Resource(pbcatalog.ServiceEndpointsType, "foo").
			WithTenancy(tenancy).ID()
		suite.endpointsFoo = append(suite.endpointsFoo, endpointsFooID)

		endpointsBarID := rtest.Resource(pbcatalog.ServiceEndpointsType, "bar").
			WithTenancy(tenancy).ID()
		suite.endpointsBar = append(suite.endpointsBar, endpointsBarID)

		suite.workloadsAPI = append(suite.workloadsAPI, rtest.Resource(pbcatalog.WorkloadType, "api-1").
			WithData(suite.T(), workloadData).
			WithTenancy(tenancy).
			Build())
		suite.workloadsWeb = append(suite.workloadsWeb, rtest.Resource(pbcatalog.WorkloadType, "web-1").
			WithData(suite.T(), workloadData).
			WithTenancy(tenancy).
			Build())
	}
}

func (suite *selectionTrackerSuite) TearDownTest() {
	suite.executedTenancies = nil
	suite.workloadsAPI = nil
	suite.workloadsWeb = nil
	suite.endpointsFoo = nil
	suite.endpointsBar = nil
}

func (suite *selectionTrackerSuite) requireMappedIDs(t *testing.T, workload *pbresource.Resource, ids ...*pbresource.ID) {
	t.Helper()

	reqs, err := suite.tracker.MapWorkload(context.Background(), suite.rt, workload)
	require.NoError(suite.T(), err)
	require.Len(t, reqs, len(ids))
	for _, id := range ids {
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: id})
	}
}

func (suite *selectionTrackerSuite) requireMappedIDsAllTenancies(t *testing.T, workloads []*pbresource.Resource, ids ...[]*pbresource.ID) {
	t.Helper()

	for i := range suite.executedTenancies {
		reqs, err := suite.tracker.MapWorkload(context.Background(), suite.rt, workloads[i])
		require.NoError(suite.T(), err)
		require.Len(t, reqs, len(ids))
		for _, id := range ids {
			prototest.AssertContainsElement(t, reqs, controller.Request{ID: id[i]})
		}
	}
}

func (suite *selectionTrackerSuite) trackIDForSelectorInAllTenancies(ids []*pbresource.ID, selector *pbcatalog.WorkloadSelector) {
	suite.T().Helper()

	for i := range suite.executedTenancies {
		suite.tracker.TrackIDForSelector(ids[i], selector)
	}
}

func (suite *selectionTrackerSuite) TestMapWorkload_Empty() {
	// If we aren't tracking anything than the default mapping behavior
	// should be to return an empty list of requests.
	suite.requireMappedIDs(suite.T(),
		rtest.Resource(pbcatalog.WorkloadType, "api-1").WithData(suite.T(), workloadData).Build())
}

func (suite *selectionTrackerSuite) TestUntrackID_Empty() {
	// this test has no assertions but mainly is here to prove that things
	// don't explode if this is attempted.
	suite.tracker.UntrackID(rtest.Resource(pbcatalog.ServiceEndpointsType, "foo").ID())
}

func (suite *selectionTrackerSuite) TestTrackAndMap_SingleResource_MultipleWorkloadMappings() {
	// This test aims to prove that tracking a resources workload selector and
	// then mapping a workload back to that resource works as expected when the
	// result set is a single resource. This test will ensure that both prefix
	// and exact match criteria are handle correctly and that one resource
	// can be mapped from multiple distinct workloads.

	// Create resources for the test and track endpoints.
	suite.trackIDForSelectorInAllTenancies(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names:    []string{"bar", "api", "web-1"},
		Prefixes: []string{"api-"},
	})

	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsAPI, suite.endpointsFoo)
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb, suite.endpointsFoo)
}

func (suite *selectionTrackerSuite) TestTrackAndMap_MultiResource_SingleWorkloadMapping() {
	// This test aims to prove that multiple resources selecting of a workload
	// will result in multiple requests when mapping that workload.

	cases := map[string]struct {
		selector *pbcatalog.WorkloadSelector
	}{
		"names": {
			selector: &pbcatalog.WorkloadSelector{
				Names: []string{"api-1"},
			},
		},
		"prefixes": {
			selector: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api"},
			},
		},
	}

	for name, c := range cases {
		suite.T().Run(name, func(t *testing.T) {
			for i := range suite.executedTenancies {
				// associate the foo endpoints with some workloads
				suite.tracker.TrackIDForSelector(suite.endpointsFoo[i], c.selector)

				// associate the bar endpoints with some workloads
				suite.tracker.TrackIDForSelector(suite.endpointsBar[i], c.selector)
			}

			// now the mapping should return both endpoints resource ids
			suite.requireMappedIDsAllTenancies(t, suite.workloadsAPI, suite.endpointsFoo, suite.endpointsBar)
		})
	}
}

func (suite *selectionTrackerSuite) TestDuplicateTracking() {
	// This test aims to prove that tracking some ID multiple times doesn't
	// result in multiple requests for the same ID

	// associate the foo endpoints with some workloads 3 times without changing
	// the selection criteria. The second two times should be no-ops
	workloadAPI1 := rtest.Resource(pbcatalog.WorkloadType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), workloadData).Build()
	endpointsFoo := rtest.Resource(pbcatalog.ServiceEndpointsType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	suite.tracker.TrackIDForSelector(endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	suite.tracker.TrackIDForSelector(endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	suite.tracker.TrackIDForSelector(endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	// regardless of the number of times tracked we should only see a single request
	suite.requireMappedIDs(suite.T(), workloadAPI1, endpointsFoo)
}

func (suite *selectionTrackerSuite) TestModifyTracking() {
	// This test aims to prove that modifying selection criteria for a resource
	// works as expected. Adding new criteria results in all being tracked.
	// Removal of some criteria doesn't result in removal of all etc. More or
	// less we want to ensure that updating selection criteria leaves the
	// tracker in a consistent/expected state.

	// Create resources for the test and track endpoints.
	suite.trackIDForSelectorInAllTenancies(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names: []string{"web-1"},
	})

	// ensure that api-1 isn't mapped but web-1 is
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsAPI)
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb, suite.endpointsFoo)

	// now also track the api- prefix
	suite.trackIDForSelectorInAllTenancies(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names:    []string{"web-1"},
		Prefixes: []string{"api-"},
	})

	// ensure that both workloads are mapped appropriately
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsAPI, suite.endpointsFoo)
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb, suite.endpointsFoo)

	// now remove the web tracking
	suite.trackIDForSelectorInAllTenancies(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Prefixes: []string{"api-"},
	})

	// ensure that only api-1 is mapped
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsAPI, suite.endpointsFoo)
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb)
}

func (suite *selectionTrackerSuite) TestRemove() {
	// This test aims to prove that removal of a resource from tracking
	// actually prevents subsequent mapping calls from returning the
	// workload.

	// track the web-1 workload
	suite.trackIDForSelectorInAllTenancies(suite.endpointsFoo, &pbcatalog.WorkloadSelector{
		Names: []string{"web-1"},
	})

	// ensure web-1 is mapped
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb, suite.endpointsFoo)

	for i := range suite.executedTenancies {
		// untrack the resource
		suite.tracker.UntrackID(suite.endpointsFoo[i])
	}

	// ensure that we no longer map the previous workload to the resource
	suite.requireMappedIDsAllTenancies(suite.T(), suite.workloadsWeb)
}

func TestWorkloadSelectionSuite(t *testing.T) {
	suite.Run(t, new(selectionTrackerSuite))
}
