// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"sync"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const IDIndex = "id"

type kindIndices struct {
	mu sync.RWMutex

	it unversionedType

	indices map[string]*index.Index
}

func newKindIndices() *kindIndices {
	kind := &kindIndices{
		indices: make(map[string]*index.Index),
	}

	// add the id index
	kind.indices[IDIndex] = indexers.IDIndex(IDIndex, index.IndexRequired)

	return kind
}

func (k *kindIndices) addIndex(i *index.Index) error {
	_, found := k.indices[i.Name()]
	if found {
		return DuplicateIndexError{name: i.Name()}
	}

	k.indices[i.Name()] = i
	return nil
}

func (k *kindIndices) get(indexName string, args ...any) (*pbresource.Resource, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	idx, err := k.getIndex(indexName)
	if err != nil {
		return nil, err
	}

	r, err := idx.Txn().Get(args...)
	if err != nil {
		return nil, IndexError{err: err, name: indexName}
	}
	return r, nil
}

func (k *kindIndices) listIterator(indexName string, args ...any) (ResourceIterator, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	idx, err := k.getIndex(indexName)
	if err != nil {
		return nil, err
	}

	iter, err := idx.Txn().ListIterator(args...)
	if err != nil {
		return nil, IndexError{err: err, name: indexName}
	}
	return iter, nil
}

func (k *kindIndices) parentsIterator(indexName string, args ...any) (ResourceIterator, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	idx, err := k.getIndex(indexName)
	if err != nil {
		return nil, err
	}

	iter, err := idx.Txn().ParentsIterator(args...)
	if err != nil {
		return nil, IndexError{err: err, name: indexName}
	}
	return iter, nil
}

func (k *kindIndices) insert(r *pbresource.Resource) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	idx, err := k.getIndex(IDIndex)
	if err != nil {
		return err
	}

	existing, err := idx.Txn().Get(r.Id)
	if err != nil {
		return err
	}

	commit := false
	for name, idx := range k.indices {
		txn := idx.Txn()

		// Delete the previous version of the resource from the index.
		if existing != nil {
			if err := txn.Delete(existing); err != nil {
				return IndexError{name: name, err: err}
			}
		}

		// Now insert the new version into the index.
		err := txn.Insert(r)
		if err != nil {
			return IndexError{name: name, err: err}
		}
		// commit all radix trees once we know all index applies were successful. This is
		// still while holding the big write lock for this resource type so the order that
		// the radix tree updates occur shouldn't matter.
		defer func() {
			if commit {
				txn.Commit()
			}
		}()
	}

	// set commit to true so that the deferred funcs will commit all the radix tree transactions
	commit = true

	return nil
}

func (k *kindIndices) delete(r *pbresource.Resource) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	idx, err := k.getIndex(IDIndex)
	if err != nil {
		return err
	}

	idTxn := idx.Txn()

	existing, err := idTxn.Get(r.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	commit := false
	for name, idx := range k.indices {
		txn := idx.Txn()

		if err := txn.Delete(existing); err != nil {
			return IndexError{name: name, err: err}
		}

		// commit all radix trees once we know all index applies were successful. This is
		// still while holding the big write lock for this resource type so the order that
		// the radix tree updates occur shouldn't matter.
		defer func() {
			if commit {
				txn.Commit()
			}
		}()
	}

	// set commit to true so that the deferred txn commits will get executed
	commit = true

	return nil
}

func (k *kindIndices) getIndex(name string) (*index.Index, error) {
	idx, ok := k.indices[name]
	if !ok {
		return nil, CacheTypeError{err: IndexNotFoundError{name: name}, it: k.it}
	}

	return idx, nil
}
