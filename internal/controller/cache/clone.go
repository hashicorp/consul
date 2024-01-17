// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"github.com/hashicorp/consul/internal/protoutil"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// NewCloningReadOnlyCache will wrap the provided ReadOnlyCache with
// an implementation which clones resources that are read out of the cache.
//
// In order to protect the Cache (and the resource service it shares resources with)
// from accidental user modification, the ReadOnlyCache interface is wrapped with
// the cloning variant. This wrapped/cloning cache is then what is given to
// dependency mappers and controller reconcilers. This allows those methods to
// freely modify data they get back from the cache without having to worry about
// accidental data modifications that could wreck the consistent state storage.
func NewCloningReadOnlyCache(cache ReadOnlyCache) ReadOnlyCache {
	return cloningReadOnlyCache{ReadOnlyCache: cache}
}

type cloningReadOnlyCache struct {
	ReadOnlyCache
}

func (c cloningReadOnlyCache) Get(it *pbresource.Type, indexName string, args ...any) (*pbresource.Resource, error) {
	res, err := c.ReadOnlyCache.Get(it, indexName, args...)
	if err != nil {
		return nil, err
	}

	return protoutil.Clone(res), nil
}

func (c cloningReadOnlyCache) List(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error) {
	resources, err := c.ReadOnlyCache.List(it, indexName, args...)
	if err != nil {
		return nil, err
	}

	return protoutil.CloneSlice(resources), nil
}

func (c cloningReadOnlyCache) ListIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error) {
	rit, err := c.ReadOnlyCache.ListIterator(it, indexName, args...)
	if err != nil {
		return nil, err
	}

	return cloningIterator{ResourceIterator: rit}, nil
}

func (c cloningReadOnlyCache) Parents(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error) {
	resources, err := c.ReadOnlyCache.Parents(it, indexName, args...)
	if err != nil {
		return nil, err
	}

	return protoutil.CloneSlice(resources), nil
}

func (c cloningReadOnlyCache) ParentsIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error) {
	rit, err := c.ReadOnlyCache.ParentsIterator(it, indexName, args...)
	if err != nil {
		return nil, err
	}

	return cloningIterator{ResourceIterator: rit}, nil
}

func (c cloningReadOnlyCache) Query(name string, args ...any) (ResourceIterator, error) {
	rit, err := c.ReadOnlyCache.Query(name, args...)
	if err != nil {
		return nil, err
	}

	return cloningIterator{ResourceIterator: rit}, nil
}

type cloningIterator struct {
	ResourceIterator
}

func (it cloningIterator) Next() *pbresource.Resource {
	res := it.ResourceIterator.Next()
	if res == nil {
		return nil
	}

	return protoutil.Clone(res)
}
