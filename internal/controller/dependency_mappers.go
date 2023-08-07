// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controller

import (
	"context"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DependencyMapper is called when a dependency watched via WithWatch is changed
// to determine which of the controller's managed resources need to be reconciled.
type DependencyMapper func(
	ctx context.Context,
	rt Runtime,
	res *pbresource.Resource,
) ([]Request, error)

// MapOwner implements a DependencyMapper that returns the updated resource's owner.
func MapOwner(_ context.Context, _ Runtime, res *pbresource.Resource) ([]Request, error) {
	var reqs []Request
	if res.Owner != nil {
		reqs = append(reqs, Request{ID: res.Owner})
	}
	return reqs, nil
}

// MapOwnerFiltered creates a DependencyMapper that returns owner IDs as Requests
// if the type of the owner ID matches the given filter type.
func MapOwnerFiltered(filter *pbresource.Type) DependencyMapper {
	return func(_ context.Context, _ Runtime, res *pbresource.Resource) ([]Request, error) {
		if res.Owner == nil {
			return nil, nil
		}

		if !resource.EqualType(res.Owner.GetType(), filter) {
			return nil, nil
		}

		return []Request{{ID: res.Owner}}, nil
	}
}

// ReplaceType creates a DependencyMapper that returns request IDs with the same
// name and tenancy as the original resource but with the type replaced with
// the type specified as this functions parameter.
func ReplaceType(desiredType *pbresource.Type) DependencyMapper {
	return func(_ context.Context, _ Runtime, res *pbresource.Resource) ([]Request, error) {
		return []Request{
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
