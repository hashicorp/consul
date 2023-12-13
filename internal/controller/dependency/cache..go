// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// CacheIDModifier is used to alter the Resource ID of the various Cache*
// controller.DependencyMappers prior to making the request to the cache. This is most
// useful to replace the type of resource queried (such as when resource types
// are name aligned) or to modify the tenancy in some way.
type CacheIDModifier func(
	ctx context.Context,
	rt controller.Runtime,
	id *pbresource.ID,
) (*pbresource.ID, error)

// CacheGetMapper is used to map an event for a watched resource by performing a Get of
// a single cached resource of any type.
func CacheGetMapper(indexedType *pbresource.Type, indexName string, mods ...CacheIDModifier) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		id, err := applyCacheIDMods(ctx, rt, res.GetId(), mods...)
		if err != nil {
			return nil, err
		}

		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapping dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		mapped, err := rt.Cache.Get(indexedType, indexName, id)
		if err != nil {
			rt.Logger.Error("failed to map dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf("failed to list from cache index %q on type %q: %w", indexName, resource.ToGVK(indexedType), err)
		}

		results := []controller.Request{
			{ID: mapped.GetId()},
		}
		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapped dependencies",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"dependency", mapped.GetId(),
			)
		}

		return results, nil
	}
}

// CacheListMapper is used to map the incoming resource to a set of requests for all
// the cached resources returned by the caches List operation.
func CacheListMapper(indexedType *pbresource.Type, indexName string, mods ...CacheIDModifier) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		id, err := applyCacheIDMods(ctx, rt, res.GetId(), mods...)
		if err != nil {
			return nil, err
		}

		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapping dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		iter, err := rt.Cache.ListIterator(indexedType, indexName, id)
		if err != nil {
			rt.Logger.Error("failed to map dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf("failed to list from cache index %q on type %q: %w", indexName, resource.ToGVK(indexedType), err)
		}

		var results []controller.Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			results = append(results, controller.Request{ID: res.GetId()})
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

// CacheParentsMapper is used to map the incoming resource to a set of requests for all
// the cached resources returned by the caches Parents operation.
func CacheParentsMapper(indexedType *pbresource.Type, indexName string, mods ...CacheIDModifier) controller.DependencyMapper {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
		id, err := applyCacheIDMods(ctx, rt, res.GetId(), mods...)
		if err != nil {
			return nil, err
		}

		if rt.Logger.IsTrace() {
			rt.Logger.Trace("mapping dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		iter, err := rt.Cache.ParentsIterator(indexedType, indexName, id)
		if err != nil {
			rt.Logger.Error("failed to map dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf("failed to list from cache index %q on type %q: %w", indexName, resource.ToGVK(indexedType), err)
		}

		var results []controller.Request
		for res := iter.Next(); res != nil; res = iter.Next() {
			results = append(results, controller.Request{ID: res.GetId()})
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

// CacheListTransform operates like CacheListMapper. The only difference is that the cache results
// are left as the whole cached resource instead of condensing down to just their IDs.
func CacheListTransform(indexedType *pbresource.Type, indexName string, mods ...CacheIDModifier) DependencyTransform {
	return func(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]*pbresource.Resource, error) {
		id, err := applyCacheIDMods(ctx, rt, res.GetId(), mods...)
		if err != nil {
			return nil, err
		}

		if rt.Logger.IsTrace() {
			rt.Logger.Trace("transforming dependencies from cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
			)
		}
		results, err := rt.Cache.List(indexedType, indexName, id)
		if err != nil {
			rt.Logger.Error("failed to transform dependencies from the cache",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"error", err,
			)
			return nil, fmt.Errorf(
				"failed to list from cache index %q on type %q: %w",
				indexName,
				resource.ToGVK(indexedType),
				err,
			)
		}

		if rt.Logger.IsTrace() {
			rt.Logger.Trace("transformed dependencies",
				"type", resource.ToGVK(indexedType),
				"index", indexName,
				"resource", resource.IDToString(res.GetId()),
				"dependencies", results,
			)
		}

		return results, nil
	}
}

// ReplaceCacheIDType will generate a CacheIDModifier that replaces the original ID's
// type with the desired type specified.
func ReplaceCacheIDType(desiredType *pbresource.Type) CacheIDModifier {
	return func(_ context.Context, _ controller.Runtime, id *pbresource.ID) (*pbresource.ID, error) {
		return resource.ReplaceType(desiredType, id), nil
	}
}

func applyCacheIDMods(ctx context.Context, rt controller.Runtime, id *pbresource.ID, mods ...CacheIDModifier) (*pbresource.ID, error) {
	var err error
	for _, mod := range mods {
		id, err = mod(ctx, rt, id)
		if err != nil {
			return nil, err
		}
	}
	return id, err
}
