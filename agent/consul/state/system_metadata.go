// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const tableSystemMetadata = "system-metadata"

func systemMetadataTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableSystemMetadata,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: true,
				},
			},
		},
	}
}

// SystemMetadataEntries used to pull all the system metadata entries for the snapshot.
func (s *Snapshot) SystemMetadataEntries() ([]*structs.SystemMetadataEntry, error) {
	entries, err := s.tx.Get(tableSystemMetadata, "id")
	if err != nil {
		return nil, err
	}

	var ret []*structs.SystemMetadataEntry
	for wrapped := entries.Next(); wrapped != nil; wrapped = entries.Next() {
		ret = append(ret, wrapped.(*structs.SystemMetadataEntry))
	}

	return ret, nil
}

// SystemMetadataEntry is used when restoring from a snapshot.
func (s *Restore) SystemMetadataEntry(entry *structs.SystemMetadataEntry) error {
	// Insert
	if err := s.tx.Insert(tableSystemMetadata, entry); err != nil {
		return fmt.Errorf("failed restoring system metadata object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, entry.ModifyIndex, tableSystemMetadata); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// SystemMetadataSet is called to do an upsert of a set of system metadata entries.
func (s *Store) SystemMetadataSet(idx uint64, entry *structs.SystemMetadataEntry) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := systemMetadataSetTxn(tx, idx, entry); err != nil {
		return err
	}

	return tx.Commit()
}

// systemMetadataSetTxn upserts a system metadata inside of a transaction.
func systemMetadataSetTxn(tx WriteTxn, idx uint64, entry *structs.SystemMetadataEntry) error {
	// The only validation we care about is non-empty keys.
	if entry.Key == "" {
		return fmt.Errorf("missing key on system metadata")
	}

	// Check for existing.
	var existing *structs.SystemMetadataEntry
	existingRaw, err := tx.First(tableSystemMetadata, "id", entry.Key)
	if err != nil {
		return fmt.Errorf("failed system metadata lookup: %s", err)
	}

	if existingRaw != nil {
		existing = existingRaw.(*structs.SystemMetadataEntry)
	}

	// Set the indexes
	if existing != nil {
		entry.CreateIndex = existing.CreateIndex
		entry.ModifyIndex = idx
	} else {
		entry.CreateIndex = idx
		entry.ModifyIndex = idx
	}

	// Insert the system metadata and update the index
	if err := tx.Insert(tableSystemMetadata, entry); err != nil {
		return fmt.Errorf("failed inserting system metadata: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableSystemMetadata, idx}); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}

// SystemMetadataGet is called to get a system metadata.
func (s *Store) SystemMetadataGet(ws memdb.WatchSet, key string) (uint64, *structs.SystemMetadataEntry, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return systemMetadataGetTxn(tx, ws, key)
}

func systemMetadataGetTxn(tx ReadTxn, ws memdb.WatchSet, key string) (uint64, *structs.SystemMetadataEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableSystemMetadata)

	// Get the existing contents.
	watchCh, existing, err := tx.FirstWatch(tableSystemMetadata, "id", key)
	if err != nil {
		return 0, nil, fmt.Errorf("failed system metadata lookup: %s", err)
	}
	ws.Add(watchCh)

	if existing == nil {
		return idx, nil, nil
	}

	entry, ok := existing.(*structs.SystemMetadataEntry)
	if !ok {
		return 0, nil, fmt.Errorf("system metadata %q is an invalid type: %T", key, existing)
	}

	return idx, entry, nil
}

// SystemMetadataList is called to get all system metadata objects.
func (s *Store) SystemMetadataList(ws memdb.WatchSet) (uint64, []*structs.SystemMetadataEntry, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return systemMetadataListTxn(tx, ws)
}

func systemMetadataListTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, []*structs.SystemMetadataEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableSystemMetadata)

	iter, err := tx.Get(tableSystemMetadata, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed system metadata lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results []*structs.SystemMetadataEntry
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(*structs.SystemMetadataEntry))
	}
	return idx, results, nil
}

func (s *Store) SystemMetadataDelete(idx uint64, entry *structs.SystemMetadataEntry) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := systemMetadataDeleteTxn(tx, idx, entry.Key); err != nil {
		return err
	}

	return tx.Commit()
}

func systemMetadataDeleteTxn(tx WriteTxn, idx uint64, key string) error {
	// Try to retrieve the existing system metadata.
	existing, err := tx.First(tableSystemMetadata, "id", key)
	if err != nil {
		return fmt.Errorf("failed system metadata lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// Delete the system metadata from the DB and update the index.
	if err := tx.Delete(tableSystemMetadata, existing); err != nil {
		return fmt.Errorf("failed removing system metadata: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableSystemMetadata, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}
