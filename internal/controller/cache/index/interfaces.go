// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import "github.com/hashicorp/consul/proto-public/pbresource"

// Indexer is the base interface that all indexers must implement. Additionally
// an indexer must also implement either the SingleIndexer or MultiIndexer interface
//
//go:generate mockery --name Indexer --with-expecter
type Indexer interface {
	FromArgs(args ...interface{}) ([]byte, error)
}

// SingleIndexer is the interface to use to extract a single index value from a
// resource. E.g. extracting the resources owner ID.
//
//go:generate mockery --name SingleIndexer --with-expecter
type SingleIndexer interface {
	Indexer
	FromResource(r *pbresource.Resource) (bool, []byte, error)
}

// MultiIndexer is the interface to implement for extracting multiple index
// values from a resource. E.g. extracting all resource References from the
// slice of references.
//
//go:generate mockery --name MultiIndexer --with-expecter
type MultiIndexer interface {
	Indexer
	FromResource(r *pbresource.Resource) (bool, [][]byte, error)
}

// IndexOption is a functional option to use to modify an Indexes behavior.
type IndexOption func(*Index)

// IndexRequired will cause the index to return a MissingRequiredIndexError
// in the event that the indexer returns no values for a resource.
func IndexRequired(s *Index) {
	s.required = true
}

// ResourceIterator is the interface that will be returned to iterate through
// a collection of results.
//
//go:generate mockery --name ResourceIterator --with-expecter
type ResourceIterator interface {
	Next() *pbresource.Resource
}

// Txn is an interface to control changes to an index within an transaction.
type Txn interface {
	Get(args ...any) (*pbresource.Resource, error)
	ListIterator(args ...any) (ResourceIterator, error)
	ParentsIterator(args ...any) (ResourceIterator, error)
	Insert(r *pbresource.Resource) error
	Delete(r *pbresource.Resource) error
	Commit()
}
