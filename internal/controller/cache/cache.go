package cache

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const IDIndex = "id"

type IndexedType struct {
	Type *pbresource.Type
	// Indexes []*Index
}

type Cache interface {
	AddType(it *pbresource.Type)
	AddIndex(it *pbresource.Type, indexName string, index *Index) error
	ReadOnlyCache
	WriteCache
}

type ReadOnlyCache interface {
	Get(it *pbresource.Type, indexName string, args ...any) (*pbresource.Resource, error)
	List(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error)
	ListIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error)
	Parents(it *pbresource.Type, indexName string, args ...any) ([]*pbresource.Resource, error)
	ParentsIterator(it *pbresource.Type, indexName string, args ...any) (ResourceIterator, error)
}

type WriteCache interface {
	Insert(r *pbresource.Resource) error
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
	kinds map[unversionedType]*kindIndices
}

func New() Cache {
	return &cache{
		kinds: make(map[unversionedType]*kindIndices),
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

func (c *cache) AddIndex(it *pbresource.Type, indexName string, index *Index) error {
	kind := c.ensureTypeCached(it)
	err := kind.addIndex(indexName, index)
	if err != nil {
		return CacheTypeError{it: unversionedType{Group: it.Group, Kind: it.Kind}, err: err}
	}
	return nil
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

type resourceIterable interface {
	Next() ([]byte, []*pbresource.Resource, bool)
}

type resourceIterator struct {
	current []*pbresource.Resource
	iter    resourceIterable
}

func (i *resourceIterator) Next() *pbresource.Resource {
	if i == nil {
		return nil
	}

	// Maybe get a new list from the internal iterator
	if len(i.current) == 0 {
		_, i.current, _ = i.iter.Next()
	}

	var rsc *pbresource.Resource
	switch len(i.current) {
	case 0:
		// we are completely out of data so we can return
	case 1:
		rsc = i.current[0]
		i.current = nil
	default:
		rsc = i.current[0]
		i.current = i.current[1:]
	}

	return rsc
}
