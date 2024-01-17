// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapSelectorToWorkloads will use the "id" index on watched Workload type to find all current
// workloads selected by the resource.
func MapSelectorToWorkloads[T WorkloadSelecting](_ context.Context, rt controller.Runtime, r *pbresource.Resource) ([]controller.Request, error) {
	res, err := resource.Decode[T](r)
	if err != nil {
		return nil, err
	}

	sel := res.Data.GetWorkloads()
	var reqs []controller.Request

	// Generate requests for all Workloads specified by an exact name
	for _, name := range sel.GetNames() {
		reqs = append(reqs, controller.Request{
			ID: &pbresource.ID{
				Type:    pbcatalog.WorkloadType,
				Name:    name,
				Tenancy: r.Id.Tenancy,
			},
		})
	}

	// Generate requests for workloads that would match the given prefix.
	for _, prefix := range sel.GetPrefixes() {
		iter, err := rt.Cache.ListIterator(pbcatalog.WorkloadType, "id", &pbresource.ID{
			Type:    pbcatalog.WorkloadType,
			Name:    prefix,
			Tenancy: r.Id.Tenancy,
		}, index.IndexQueryOptions{Prefix: true})

		if err != nil {
			return nil, err
		}

		for res := iter.Next(); res != nil; res = iter.Next() {
			reqs = append(reqs, controller.Request{
				ID: res.Id,
			})
		}
	}

	return reqs, nil
}

// MapWorkloadsToSelectors returns a DependencyMapper that will use the specified index to map a workload
// to resources that select it.
//
// This mapper can only be used on watches for the Workload type and works in conjunction with the Index
// created by this package.
func MapWorkloadsToSelectors(indexType *pbresource.Type, indexName string) controller.DependencyMapper {
	return dependency.CacheParentsMapper(indexType, indexName)
}
