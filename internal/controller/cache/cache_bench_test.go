package cache

import (
	"testing"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/internal/controller/cache/index"
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
func Benchmark_cache_Get(b *testing.B) {}

func Benchmark_cache_List(b *testing.B) {}

func Benchmark_cache_ListIterator(b *testing.B) {}

func Benchmark_cache_Parents(b *testing.B) {}

func Benchmark_cache_ParentsIterator(b *testing.B) {}

func Benchmark_cache_Query(b *testing.B) {}

// From WriteCache interface
func Benchmark_cache_Insert(b *testing.B) {}

func Benchmark_cache_Delete(b *testing.B) {}

type testSingleIndexer struct{}

func (testSingleIndexer) FromArgs(args ...any) ([]byte, error) {
	return index.ReferenceOrIDFromArgs(args)
}

func (testSingleIndexer) FromResource(*pbresource.Resource) (bool, []byte, error) {
	return false, nil, nil
}
