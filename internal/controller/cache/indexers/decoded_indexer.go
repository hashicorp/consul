// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

type FromArgs func(args ...any) ([]byte, error)
type SingleIndexer[T proto.Message] func(r *resource.DecodedResource[T]) (bool, []byte, error)
type MultiIndexer[T proto.Message] func(r *resource.DecodedResource[T]) (bool, [][]byte, error)

func DecodedSingleIndexer[T proto.Message](name string, args FromArgs, idx SingleIndexer[T]) *index.Index {
	return index.New(name, &singleIndexer[T]{
		decodedIndexer: idx,
		indexArgs:      args,
	})
}

func DecodedMultiIndexer[T proto.Message](name string, args FromArgs, idx MultiIndexer[T]) *index.Index {
	return index.New(name, &multiIndexer[T]{
		indexArgs:      args,
		decodedIndexer: idx,
	})
}

type singleIndexer[T proto.Message] struct {
	indexArgs      FromArgs
	decodedIndexer SingleIndexer[T]
}

func (i *singleIndexer[T]) FromArgs(args ...any) ([]byte, error) {
	return i.indexArgs(args...)
}

func (i *singleIndexer[T]) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	res, err := resource.Decode[T](r)
	if err != nil {
		return false, nil, err
	}

	return i.decodedIndexer(res)
}

type multiIndexer[T proto.Message] struct {
	decodedIndexer MultiIndexer[T]
	indexArgs      FromArgs
}

func (i *multiIndexer[T]) FromArgs(args ...any) ([]byte, error) {
	return i.indexArgs(args...)
}

func (i *multiIndexer[T]) FromResource(r *pbresource.Resource) (bool, [][]byte, error) {
	res, err := resource.Decode[T](r)
	if err != nil {
		return false, nil, err
	}

	return i.decodedIndexer(res)
}
