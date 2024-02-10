package cache

import (
	"testing"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var resultCache Cache

// From Cache interface
func Benchmark_cache_AddType_New(b *testing.B) {
	c := New()

	// Always add a new type since if existing then it is just a map access
	newTypes := make([]*pbresource.Type, 0, b.N)
	for i := 0; i < b.N; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.AddType(newTypes[i])
	}
	// store result as package level variable to ensure not removed by compiler
	resultCache = c
}

func Benchmark_cache_AddType_Existing(b *testing.B) {
	c := New()

	u, err := uuid.GenerateUUID()
	if err != nil {
		b.Error(err)
	}

	newType := &pbresource.Type{
		Group:        u,
		GroupVersion: u,
		Kind:         u,
	}

	c.AddType(newType)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.AddType(newType)
	}
	// store result as package level variable to ensure not removed by compiler
	resultCache = c
}

func Benchmark_cache_AddIndex(b *testing.B) {
	c := New()

	newTypes := make([]*pbresource.Type, 0, b.N)
	newIndices := make([]*index.Index, 0, b.N)
	for i := 0; i < b.N; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)

		newIndex := index.New(u, testSingleIndexer{})
		newIndices = append(newIndices, newIndex)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := c.AddIndex(newTypes[i], newIndices[i])
		if err != nil {
			b.Error(err)
		}
	}
	resultCache = c
}

func Benchmark_cache_AddQuery(b *testing.B) {
	c := New()

	names := make([]string, 0, b.N)
	for i := 0; i < b.N; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		names = append(names, u)
	}

	queryFunc := func(c ReadOnlyCache, args ...any) (ResourceIterator, error) {
		return nil, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := c.AddQuery(names[i], queryFunc)
		if err != nil {
			b.Error(err)
		}
	}

	resultCache = c
}

// From ReadOnlyCache interface
func Benchmark_cache_Get_CacheHit(b *testing.B) {
	c := New()

	typeCount := 1000

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)
		newIndices = append(newIndices, getTestIndexer())

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]

		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Insert the item into the cache ahead of time
		if err = c.Insert(item); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := c.Get(resources[i].Id.Type, "uuid", resources[i].Id.Name)
		if err != nil {
			b.Error(err)
		}
		if res == nil {
			b.Logf("unexpected cache miss")
		}
	}
	resultCache = c
}

func Benchmark_cache_Get_CacheMiss(b *testing.B) {
	c := New()

	typeCount := 1000

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)
		newIndices = append(newIndices, getTestIndexer())

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Don't add the item to the cache since we want all misses
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := c.Get(resources[i].Id.Type, "uuid", resources[i].Id.Name)
		if err != nil {
			b.Error(err)
		}
		if res != nil {
			b.Logf("unexpected cache hit")
		}
	}
	resultCache = c
}

func Benchmark_cache_List(b *testing.B) {
	c := New()

	// reduce the number of types, so we have a higher number of resources per type
	typeCount := 5

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)
		newIndices = append(newIndices, getTestIndexer())

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Insert the item into the cache ahead of time
		if err = c.Insert(item); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := c.List(resources[i].Id.Type, "uuid", resources[i].Id.Name)
		if err != nil {
			b.Error(err)
		}
		if len(res) <= 0 {
			b.Error("unexpected empty iterator")
		}
	}
	resultCache = c
}

func Benchmark_cache_ListIterator(b *testing.B) {
	c := New()

	// reduce the number of types, so we have a higher number of resources per type
	typeCount := 5

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)
		newIndices = append(newIndices, getTestIndexer())

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Insert the item into the cache ahead of time
		if err = c.Insert(item); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := c.ListIterator(resources[i].Id.Type, "uuid", resources[i].Id.Name)
		if err != nil {
			b.Error(err)
		}
		if res == nil {
			b.Error("unexpected nil iterator")
		}
	}
	resultCache = c
}

func Benchmark_cache_Parents(b *testing.B) {
	// TODO (@johnlanda)
}

func Benchmark_cache_ParentsIterator(b *testing.B) {
  // TODO (@johnlanda)
}

func Benchmark_cache_Query(b *testing.B) {
	c := New()

	names := make([]string, 0, b.N)
	for i := 0; i < b.N; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		names = append(names, u)
	}

	queryFunc := func(c ReadOnlyCache, args ...any) (ResourceIterator, error) {
		return nil, nil
	}

	for i := 0; i < b.N; i++ {
		err := c.AddQuery(names[i], queryFunc)
		if err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.Query(names[i])
		if err != nil {
			b.Error(err)
		}
	}
	resultCache = c
}

// From WriteCache interface
func Benchmark_cache_Insert_New(b *testing.B) {
	c := New()

	typeCount := 1000

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)

		newIndex := index.New(u, testSingleIndexer{})
		newIndices = append(newIndices, newIndex)

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		// pull the uuid out of the type
		u := resourceType.GroupVersion
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := c.Insert(resources[i])
		if err != nil {
			b.Error(err)
		}
	}
	resultCache = c
}

func Benchmark_cache_Insert_Existing(b *testing.B) {
	c := New()

	typeCount := 1000

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)

		newIndex := index.New(u, testSingleIndexer{})
		newIndices = append(newIndices, newIndex)

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		// pull the uuid out of the type
		u := resourceType.GroupVersion
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Insert the item into the cache ahead of time
		if err := c.Insert(item); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := c.Insert(resources[i])
		if err != nil {
			b.Error(err)
		}
	}
	resultCache = c
}

func Benchmark_cache_Delete(b *testing.B) {
	c := New()

	typeCount := 1000

	// generate types and add indices
	newTypes := make([]*pbresource.Type, 0, typeCount)
	newIndices := make([]*index.Index, 0, typeCount)
	resources := make([]*pbresource.Resource, 0, b.N)
	for i := 0; i < typeCount; i++ {
		u, err := uuid.GenerateUUID()
		if err != nil {
			b.Error(err)
		}

		newType := &pbresource.Type{
			Group:        u,
			GroupVersion: u,
			Kind:         u,
		}
		newTypes = append(newTypes, newType)

		newIndex := index.New(u, testSingleIndexer{})
		newIndices = append(newIndices, newIndex)

		if err = c.AddIndex(newTypes[i], newIndices[i]); err != nil {
			b.Error(err)
		}
	}

	for i := 0; i < b.N; i++ {
		resourceType := newTypes[i%len(newTypes)]
		// pull the uuid out of the type
		u := resourceType.GroupVersion
		item := &pbresource.Resource{
			Id: &pbresource.ID{
				Uid:  u,
				Name: u,
				Type: resourceType,
			},
		}
		resources = append(resources, item)
		// Insert the item into the cache ahead of time
		if err := c.Insert(item); err != nil {
			b.Error(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := c.Delete(resources[i])
		if err != nil {
			b.Error(err)
		}
	}
	resultCache = c
}

type testSingleIndexer struct{}

func (testSingleIndexer) FromArgs(args ...any) ([]byte, error) {
	return index.ReferenceOrIDFromArgs(args)
}

func (testSingleIndexer) FromResource(*pbresource.Resource) (bool, []byte, error) {
	return false, nil, nil
}

func getTestIndexer() *index.Index {
	return indexers.DecodedSingleIndexer(
		"uuid",
		index.SingleValueFromArgs(func(value string) ([]byte, error) {
			return []byte(value), nil
		}),
		func(r *resource.DecodedResource[*pbresource.Resource]) (bool, []byte, error) {
			return true, []byte(r.Id.Name), nil
		})
}
