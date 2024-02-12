// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MapOwner implements a DependencyMapper that returns the updated resource's owner.
func MapOwner(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var reqs []controller.Request
	if res.Owner != nil {
		reqs = append(reqs, controller.Request{ID: res.Owner})
	}
	return reqs, nil
}

// MapOwnerFiltered creates a DependencyMapper that returns owner IDs as Requests
// if the type of the owner ID matches the given filter type.
func MapOwnerFiltered(filter *pbresource.Type) controller.DependencyMapper {
	return func(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		if res.Owner == nil {
			return nil, nil
		}

		if !resource.EqualType(res.Owner.GetType(), filter) {
			return nil, nil
		}

		return []controller.Request{{ID: res.Owner}}, nil
	}
}

// ReplaceType creates a DependencyMapper that returns request IDs with the same
// name and tenancy as the original resource but with the type replaced with
// the type specified as this functions parameter.
func ReplaceType(desiredType *pbresource.Type) controller.DependencyMapper {
	return func(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		return []controller.Request{
			{
				ID: &pbresource.ID{
					Type:    desiredType,
					Tenancy: res.Id.Tenancy,
					Name:    res.Id.Name,
				},
			},
		}, nil
	}
}

type DecodedDependencyMapper[T proto.Message] func(context.Context, controller.Runtime, *resource.DecodedResource[T]) ([]controller.Request, error)

func MapDecoded[T proto.Message](mapper DecodedDependencyMapper[T]) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		decoded, err := resource.Decode[T](res)
		if err != nil {
			return nil, err
		}
		return mapper(ctx, rt, decoded)
	}
}
