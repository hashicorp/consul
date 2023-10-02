package cache

import (
	"sync"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type kindIndices struct {
	mu sync.RWMutex

	it unversionedType

	indices map[string]*Index
}

func newKindIndices() *kindIndices {
	kind := &kindIndices{
		indices: make(map[string]*Index),
	}

	// add the id index
	kind.indices[IDIndex] = NewIndex(idIndexer{}, IndexRequired)

	return kind
}

func (k *kindIndices) addIndex(name string, i *Index) error {
	_, found := k.indices[name]
	if found {
		return DuplicateIndexError{name: name}
	}

	k.indices[name] = i
	return nil
}

func (k *kindIndices) get(indexName string, args ...any) (*pbresource.Resource, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	idx, err := k.getIndex(indexName)
	if err != nil {
		return nil, err
	}

	r, err := idx.txn().get(args...)
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

	iter, err := idx.txn().listIterator(args...)
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

	iter, err := idx.txn().parentsIterator(args...)
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

	indexed, val, err := idx.getIndexValuesFromResource(r)
	if err != nil {
		return err
	}
	if !indexed || len(val) != 1 {
		panic("all resources must have the 'id' index which must produce a singler index value")
	}

	existing := idx.txn().getRaw(val[0])

	commit := false
	for name, idx := range k.indices {
		txn := idx.txn()

		if existing != nil {
			if err := txn.delete(existing); err != nil {
				return IndexError{name: name, err: err}
			}
		}

		err := txn.insert(r)
		if err != nil {
			return IndexError{name: name, err: err}
		}
		// commit all radix trees once we know all index applies were successful. This is
		// still while holding the big write lock for this resource type so the order that
		// the radix tree updates occur shouldn't matter.
		defer func() {
			if commit {
				txn.commit()
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

	indexed, val, err := idx.getIndexValuesFromResource(r)
	if err != nil {
		return err
	}
	if !indexed || len(val) != 1 {
		panic("all resources must have the 'id' index which must produce a singler index value")
	}

	existing := idx.txn().getRaw(val[0])
	if existing == nil {
		return nil
	}

	commit := false
	for name, idx := range k.indices {
		txn := idx.txn()

		if err := txn.delete(existing); err != nil {
			return IndexError{name: name, err: err}
		}

		// commit all radix trees once we know all index applies were successful. This is
		// still while holding the big write lock for this resource type so the order that
		// the radix tree updates occur shouldn't matter.
		defer func() {
			if commit {
				txn.commit()
			}
		}()
	}

	// set commit to true so that the deferred txn commits will get executed
	commit = true

	return nil
}

func (k *kindIndices) getIndex(name string) (*Index, error) {
	idx, ok := k.indices[name]
	if !ok {
		return nil, CacheTypeError{err: IndexNotFoundError{name: name}, it: k.it}
	}

	return idx, nil
}
