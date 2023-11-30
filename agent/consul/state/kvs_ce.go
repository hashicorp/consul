// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package state

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func kvsIndexer() indexerSingleWithPrefix[singleValueID, singleValueID, any] {
	return indexerSingleWithPrefix[singleValueID, singleValueID, any]{
		readIndex:   indexFromIDValue,
		writeIndex:  indexFromIDValue,
		prefixIndex: prefixIndexForIDValue,
	}
}

func prefixIndexForIDValue(arg interface{}) ([]byte, error) {
	switch v := arg.(type) {
	// DeletePrefix always uses a string, pass it along unmodified
	case string:
		return []byte(v), nil
	case acl.EnterpriseMeta:
		return nil, nil
	case singleValueID:
		var b indexBuilder
		if v.IDValue() != "" {
			// Omit null terminator, because we want to prefix match keys
			b.String(v.IDValue())
		}
		prefix := bytes.Trim(b.Bytes(), "\x00")
		return prefix, nil
	}
	return nil, fmt.Errorf("unexpected type %T for singleValueID prefix index", arg)
}

func insertKVTxn(tx WriteTxn, entry *structs.DirEntry, updateMax bool, _ bool) error {
	if err := tx.Insert(tableKVs, entry); err != nil {
		return err
	}

	if updateMax {
		if err := indexUpdateMaxTxn(tx, entry.ModifyIndex, tableKVs); err != nil {
			return fmt.Errorf("failed updating kvs index: %v", err)
		}
	} else {
		if err := tx.Insert(tableIndex, &IndexEntry{tableKVs, entry.ModifyIndex}); err != nil {
			return fmt.Errorf("failed updating kvs index: %s", err)
		}
	}
	return nil
}

func kvsListEntriesTxn(tx ReadTxn, ws memdb.WatchSet, prefix string, entMeta acl.EnterpriseMeta) (uint64, structs.DirEntries, error) {
	var ents structs.DirEntries
	var lindex uint64

	entries, err := tx.Get(tableKVs, indexID+"_prefix", prefix)
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	ws.Add(entries.WatchCh())

	// Gather all of the keys found
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		e := entry.(*structs.DirEntry)
		ents = append(ents, e)
		if e.ModifyIndex > lindex {
			lindex = e.ModifyIndex
		}
	}
	return lindex, ents, nil
}

// kvsDeleteTreeTxn is the inner method that does a recursive delete inside an
// existing transaction.
func (s *Store) kvsDeleteTreeTxn(tx WriteTxn, idx uint64, prefix string, entMeta *acl.EnterpriseMeta) error {
	// For prefix deletes, only insert one tombstone and delete the entire subtree
	deleted, err := tx.DeletePrefix(tableKVs, indexID+"_prefix", prefix)
	if err != nil {
		return fmt.Errorf("failed recursive deleting kvs entry: %s", err)
	}

	if deleted {
		if prefix != "" { // don't insert a tombstone if the entire tree is deleted, all watchers on keys will see the max_index of the tree
			if err := s.kvsGraveyard.InsertTxn(tx, prefix, idx, entMeta); err != nil {
				return fmt.Errorf("failed adding to graveyard: %s", err)
			}
		}

		if err := tx.Insert(tableIndex, &IndexEntry{"kvs", idx}); err != nil {
			return fmt.Errorf("failed updating index: %s", err)
		}
	}
	return nil
}

func kvsMaxIndex(tx ReadTxn, entMeta acl.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "kvs", "tombstones")
}

func kvsDeleteWithEntry(tx WriteTxn, entry *structs.DirEntry, idx uint64) error {
	// Delete the entry and update the index.
	if err := tx.Delete(tableKVs, entry); err != nil {
		return fmt.Errorf("failed deleting kvs entry: %s", err)
	}

	if err := tx.Insert(tableIndex, &IndexEntry{tableKVs, idx}); err != nil {
		return fmt.Errorf("failed updating kvs index: %s", err)
	}

	return nil
}
