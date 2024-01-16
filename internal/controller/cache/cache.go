// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Query is the function type to use for named query callbacks
type Query func(c ReadOnlyCache, args ...any) (ResourceIterator, error)

type Cache interface {
	// AddType will add a new resource type to the cache. This will
	// include configuring an `id` index based on the resources Id
	AddType(it *pbresource.Type)
	// AddIndex will add a new index for the specified type to the cache.
	// If the type isn't yet known to the cache, it will first add it
	// including setting up its `id` index.
	AddIndex(it *pbresource.Type, index *index.Index) error
	// AddQuery will add a new named query to the cache. This query
	// can potentially use multiple different cache indexes to come
	// up with the final result iterator.
	AddQuery(name string, fn Query) error

	ReadOnlyCache
	WriteCache
}

// ReadOnlyCache is the set of methods on the Resource cache that can be used
// to query the cache.
type ReadOnlyCache interface {
	// Get retrieves a single resource from the specified index that matches the provided args.
	// If more than one match is found the first is returned.
	Get(it *pbresource.Type, indexName string, args ...any) (*pbresource.Resource, error)

	// List retrieves all the resources from the specified index matching the provided args.
	List(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error)

	// ListIterator retrieves an iterator over all resources from the specified index matching the provided args.
	ListIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error)

	// Parents retrieves all resources whos index value is a parent (or prefix) of the value calculated
	// from the provided args.
	Parents(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error)

	// ParentsIterator retrieves an iterator over all resources whos index value is a parent (or prefix)
	// of the value calculated from the provided args.
	ParentsIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error)

	// Query will execute a named query against the cache and return an interator over its results
	Query(name string, args ...any) (ResourceIterator, error)
}

type WriteCache interface {
	// Insert will add a single resource into the cache. If it already exists, this will update
	// all indexing to the current values.
	Insert(r *pbresource.Resource) error

	// Delete will remove a single resource from the cache.
	Delete(r *pbresource.Resource) error
}

type ResourceIterator interface {
	Next() *pbresource.Resource
}

type unversionedType struct {
	Group string
	Kind  string
}

type cache struct {
	kinds   map[unversionedType]*kindIndices
	queries map[string]Query
}

func New() Cache {
	return newCache()
}

func newCache() *cache {
	return &cache{
		kinds:   make(map[unversionedType]*kindIndices),
		queries: make(map[string]Query),
	}
}

func (c *cache) ensureTypeCached(it *pbresource.Type) *kindIndices {
	ut := unversionedType{Group: it.Group, Kind: it.Kind}

	_, ok := c.kinds[ut]
	if !ok {
		c.kinds[ut] = newKindIndices()
	}

	return c.kinds[ut]
}

func (c *cache) AddType(it *pbresource.Type) {
	c.ensureTypeCached(it)
}

func (c *cache) AddIndex(it *pbresource.Type, index *index.Index) error {
	kind := c.ensureTypeCached(it)
	err := kind.addIndex(index)
	if err != nil {
		return CacheTypeError{it: unversionedType{Group: it.Group, Kind: it.Kind}, err: err}
	}
	return nil
}

func (c *cache) AddQuery(name string, fn Query) error {
	if fn == nil {
		return QueryRequired
	}
	if _, found := c.queries[name]; found {
		return DuplicateQueryError{name: name}
	}

	c.queries[name] = fn
	return nil
}

func (c *cache) Query(name string, args ...any) (ResourceIterator, error) {
	fn, found := c.queries[name]
	if !found {
		return nil, QueryNotFoundError{name: name}
	}

	return fn(c, args...)
}

func (c *cache) Get(it *pbresource.Type, indexName string, args ...any) (*pbresource.Resource, error) {
	indices, err := c.getTypeIndices(it)
	if err != nil {
		return nil, err
	}

	return indices.get(indexName, args...)
}

func (c *cache) ListIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error) {
	indices, err := c.getTypeIndices(it)
	if err != nil {
		return nil, err
	}

	return indices.listIterator(indexName, args...)
}

func (c *cache) List(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error) {
	return expandIterator(c.ListIterator(it, indexName, args...))
}

func (c *cache) ParentsIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error) {
	indices, err := c.getTypeIndices(it)
	if err != nil {
		return nil, err
	}

	return indices.parentsIterator(indexName, args...)
}

func (c *cache) Parents(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error) {
	return expandIterator(c.ParentsIterator(it, indexName, args...))
}

func (c *cache) Insert(r *pbresource.Resource) error {
	indices, err := c.getTypeIndices(r.Id.Type)
	if err != nil {
		return err
	}

	return indices.insert(r)
}

func (c *cache) Delete(r *pbresource.Resource) error {
	indices, err := c.getTypeIndices(r.Id.Type)
	if err != nil {
		return err
	}

	return indices.delete(r)
}

func (c *cache) getTypeIndices(it *pbresource.Type) (*kindIndices, error) {
	if it == nil {
		return nil, TypeUnspecifiedError
	}

	ut := unversionedType{Group: it.Group, Kind: it.Kind}

	indices, ok := c.kinds[ut]
	if !ok {
		return nil, TypeNotIndexedError
	}
	return indices, nil
}

func expandIterator(iter ResourceIterator, err error) ([]*pbresource.Resource, error) {
	if err != nil {
		return nil, err
	}

	var results []*pbresource.Resource
	for res := iter.Next(); res != nil; res = iter.Next() {
		results = append(results, res)
	}

	return results, nil
}
