// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DependencyTransform is used when the incoming resource from a watch needs to
// be transformed to a different resource or set of resources and then have
// a DependencyMapper executed on each element in that result set. This should
// only be needed for dealing with more complex dependency relationships where
// the managed type and watched type are not directly related.
type DependencyTransform func(
	ctx context.Context,
	rt controller.Runtime,
	res *pbresource.Resource,
) ([]*pbresource.Resource, error)

// MapperWithTransform will execute the provided DependencyTransform and then execute
// the provided DependencyMapper once for each of the resources output by the transform.
// The DependencyMapper outputs will then be concatenated together to form the whole
// set of mapped Requests.
func MapperWithTransform(mapper controller.DependencyMapper, transform DependencyTransform) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		transformed, err := transform(ctx, rt, res)
		if err != nil {
			return nil, err
		}

		var reqs []controller.Request
		for _, res := range transformed {
			newReqs, err := mapper(ctx, rt, res)
			if err != nil {
				return nil, err
			}

			reqs = append(reqs, newReqs...)
		}

		return reqs, nil
	}
}

// TransformChain takes a set of transformers and will execute them as a pipeline.
// The first transformer will output some resources. Those resources will then be
// used as inputs to the next transform. The chain will then continue until all
// transformers have been run.
func TransformChain(transformers ...DependencyTransform) DependencyTransform {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]*pbresource.Resource, error) {
		toTransform := []*pbresource.Resource{res}

		for _, transform := range transformers {
			var nextResources []*pbresource.Resource

			for _, res := range toTransform {
				next, err := transform(ctx, rt, res)
				if err != nil {
					return nil, err
				}

				nextResources = append(nextResources, next...)
			}

			toTransform = nextResources
		}

		return toTransform, nil
	}
}
