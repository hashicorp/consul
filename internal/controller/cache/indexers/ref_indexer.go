// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

//go:generate mockery --name RefOrIDFetcher --with-expecter
type RefOrIDFetcher[T proto.Message, V resource.ReferenceOrID] func(*resource.DecodedResource[T]) []V

func RefOrIDIndex[T proto.Message, V resource.ReferenceOrID](name string, fetch RefOrIDFetcher[T, V]) *index.Index {
	return DecodedMultiIndexer[T](name, index.ReferenceOrIDFromArgs, func(r *resource.DecodedResource[T]) (bool, [][]byte, error) {
		refs := fetch(r)
		indexes := make([][]byte, len(refs))
		for idx, ref := range refs {
			indexes[idx] = index.IndexFromRefOrID(ref)
		}

		return len(indexes) > 0, indexes, nil
	})
}

type BoundReferences interface {
	GetBoundReferences() []*pbresource.Reference
	proto.Message
}

func BoundRefsIndex[T BoundReferences](name string) *index.Index {
	return RefOrIDIndex[T](name, func(res *resource.DecodedResource[T]) []*pbresource.Reference {
		return res.Data.GetBoundReferences()
	})
}
