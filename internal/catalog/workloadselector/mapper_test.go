// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/cachemock"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/index/indexmock"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

var injectedError = errors.New("injected error")

func TestMapSelectorToWorkloads(t *testing.T) {
	cache := cachemock.NewReadOnlyCache(t)

	rt := controller.Runtime{
		Cache: cache,
	}

	mres := indexmock.NewResourceIterator(t)

	svc := resourcetest.Resource(pbcatalog.ServiceType, "api").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
				Names:    []string{"foo"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	api1 := resourcetest.Resource(pbcatalog.WorkloadType, "api-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	api2 := resourcetest.Resource(pbcatalog.WorkloadType, "api-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	fooRes := resourcetest.Resource(pbcatalog.WorkloadType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	cache.EXPECT().
		ListIterator(pbcatalog.WorkloadType, "id", &pbresource.ID{
			Type:    pbcatalog.WorkloadType,
			Name:    "api-",
			Tenancy: resource.DefaultNamespacedTenancy(),
		}, index.IndexQueryOptions{Prefix: true}).
		Return(mres, nil).
		Once()
	cache.EXPECT().
		Get(pbcatalog.WorkloadType, "id", &pbresource.ID{
			Type:    pbcatalog.WorkloadType,
			Name:    "foo",
			Tenancy: resource.DefaultNamespacedTenancy(),
		}).
		Return(fooRes, nil).
		Once()

	mres.EXPECT().Next().Return(api1).Once()
	mres.EXPECT().Next().Return(api2).Once()
	mres.EXPECT().Next().Return(nil).Once()

	expected := []controller.Request{
		{ID: fooRes.Id},
		{ID: api1.Id},
		{ID: api2.Id},
	}

	reqs, err := MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), rt, svc)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, expected, reqs)
}

func TestMapSelectorToWorkloads_DecodeError(t *testing.T) {
	res := resourcetest.Resource(pbcatalog.ServiceType, "foo").
		WithData(t, &pbcatalog.DNSPolicy{}).
		Build()

	reqs, err := MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), controller.Runtime{}, res)
	require.Nil(t, reqs)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestMapSelectorToWorkloads_CacheError(t *testing.T) {
	cache := cachemock.NewReadOnlyCache(t)

	rt := controller.Runtime{
		Cache: cache,
	}

	svc := resourcetest.Resource(pbcatalog.ServiceType, "api").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	cache.EXPECT().
		ListIterator(pbcatalog.WorkloadType, "id", &pbresource.ID{
			Type:    pbcatalog.WorkloadType,
			Name:    "api-",
			Tenancy: resource.DefaultNamespacedTenancy(),
		}, index.IndexQueryOptions{Prefix: true}).
		Return(nil, injectedError).
		Once()

	reqs, err := MapSelectorToWorkloads[*pbcatalog.Service](context.Background(), rt, svc)
	require.ErrorIs(t, err, injectedError)
	require.Nil(t, reqs)
}

func TestMapWorkloadsToSelectors(t *testing.T) {
	cache := cachemock.NewReadOnlyCache(t)
	rt := controller.Runtime{
		Cache:  cache,
		Logger: hclog.NewNullLogger(),
	}

	dm := MapWorkloadsToSelectors(pbcatalog.ServiceType, "selected-workloads")

	workload := resourcetest.Resource(pbcatalog.WorkloadType, "api-123").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	svc1 := resourcetest.Resource(pbcatalog.ServiceType, "foo").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	svc2 := resourcetest.Resource(pbcatalog.ServiceType, "bar").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	mres := indexmock.NewResourceIterator(t)

	cache.EXPECT().
		ParentsIterator(pbcatalog.ServiceType, "selected-workloads", workload.Id).
		Return(mres, nil).
		Once()

	mres.EXPECT().Next().Return(svc1).Once()
	mres.EXPECT().Next().Return(svc2).Once()
	mres.EXPECT().Next().Return(nil).Once()

	reqs, err := dm(context.Background(), rt, workload)
	require.NoError(t, err)
	expected := []controller.Request{
		{ID: svc1.Id},
		{ID: svc2.Id},
	}
	prototest.AssertElementsMatch(t, expected, reqs)

}
