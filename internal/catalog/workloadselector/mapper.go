// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapSelectorToWorkloads will use the "id" index on watched Workload type to find all current
// workloads selected by the resource.
func MapSelectorToWorkloads[T WorkloadSelecting](_ context.Context, rt controller.Runtime, r *pbresource.Resource) ([]controller.Request, error) {
	res, err := resource.Decode[T](r)
	if err != nil {
		return nil, err
	}

	workloads, err := GetWorkloadsWithSelector[T](rt.Cache, res)
	if err != nil {
		return nil, err
	}

	reqs := make([]controller.Request, len(workloads))
	for i, workload := range workloads {
		reqs[i] = controller.Request{
			ID: workload.Id,
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
