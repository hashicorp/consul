// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// txnKVS handles all KV-related operations.
func (s *Store) txnKVS(tx WriteTxn, idx uint64, op *structs.TxnKVOp) (structs.TxnResults, error) {
	var entry *structs.DirEntry
	var err error

	switch op.Verb {
	case api.KVSet:
		entry = &op.DirEnt
		err = kvsSetTxn(tx, idx, entry, false)

	case api.KVDelete:
		err = s.kvsDeleteTxn(tx, idx, op.DirEnt.Key, &op.DirEnt.EnterpriseMeta)

	case api.KVDeleteCAS:
		var ok bool
		ok, err = s.kvsDeleteCASTxn(tx, idx, op.DirEnt.ModifyIndex, op.DirEnt.Key, &op.DirEnt.EnterpriseMeta)
		if !ok && err == nil {
			err = fmt.Errorf("failed to delete key %q, index is stale", op.DirEnt.Key)
		}

	case api.KVDeleteTree:
		err = s.kvsDeleteTreeTxn(tx, idx, op.DirEnt.Key, &op.DirEnt.EnterpriseMeta)

	case api.KVCAS:
		var ok bool
		entry = &op.DirEnt
		ok, err = kvsSetCASTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to set key %q, index is stale", op.DirEnt.Key)
		}

	case api.KVLock:
		var ok bool
		entry = &op.DirEnt
		ok, err = kvsLockTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to lock key %q, lock is already held", op.DirEnt.Key)
		}

	case api.KVUnlock:
		var ok bool
		entry = &op.DirEnt
		ok, err = kvsUnlockTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to unlock key %q, lock isn't held, or is held by another session", op.DirEnt.Key)
		}

	case api.KVGet:
		_, entry, err = kvsGetTxn(tx, nil, op.DirEnt.Key, op.DirEnt.EnterpriseMeta)
		if entry == nil && err == nil {
			err = fmt.Errorf("key %q doesn't exist", op.DirEnt.Key)
		}

	case api.KVGetOrEmpty:
		_, entry, err = kvsGetTxn(tx, nil, op.DirEnt.Key, op.DirEnt.EnterpriseMeta)
		if entry == nil && err == nil {
			entry = &op.DirEnt
			entry.Value = nil
		}

	case api.KVGetTree:
		var entries structs.DirEntries
		_, entries, err = s.kvsListTxn(tx, nil, op.DirEnt.Key, op.DirEnt.EnterpriseMeta)
		if err == nil {
			results := make(structs.TxnResults, 0, len(entries))
			for _, e := range entries {
				result := structs.TxnResult{KV: e}
				results = append(results, &result)
			}
			return results, nil
		}

	case api.KVCheckSession:
		entry, err = kvsCheckSessionTxn(tx, op.DirEnt.Key, op.DirEnt.Session, &op.DirEnt.EnterpriseMeta)

	case api.KVCheckIndex:
		entry, err = kvsCheckIndexTxn(tx, op.DirEnt.Key, op.DirEnt.ModifyIndex, op.DirEnt.EnterpriseMeta)

	case api.KVCheckNotExists:
		_, entry, err = kvsGetTxn(tx, nil, op.DirEnt.Key, op.DirEnt.EnterpriseMeta)
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
		if op.Verb == api.KVGet || op.Verb == api.KVGetOrEmpty {
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

// txnSession handles all Session-related operations.
func txnSession(tx WriteTxn, idx uint64, op *structs.TxnSessionOp) error {
	var err error

	switch op.Verb {
	case api.SessionDelete:
		err = sessionDeleteWithSession(tx, &op.Session, idx)
	default:
		err = fmt.Errorf("unknown Session verb %q", op.Verb)
	}
	if err != nil {
		return fmt.Errorf("failed to delete session: %v", err)
	}

	return nil
}

// txnLegacyIntention handles all Intention-related operations.
//
// Deprecated: see TxnOp.Intention description
func txnLegacyIntention(tx WriteTxn, idx uint64, op *structs.TxnIntentionOp) error {
	switch op.Op {
	case structs.IntentionOpCreate, structs.IntentionOpUpdate:
		return legacyIntentionSetTxn(tx, idx, op.Intention)
	case structs.IntentionOpDelete:
		return legacyIntentionDeleteTxn(tx, idx, op.Intention.ID)
	case structs.IntentionOpDeleteAll:
		fallthrough // deliberately not available via this api
	case structs.IntentionOpUpsert:
		fallthrough // deliberately not available via this api
	default:
		return fmt.Errorf("unknown Intention op %q", op.Op)
	}
}

// txnNode handles all Node-related operations.
func (s *Store) txnNode(tx WriteTxn, idx uint64, op *structs.TxnNodeOp) (structs.TxnResults, error) {
	var entry *structs.Node
	var err error

	// TODO(partitions): change these errors to include node partitions when printing

	getNode := func() (*structs.Node, error) {
		if op.Node.ID != "" {
			return getNodeIDTxn(tx, op.Node.ID, op.Node.GetEnterpriseMeta(), op.Node.PeerName)
		} else {
			return getNodeTxn(tx, op.Node.Node, op.Node.GetEnterpriseMeta(), op.Node.PeerName)
		}
	}

	switch op.Verb {
	case api.NodeGet:
		entry, err = getNode()
		if entry == nil && err == nil {
			err = fmt.Errorf("node %q doesn't exist", op.Node.Node)
		}

	case api.NodeSet:
		err = s.ensureNodeTxn(tx, idx, false, &op.Node)
		if err == nil {
			entry, err = getNode()
		}

	case api.NodeCAS:
		var ok bool
		ok, err = s.ensureNodeCASTxn(tx, idx, &op.Node)
		if !ok && err == nil {
			err = fmt.Errorf("failed to set node %q, index is stale", op.Node.Node)
			break
		}
		entry, err = getNode()

	case api.NodeDelete:
		err = s.deleteNodeTxn(tx, idx, op.Node.Node, op.Node.GetEnterpriseMeta(), op.Node.PeerName)

	case api.NodeDeleteCAS:
		var ok bool
		ok, err = s.deleteNodeCASTxn(tx, idx, op.Node.ModifyIndex, op.Node.Node, op.Node.GetEnterpriseMeta(), op.Node.PeerName)
		if !ok && err == nil {
			err = fmt.Errorf("failed to delete node %q, index is stale", op.Node.Node)
		}

	default:
		err = fmt.Errorf("unknown Node verb %q", op.Verb)
	}
	if err != nil {
		return nil, err
	}

	// For a GET we keep the value, otherwise we clone and blank out the
	// value (we have to clone so we don't modify the entry being used by
	// the state store).
	if entry != nil {
		if op.Verb == api.NodeGet {
			result := structs.TxnResult{Node: entry}
			return structs.TxnResults{&result}, nil
		}

		clone := *entry
		result := structs.TxnResult{Node: &clone}
		return structs.TxnResults{&result}, nil
	}

	return nil, nil
}

// txnService handles all Service-related operations.
func (s *Store) txnService(tx WriteTxn, idx uint64, op *structs.TxnServiceOp) (structs.TxnResults, error) {
	switch op.Verb {
	case api.ServiceGet:
		entry, err := getNodeServiceTxn(tx, nil, op.Node, op.Service.ID, &op.Service.EnterpriseMeta, op.Service.PeerName)
		switch {
		case err != nil:
			return nil, err
		case entry == nil:
			return nil, fmt.Errorf("service %q on node %q doesn't exist", op.Service.ID, op.Node)
		default:
			return structs.TxnResults{&structs.TxnResult{Service: entry}}, nil
		}

	case api.ServiceSet:
		if err := ensureServiceTxn(tx, idx, op.Node, false, &op.Service); err != nil {
			return nil, err
		}
		entry, err := getNodeServiceTxn(tx, nil, op.Node, op.Service.ID, &op.Service.EnterpriseMeta, op.Service.PeerName)
		return newTxnResultFromNodeServiceEntry(entry), err

	case api.ServiceCAS:
		err := ensureServiceCASTxn(tx, idx, op.Node, &op.Service)
		switch {
		case err == errCASCompareFailed:
			err := fmt.Errorf("failed to set service %q on node %q, index is stale", op.Service.ID, op.Node)
			return nil, err
		case err != nil:
			return nil, err
		}

		entry, err := getNodeServiceTxn(tx, nil, op.Node, op.Service.ID, &op.Service.EnterpriseMeta, op.Service.PeerName)
		return newTxnResultFromNodeServiceEntry(entry), err

	case api.ServiceDelete:
		err := s.deleteServiceTxn(tx, idx, op.Node, op.Service.ID, &op.Service.EnterpriseMeta, op.Service.PeerName)
		return nil, err

	case api.ServiceDeleteCAS:
		ok, err := s.deleteServiceCASTxn(tx, idx, op.Service.ModifyIndex, op.Node, op.Service.ID, &op.Service.EnterpriseMeta, op.Service.PeerName)
		if !ok && err == nil {
			return nil, fmt.Errorf("failed to delete service %q on node %q, index is stale", op.Service.ID, op.Node)
		}
		return nil, err

	default:
		return nil, fmt.Errorf("unknown Service verb %q", op.Verb)
	}
}

// newTxnResultFromNodeServiceEntry returns a TxnResults with a single result,
// a copy of entry. The entry is copied to prevent modification of the state
// store.
func newTxnResultFromNodeServiceEntry(entry *structs.NodeService) structs.TxnResults {
	if entry == nil {
		return nil
	}
	clone := *entry
	result := structs.TxnResult{Service: &clone}
	return structs.TxnResults{&result}
}

// txnCheck handles all Check-related operations.
func (s *Store) txnCheck(tx WriteTxn, idx uint64, op *structs.TxnCheckOp) (structs.TxnResults, error) {
	var entry *structs.HealthCheck
	var err error

	switch op.Verb {
	case api.CheckGet:
		_, entry, err = getNodeCheckTxn(tx, op.Check.Node, op.Check.CheckID, &op.Check.EnterpriseMeta, op.Check.PeerName)
		if entry == nil && err == nil {
			err = fmt.Errorf("check %q on node %q doesn't exist", op.Check.CheckID, op.Check.Node)
		}

	case api.CheckSet:
		err = s.ensureCheckTxn(tx, idx, false, &op.Check)
		if err == nil {
			_, entry, err = getNodeCheckTxn(tx, op.Check.Node, op.Check.CheckID, &op.Check.EnterpriseMeta, op.Check.PeerName)
		}

	case api.CheckCAS:
		var ok bool
		entry = &op.Check
		ok, err = s.ensureCheckCASTxn(tx, idx, entry)
		if !ok && err == nil {
			err = fmt.Errorf("failed to set check %q on node %q, index is stale", entry.CheckID, entry.Node)
			break
		}
		_, entry, err = getNodeCheckTxn(tx, op.Check.Node, op.Check.CheckID, &op.Check.EnterpriseMeta, op.Check.PeerName)

	case api.CheckDelete:
		err = s.deleteCheckTxn(tx, idx, op.Check.Node, op.Check.CheckID, &op.Check.EnterpriseMeta, op.Check.PeerName)

	case api.CheckDeleteCAS:
		var ok bool
		ok, err = s.deleteCheckCASTxn(tx, idx, op.Check.ModifyIndex, op.Check.Node, op.Check.CheckID, &op.Check.EnterpriseMeta, op.Check.PeerName)
		if !ok && err == nil {
			err = fmt.Errorf("failed to delete check %q on node %q, index is stale", op.Check.CheckID, op.Check.Node)
		}

	default:
		err = fmt.Errorf("unknown Check verb %q", op.Verb)
	}
	if err != nil {
		return nil, err
	}

	// For a GET we keep the value, otherwise we clone and blank out the
	// value (we have to clone so we don't modify the entry being used by
	// the state store).
	if entry != nil {
		if op.Verb == api.CheckGet {
			result := structs.TxnResult{Check: entry}
			return structs.TxnResults{&result}, nil
		}

		clone := entry.Clone()
		result := structs.TxnResult{Check: clone}
		return structs.TxnResults{&result}, nil
	}

	return nil, nil
}

// txnDispatch runs the given operations inside the state store transaction.
func (s *Store) txnDispatch(tx WriteTxn, idx uint64, ops structs.TxnOps) (structs.TxnResults, structs.TxnErrors) {
	results := make(structs.TxnResults, 0, len(ops))
	errors := make(structs.TxnErrors, 0, len(ops))
	for i, op := range ops {
		var ret structs.TxnResults
		var err error

		// Dispatch based on the type of operation.
		switch {
		case op.KV != nil:
			ret, err = s.txnKVS(tx, idx, op.KV)
		case op.Node != nil:
			ret, err = s.txnNode(tx, idx, op.Node)
		case op.Service != nil:
			ret, err = s.txnService(tx, idx, op.Service)
		case op.Check != nil:
			ret, err = s.txnCheck(tx, idx, op.Check)
		case op.Session != nil:
			err = txnSession(tx, idx, op.Session)
		case op.Intention != nil:
			// NOTE: this branch is deprecated and exists for backwards
			// compatibility with pre-1.9.0 raft logs and during upgrades.
			err = txnLegacyIntention(tx, idx, op.Intention)
		default:
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
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	results, errors := s.txnDispatch(tx, idx, ops)
	if len(errors) > 0 {
		return nil, errors
	}

	err := tx.Commit()
	if err != nil {
		return nil, structs.TxnErrors{
			{What: err.Error(), OpIndex: 0},
		}
	}
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
