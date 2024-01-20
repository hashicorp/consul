// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector_test

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestWorkloadSelectorCacheIntegration(t *testing.T) {
	c := cache.New()
	i := workloadselector.Index[*pbcatalog.Service]("selected-workloads")
	c.AddType(pbcatalog.WorkloadType)
	c.AddIndex(pbcatalog.ServiceType, i)

	rt := controller.Runtime{
		Cache:  c,
		Logger: testutil.Logger(t),
	}

	svcFoo := resourcetest.Resource(pbcatalog.ServiceType, "foo").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names:    []string{"foo"},
				Prefixes: []string{"api-", "other-"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	svcBar := resourcetest.Resource(pbcatalog.ServiceType, "bar").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names:    []string{"bar"},
				Prefixes: []string{"api-1", "something-else-"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	workloadBar := resourcetest.Resource(pbcatalog.WorkloadType, "bar").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	workloadAPIFoo := resourcetest.Resource(pbcatalog.WorkloadType, "api-foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	workloadAPI12 := resourcetest.Resource(pbcatalog.WorkloadType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	workloadFoo := resourcetest.Resource(pbcatalog.WorkloadType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	workloadSomethingElse12 := resourcetest.Resource(pbcatalog.WorkloadType, "something-else-12").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	// prime the cache with all of our services and workloads
	require.NoError(t, c.Insert(svcFoo))
	require.NoError(t, c.Insert(svcBar))
	require.NoError(t, c.Insert(workloadAPIFoo))
	require.NoError(t, c.Insert(workloadAPI12))
	require.NoError(t, c.Insert(workloadFoo))
	require.NoError(t, c.Insert(workloadSomethingElse12))

	// check that mapping a selecting resource to the list of currently selected workloads works as expected
	reqs, err := workloadselector.MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), rt, svcFoo)
	require.NoError(t, err)
	// in particular workloadSomethingElse12 should not show up here
	expected := []controller.Request{
		{ID: workloadFoo.Id},
		{ID: workloadAPI12.Id},
		{ID: workloadAPIFoo.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	reqs, err = workloadselector.MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), rt, svcBar)
	require.NoError(t, err)
	// workloadFoo and workloadAPIFoo should not show up here as they don't meet the selection critiera
	// workloadBar should not show up here because it hasn't been inserted into the cache yet.
	expected = []controller.Request{
		{ID: workloadSomethingElse12.Id},
		{ID: workloadAPI12.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	// insert workloadBar into the cache so that future calls to MapSelectorToWorkloads for svcBar show
	// the workload in the output
	require.NoError(t, c.Insert(workloadBar))

	// now validate that workloadBar shows up in the svcBar mapping
	reqs, err = workloadselector.MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), rt, svcBar)
	require.NoError(t, err)
	expected = []controller.Request{
		{ID: workloadSomethingElse12.Id},
		{ID: workloadAPI12.Id},
		{ID: workloadBar.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	// create the mapper to verify that finding services that select workloads functions correctly
	mapper := workloadselector.MapWorkloadsToSelectors(pbcatalog.ServiceType, i.Name())

	// check that workloadAPIFoo only returns a request for serviceFoo
	reqs, err = mapper(context.Background(), rt, workloadAPIFoo)
	require.NoError(t, err)
	expected = []controller.Request{
		{ID: svcFoo.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	// check that workloadAPI12 returns both services
	reqs, err = mapper(context.Background(), rt, workloadAPI12)
	require.NoError(t, err)
	expected = []controller.Request{
		{ID: svcFoo.Id},
		{ID: svcBar.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	// check that workloadSomethingElse12 returns only svcBar
	reqs, err = mapper(context.Background(), rt, workloadSomethingElse12)
	require.NoError(t, err)
	expected = []controller.Request{
		{ID: svcBar.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

	// check that workloadFoo returns only svcFoo
	reqs, err = mapper(context.Background(), rt, workloadFoo)
	require.NoError(t, err)
	expected = []controller.Request{
		{ID: svcFoo.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

}
