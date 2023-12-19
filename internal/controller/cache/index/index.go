// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
	iradix "github.com/hashicorp/go-immutable-radix/v2"
)

type Index struct {
	name     string
	required bool
	indexer  MultiIndexer
	tree     *iradix.Tree[[]*pbresource.Resource]
}

func New(name string, i Indexer, opts ...IndexOption) *Index {
	if name == "" {
		panic("all indexers must have a non-empty name")
	}
	if i == nil {
		panic("no indexer was supplied when creating a new cache Index")
	}

	var multiIndexer MultiIndexer
	switch v := i.(type) {
	case SingleIndexer:
		multiIndexer = singleIndexWrapper{indexer: v}
	case MultiIndexer:
		multiIndexer = v
	default:
		panic("The Indexer must also implement one of the SingleIndexer or MultiIndexer interfaces")
	}

	idx := &Index{
		name:    name,
		indexer: multiIndexer,
		tree:    iradix.New[[]*pbresource.Resource](),
	}

	for _, opt := range opts {
		opt(idx)
	}

	return idx
}

func (i *Index) Name() string {
	return i.name
}

func (i *Index) Txn() Txn {
	return &txn{
		inner: i.tree.Txn(),
		index: i,
		dirty: false,
	}
}

func (i *Index) fromArgs(args ...any) ([]byte, error) {
	return i.indexer.FromArgs(args...)
}

func (i *Index) fromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	return i.indexer.FromResource(r)
}

type singleIndexWrapper struct {
	indexer SingleIndexer
}

func (s singleIndexWrapper) FromArgs(args ...any) ([]byte, error) {
	return s.indexer.FromArgs(args...)
}

func (s singleIndexWrapper) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	indexed, val, err := s.indexer.FromResource(r)
	if err != nil || !indexed {
		return false, nil, err
	}

	return true, [][]byte{val}, nil
}
