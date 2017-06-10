package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-memdb"
)

// txnKVS handles all KV-related operations.
func (s *Store) txnKVS(tx *memdb.Txn, idx uint64, op *structs.TxnKVOp) (structs.TxnResults, error) {
	var entry *structs.DirEntry
	var err error

	switch op.Verb {
	case api.KVSet:
		entry = &op.DirEnt
		err = s.kvsSetTxn(tx, idx, entry, false)

	case api.KVDelete:
		err = s.kvsDeleteTxn(tx, idx, op.DirEnt.Key)

	case api.KVDeleteCAS:
		var ok bool
		ok, err = s.kvsDeleteCASTxn(tx, idx, op.DirEnt.ModifyIndex, op.DirEnt.Key)
		if !ok && err == nil {
			err = fmt.Errorf("failed to delete key %q, index is stale", op.DirEnt.Key)
		}

	case api.KVDeleteTree:
		err = s.kvsDeleteTreeTxn(tx, idx, op.DirEnt.Key)

	case api.KVCAS:
		var ok bool
		entry = &op.DirEnt
		ok, err = s.kvsSetCASTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to set key %q, index is stale", op.DirEnt.Key)
		}

	case api.KVLock:
		var ok bool
		entry = &op.DirEnt
		ok, err = s.kvsLockTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to lock key %q, lock is already held", op.DirEnt.Key)
		}

	case api.KVUnlock:
		var ok bool
		entry = &op.DirEnt
		ok, err = s.kvsUnlockTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to unlock key %q, lock isn't held, or is held by another session", op.DirEnt.Key)
		}

	case api.KVGet:
		_, entry, err = s.kvsGetTxn(tx, nil, op.DirEnt.Key)
		if entry == nil && err == nil {
			err = fmt.Errorf("key %q doesn't exist", op.DirEnt.Key)
		}

	case api.KVGetTree:
		var entries structs.DirEntries
		_, entries, err = s.kvsListTxn(tx, nil, op.DirEnt.Key)
		if err == nil {
			results := make(structs.TxnResults, 0, len(entries))
			for _, e := range entries {
				result := structs.TxnResult{KV: e}
				results = append(results, &result)
			}
			return results, nil
		}

	case api.KVCheckSession:
		entry, err = s.kvsCheckSessionTxn(tx, op.DirEnt.Key, op.DirEnt.Session)

	case api.KVCheckIndex:
		entry, err = s.kvsCheckIndexTxn(tx, op.DirEnt.Key, op.DirEnt.ModifyIndex)

	case api.KVCheckNotExists:
		_, entry, err = s.kvsGetTxn(tx, nil, op.DirEnt.Key)
		if entry != nil && err == nil {
			err = fmt.Errorf("key %q exists", op.DirEnt.Key)
		}

	default:
		err = fmt.Errorf("unknown KV verb %q", op.Verb)
	}
	if err != nil {
		return nil, err
	}

	// For a GET we keep the value, otherwise we clone and blank out the
	// value (we have to clone so we don't modify the entry being used by
	// the state store).
	if entry != nil {
		if op.Verb == api.KVGet {
			result := structs.TxnResult{KV: entry}
			return structs.TxnResults{&result}, nil
		}

		clone := entry.Clone()
		clone.Value = nil
		result := structs.TxnResult{KV: clone}
		return structs.TxnResults{&result}, nil
	}

	return nil, nil
}

// txnDispatch runs the given operations inside the state store transaction.
func (s *Store) txnDispatch(tx *memdb.Txn, idx uint64, ops structs.TxnOps) (structs.TxnResults, structs.TxnErrors) {
	results := make(structs.TxnResults, 0, len(ops))
	errors := make(structs.TxnErrors, 0, len(ops))
	for i, op := range ops {
		var ret structs.TxnResults
		var err error

		// Dispatch based on the type of operation.
		if op.KV != nil {
			ret, err = s.txnKVS(tx, idx, op.KV)
		} else {
			err = fmt.Errorf("no operation specified")
		}

		// Accumulate the results.
		results = append(results, ret...)

		// Capture any error along with the index of the operation that
		// failed.
		if err != nil {
			errors = append(errors, &structs.TxnError{
				OpIndex: i,
				What:    err.Error(),
			})
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return results, nil
}

// TxnRW tries to run the given operations all inside a single transaction. If
// any of the operations fail, the entire transaction will be rolled back. This
// is done in a full write transaction on the state store, so reads and writes
// are possible
func (s *Store) TxnRW(idx uint64, ops structs.TxnOps) (structs.TxnResults, structs.TxnErrors) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	results, errors := s.txnDispatch(tx, idx, ops)
	if len(errors) > 0 {
		return nil, errors
	}

	tx.Commit()
	return results, nil
}

// TxnRO runs the given operations inside a single read transaction in the state
// store. You must verify outside this function that no write operations are
// present, otherwise you'll get an error from the state store.
func (s *Store) TxnRO(ops structs.TxnOps) (structs.TxnResults, structs.TxnErrors) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	results, errors := s.txnDispatch(tx, 0, ops)
	if len(errors) > 0 {
		return nil, errors
	}

	return results, nil
}
