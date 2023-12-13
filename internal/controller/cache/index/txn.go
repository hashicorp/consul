// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	iradix "github.com/hashicorp/go-immutable-radix/v2"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type txn struct {
	inner *iradix.Txn[[]*pbresource.Resource]
	index *Index
	dirty bool
}

func (t *txn) Get(args ...any) (*pbresource.Resource, error) {
	val, err := t.index.fromArgs(args...)
	if err != nil {
		return nil, err
	}

	return t.getRaw(val), nil
}

func (t *txn) getRaw(val []byte) *pbresource.Resource {
	resources, found := t.inner.Get(val)
	if !found || len(resources) < 1 {
		return nil
	}

	return resources[0]
}

func (t *txn) ListIterator(args ...any) (ResourceIterator, error) {
	val, err := t.index.fromArgs(args...)
	if err != nil {
		return nil, err
	}

	iter := t.inner.Root().Iterator()
	iter.SeekPrefix(val)

	return &resourceIterator{iter: iter}, nil
}

func (t *txn) ParentsIterator(args ...any) (ResourceIterator, error) {
	val, err := t.index.fromArgs(args...)
	if err != nil {
		return nil, err
	}

	iter := t.inner.Root().PathIterator(val)

	return &resourceIterator{iter: iter}, nil
}

func (t *txn) Insert(r *pbresource.Resource) error {
	indexed, vals, err := t.index.fromResource(r)
	if err != nil {
		return err
	}

	if !indexed && t.index.required {
		return MissingRequiredIndexError{Name: t.index.Name()}
	}

	for _, val := range vals {
		if t.insertOne(val, r) {
			t.dirty = true
		}
	}

	return nil
}

func (t *txn) insertOne(idxVal []byte, r *pbresource.Resource) bool {
	var newResources []*pbresource.Resource
	existing, found := t.inner.Get(idxVal)
	if found {
		newResources = make([]*pbresource.Resource, 0, len(existing)+1)
		found := false
		for _, rsc := range existing {
			if !resource.EqualID(rsc.GetId(), r.GetId()) {
				newResources = append(newResources, rsc)
			} else {
				found = true
				newResources = append(newResources, r)
			}
		}

		if !found {
			newResources = append(newResources, r)
		}
	} else {
		newResources = []*pbresource.Resource{r}
	}
	t.inner.Insert(idxVal, newResources)
	return true
}

func (t *txn) Delete(r *pbresource.Resource) error {
	indexed, vals, err := t.index.fromResource(r)
	if err != nil {
		return err
	}

	if !indexed && t.index.required {
		return MissingRequiredIndexError{Name: t.index.Name()}
	}

	for _, val := range vals {
		if t.deleteOne(val, r) {
			t.dirty = true
		}
	}

	return nil
}

func (t *txn) deleteOne(idxVal []byte, r *pbresource.Resource) bool {
	existing, found := t.inner.Get(idxVal)
	if !found {
		// this resource is not currently indexed
		return false
	}

	existingIdx := -1
	for idx, rsc := range existing {
		if resource.EqualID(rsc.GetId(), r.GetId()) {
			existingIdx = idx
			break
		}
	}

	switch {
	// The index value maps to some resources but none had the same id as the one we wish
	// to delete. Therefore we can leave the slice alone because the delete is a no-op
	case existingIdx < 0:
		return false

	// We found something (existingIdex >= 0) but there is only one thing in the array. In
	// this case we should remove the whole resource slice from the tree.
	case len(existing) == 1:
		t.inner.Delete(idxVal)

	// The first slice element is the resource to delete so we can reslice safely
	case existingIdx == 0:
		t.inner.Insert(idxVal, existing[1:])

	// The last slice element is the resource to delete so we can reslice safely
	case existingIdx == len(existing)-1:
		t.inner.Insert(idxVal, existing[:len(existing)-1])

	// The resource to delete exists somewhere in the middle of the slice. So we must
	// recreate a new backing array to maintain immutability of the data being stored
	default:
		newResources := make([]*pbresource.Resource, len(existing)-1)
		copy(newResources[0:existingIdx], existing[0:existingIdx])
		copy(newResources[existingIdx:], existing[existingIdx+1:])
		t.inner.Insert(idxVal, newResources)
	}

	return true
}

func (t *txn) Commit() {
	if t.dirty {
		t.index.tree = t.inner.Commit()
	}
}
