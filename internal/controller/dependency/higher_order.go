// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// WrapAndReplaceType will invoke the provided dependency mapper and then replace
// the type of all generated requests with the desired type.
func WrapAndReplaceType(desiredType *pbresource.Type, mapper controller.DependencyMapper) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		reqs, err := mapper(ctx, rt, res)
		if err != nil {
			return nil, err
		}

		for idx, req := range reqs {
			req.ID = resource.ReplaceType(desiredType, req.ID)
			reqs[idx] = req
		}
		return reqs, nil
	}
}

// MultiMapper can be used to concatenate the results of multiple dependency mappers. This
// helps to allow composition of different relationships without having to implement larger
// functions to perform all the various mappings. The goal here being to enable more reusuable
// dependency mappers.
func MultiMapper(mappers ...controller.DependencyMapper) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		var results []controller.Request
		for _, mapper := range mappers {
			reqs, err := mapper(ctx, rt, res)
			if err != nil {
				return nil, err
			}
			results = append(results, reqs...)
		}
		return results, nil
	}
}
