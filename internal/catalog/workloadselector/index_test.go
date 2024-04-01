// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestServiceWorkloadIndexer(t *testing.T) {
	c := cache.New()
	i := Index[*pbcatalog.Service]("selected-workloads")
	require.NoError(t, c.AddIndex(pbcatalog.ServiceType, i))

	foo := rtest.Resource(pbcatalog.ServiceType, "foo").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"api-2",
				},
				Prefixes: []string{
					"api-1",
				},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		Build()

	require.NoError(t, c.Insert(foo))

	bar := rtest.Resource(pbcatalog.ServiceType, "bar").
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"api-3",
				},
				Prefixes: []string{
					"api-2",
				},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		Build()

	require.NoError(t, c.Insert(bar))

	api123 := rtest.Resource(pbcatalog.WorkloadType, "api-123").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		Reference("")

	api2 := rtest.Resource(pbcatalog.WorkloadType, "api-2").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		Reference("")

	resources, err := c.Parents(pbcatalog.ServiceType, i.Name(), api123)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	prototest.AssertDeepEqual(t, foo, resources[0])

	resources, err = c.Parents(pbcatalog.ServiceType, i.Name(), api2)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	prototest.AssertElementsMatch(t, []*pbresource.Resource{foo, bar}, resources)

	refPrefix := &pbresource.Reference{
		Type: pbcatalog.WorkloadType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
	}
	resources, err = c.List(pbcatalog.ServiceType, i.Name(), refPrefix, index.IndexQueryOptions{Prefix: true})
	require.NoError(t, err)
	// because foo and bar both have 2 index values they will appear in the output twice
	require.Len(t, resources, 4)
	prototest.AssertElementsMatch(t, []*pbresource.Resource{foo, bar, foo, bar}, resources)
}

func TestServiceWorkloadIndexer_FromResource_Errors(t *testing.T) {
	t.Run("nil-selector", func(t *testing.T) {
		res := resourcetest.MustDecode[*pbcatalog.Service](
			t,
			resourcetest.Resource(pbcatalog.ServiceType, "foo").
				WithData(t, &pbcatalog.Service{}).
				WithTenancy(resource.DefaultNamespacedTenancy()).
				Build())

		indexed, vals, err := fromResource(res)
		require.False(t, indexed)
		require.Nil(t, vals)
		require.NoError(t, err)
	})

	t.Run("no-selections", func(t *testing.T) {
		res := resourcetest.MustDecode[*pbcatalog.Service](
			t,
			resourcetest.Resource(pbcatalog.ServiceType, "foo").
				WithData(t, &pbcatalog.Service{
					Workloads: &pbcatalog.WorkloadSelector{},
				}).
				WithTenancy(resource.DefaultNamespacedTenancy()).
				Build())

		indexed, vals, err := fromResource(res)
		require.False(t, indexed)
		require.Nil(t, vals)
		require.NoError(t, err)
	})
}
