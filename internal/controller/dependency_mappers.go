// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"fmt"

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

// CustomDependencyMapper is called when an Event occurs to determine which of the
// controller's managed resources need to be reconciled.
type CustomDependencyMapper func(
	ctx context.Context,
	rt Runtime,
	event Event,
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

func CacheListMapper(indexedType *pbresource.Type, indexName string) DependencyMapper {
	return func(_ context.Context, rt Runtime, res *pbresource.Resource) ([]Request, error) {
		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapping dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		iter, err := rt.Cache.ListIterator(indexedType, indexName, res.GetId())
		if err != nil {
			rt.Logger.Error("failed to map dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf("failed to list from cache index %q on type %q: %w", indexName, resource.ToGVK(indexedType), err)
		}

		var results []Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			results = append(results, Request{ID: res.GetId()})
		}
		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapped dependencies",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"dependencies", results,
			)
		}

		return results, nil
	}
}

func CacheParentsMapper(indexedType *pbresource.Type, indexName string) DependencyMapper {
	return func(_ context.Context, rt Runtime, res *pbresource.Resource) ([]Request, error) {
		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapping dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		iter, err := rt.Cache.ParentsIterator(indexedType, indexName, res.GetId())
		if err != nil {
			rt.Logger.Error("failed to map dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf("failed to list from cache index %q on type %q: %w", indexName, resource.ToGVK(indexedType), err)
		}

		var results []Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			results = append(results, Request{ID: res.GetId()})
		}
		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapped dependencies",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"dependencies", results,
			)
		}

		return results, nil
	}
}

func WrapAndReplaceType(desiredType *pbresource.Type, mapper DependencyMapper) DependencyMapper {
	return func(ctx context.Context, rt Runtime, res *pbresource.Resource) ([]Request, error) {
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
