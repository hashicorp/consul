package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

type RefFetcher[T proto.Message] func(*resource.DecodedResource[T]) []*pbresource.Reference

func RefIndex[T proto.Message](getRefs RefFetcher[T]) *cache.Index {
	return cache.NewIndex(&refIndexer[T]{
		getRefs: getRefs,
	})
}

type refIndexer[T proto.Message] struct {
	getRefs RefFetcher[T]
}

func (i *refIndexer[T]) FromArgs(args ...any) ([]byte, error) {
	return cache.ReferenceOrIDFromArgs(args...)
}

func (i *refIndexer[T]) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	res, err := resource.Decode[T](r)
	if err != nil {
		return false, nil, err
	}

	refs := i.getRefs(res)
	indexes := make([][]byte, len(refs))
	for idx, ref := range refs {
		indexes[idx] = cache.IndexFromRefOrID(ref)
	}

	return true, indexes, nil
}
