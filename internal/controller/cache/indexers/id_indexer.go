// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func IDIndex(name string, opts ...index.IndexOption) *index.Index {
	return index.New(name, &idOrRefIndexer{getID: getResourceID}, opts...)
}

func OwnerIndex(name string, opts ...index.IndexOption) *index.Index {
	return index.New(name, &idOrRefIndexer{getID: getOwnerID}, opts...)
}

func SingleIDOrRefIndex(name string, f GetSingleRefOrID, opts ...index.IndexOption) *index.Index {
	return index.New(name, &idOrRefIndexer{getID: f}, opts...)
}

//go:generate mockery --name GetSingleRefOrID --with-expecter
type GetSingleRefOrID func(*pbresource.Resource) resource.ReferenceOrID

func getResourceID(r *pbresource.Resource) resource.ReferenceOrID {
	return r.GetId()
}

func getOwnerID(r *pbresource.Resource) resource.ReferenceOrID {
	return r.GetOwner()
}

type idOrRefIndexer struct {
	getID GetSingleRefOrID
}

// FromArgs constructs a radix tree key from an ID for lookup.
func (i idOrRefIndexer) FromArgs(args ...any) ([]byte, error) {
	return index.ReferenceOrIDFromArgs(args...)
}

// FromObject constructs a radix tree key from a Resource at write-time, or an
// ID at delete-time.
func (i idOrRefIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	return true, index.IndexFromRefOrID(i.getID(r)), nil
}
