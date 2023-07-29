// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableKVs        = "kvs"
	tableTombstones = "tombstones"

	indexSession = "session"
)

// kvsTableSchema returns a new table schema used for storing structs.DirEntry
func kvsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableKVs,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer:      kvsIndexer(),
			},
			indexSession: {
				Name:         indexSession,
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "Session",
				},
			},
		},
	}
}

// indexFromIDValue creates an index key from any struct that implements singleValueID
func indexFromIDValue(e singleValueID) ([]byte, error) {
	v := e.IDValue()
	if v == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(v)
	return b.Bytes(), nil
}

// tombstonesTableSchema returns a new table schema used for storing tombstones
// during KV delete operations to prevent the index from sliding backwards.
func tombstonesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableTombstones,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer:      kvsIndexer(),
			},
		},
	}
}

// KVs is used to pull the full list of KVS entries for use during snapshots.
func (s *Snapshot) KVs() (memdb.ResultIterator, error) {
	return s.tx.Get(tableKVs, indexID+"_prefix")
}

// Tombstones is used to pull all the tombstones from the graveyard.
func (s *Snapshot) Tombstones() (memdb.ResultIterator, error) {
	return s.store.kvsGraveyard.DumpTxn(s.tx)
}

// KVS is used when restoring from a snapshot. Use KVSSet for general inserts.
func (s *Restore) KVS(entry *structs.DirEntry) error {
	if err := insertKVTxn(s.tx, entry, true, true); err != nil {
		return fmt.Errorf("failed inserting kvs entry: %s", err)
	}

	return nil
}

// Tombstone is used when restoring from a snapshot. For general inserts, use
// Graveyard.InsertTxn.
func (s *Restore) Tombstone(stone *Tombstone) error {
	if err := s.store.kvsGraveyard.RestoreTxn(s.tx, stone); err != nil {
		return fmt.Errorf("failed restoring tombstone: %s", err)
	}
	return nil
}

// ReapTombstones is used to delete all the tombstones with an index
// less than or equal to the given index. This is used to prevent
// unbounded storage growth of the tombstones.
func (s *Store) ReapTombstones(idx uint64, index uint64) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.kvsGraveyard.ReapTxn(tx, index); err != nil {
		return fmt.Errorf("failed to reap kvs tombstones: %s", err)
	}

	return tx.Commit()
}

// KVSSet is used to store a key/value pair.
func (s *Store) KVSSet(idx uint64, entry *structs.DirEntry) error {
	entry.EnterpriseMeta.Normalize()
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual set.
	if err := kvsSetTxn(tx, idx, entry, false); err != nil {
		return err
	}

	return tx.Commit()
}

// kvsSetTxn is used to insert or update a key/value pair in the state
// store. It is the inner method used and handles only the actual storage.
// If updateSession is true, then the incoming entry will set the new
// session (should be validated before calling this). Otherwise, we will keep
// whatever the existing session is.
func kvsSetTxn(tx WriteTxn, idx uint64, entry *structs.DirEntry, updateSession bool) error {
	existingNode, err := tx.First(tableKVs, indexID, entry)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}
	existing, _ := existingNode.(*structs.DirEntry)

	// Set the CreateIndex.
	if existing != nil {
		entry.CreateIndex = existing.CreateIndex
	} else {
		entry.CreateIndex = idx
	}

	// Preserve the existing session unless told otherwise. The "existing"
	// session for a new entry is "no session".
	if !updateSession {
		if existing != nil {
			entry.Session = existing.Session
		} else {
			entry.Session = ""
		}
	}

	// Set the ModifyIndex.
	if existing != nil && existing.Equal(entry) {
		// Skip further writing in the state store if the entry is not actually
		// changed. Nevertheless, the input's ModifyIndex should be reset
		// since the TXN API returns a copy in the response.
		entry.ModifyIndex = existing.ModifyIndex
		return nil
	}
	entry.ModifyIndex = idx

	// Store the kv pair in the state store and update the index.
	if err := insertKVTxn(tx, entry, false, false); err != nil {
		return fmt.Errorf("failed inserting kvs entry: %s", err)
	}

	return nil
}

// KVSGet is used to retrieve a key/value pair from the state store.
func (s *Store) KVSGet(ws memdb.WatchSet, key string, entMeta *acl.EnterpriseMeta) (uint64, *structs.DirEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer entMeta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	return kvsGetTxn(tx, ws, key, *entMeta)
}

// kvsGetTxn is the inner method that gets a KVS entry inside an existing
// transaction.
func kvsGetTxn(tx ReadTxn,
	ws memdb.WatchSet, key string, entMeta acl.EnterpriseMeta) (uint64, *structs.DirEntry, error) {

	// Get the table index.
	idx := kvsMaxIndex(tx, entMeta)

	watchCh, entry, err := tx.FirstWatch(tableKVs, indexID, Query{Value: key, EnterpriseMeta: entMeta})
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	ws.Add(watchCh)
	if entry != nil {
		return idx, entry.(*structs.DirEntry), nil
	}
	return idx, nil, nil
}

// KVSList is used to list out all keys under a given prefix. If the
// prefix is left empty, all keys in the KVS will be returned. The returned
// is the max index of the returned kvs entries or applicable tombstones, or
// else it's the full table indexes for kvs and tombstones.
func (s *Store) KVSList(ws memdb.WatchSet,
	prefix string, entMeta *acl.EnterpriseMeta, modifyIndexAbove uint64) (uint64, structs.DirEntries, error) {

	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer entMeta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	return s.kvsListTxn(tx, ws, prefix, *entMeta, modifyIndexAbove)
}

// kvsListTxn is the inner method that gets a list of KVS entries matching a
// prefix.
func (s *Store) kvsListTxn(tx ReadTxn,
	ws memdb.WatchSet, prefix string, entMeta acl.EnterpriseMeta, modifyIndexAbove uint64) (uint64, structs.DirEntries, error) {

	// Get the table indexes.
	idx := kvsMaxIndex(tx, entMeta)

	lindex, entries, err := kvsListEntriesTxn(tx, ws, prefix, entMeta, modifyIndexAbove)
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Check for the highest index in the graveyard. If the prefix is empty
	// then just use the full table indexes since we are listing everything.
	if prefix != "" {
		gindex, tombentries, err := s.kvsGraveyard.GetMaxIndexTxn(tx, prefix, &entMeta, modifyIndexAbove)
		if err != nil {
			return 0, nil, fmt.Errorf("failed graveyard lookup: %s", err)
		}
		if gindex > lindex {
			lindex = gindex
		}
		entries = append(entries, tombentries...)
	} else {
		lindex = idx
	}

	// Use the sub index if it was set and there are entries, otherwise use
	// the full table index from above.
	if lindex != 0 {
		idx = lindex
	}
	return idx, entries, nil
}

// KVSDelete is used to perform a shallow delete on a single key in the
// the state store.
func (s *Store) KVSDelete(idx uint64, key string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual delete
	if err := s.kvsDeleteTxn(tx, idx, key, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// kvsDeleteTxn is the inner method used to perform the actual deletion
// of a key/value pair within an existing transaction.
func (s *Store) kvsDeleteTxn(tx WriteTxn, idx uint64, key string, entMeta *acl.EnterpriseMeta) error {

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Look up the entry in the state store.
	entry, err := tx.First(tableKVs, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil
	}

	// Create a tombstone.
	if err := s.kvsGraveyard.InsertTxn(tx, key, idx, entMeta); err != nil {
		return fmt.Errorf("failed adding to graveyard: %s", err)
	}

	return kvsDeleteWithEntry(tx, entry.(*structs.DirEntry), idx)
}

// KVSDeleteCAS is used to try doing a KV delete operation with a given
// raft index. If the CAS index specified is not equal to the last
// observed index for the given key, then the call is a noop, otherwise
// a normal KV delete is invoked.
func (s *Store) KVSDeleteCAS(idx, cidx uint64, key string, entMeta *acl.EnterpriseMeta) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	set, err := s.kvsDeleteCASTxn(tx, idx, cidx, key, entMeta)
	if !set || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// kvsDeleteCASTxn is the inner method that does a CAS delete within an existing
// transaction.
func (s *Store) kvsDeleteCASTxn(tx WriteTxn, idx, cidx uint64, key string, entMeta *acl.EnterpriseMeta) (bool, error) {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	entry, err := tx.First(tableKVs, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := entry.(*structs.DirEntry)
	if !ok || e.ModifyIndex != cidx {
		return entry == nil, nil
	}

	// Call the actual deletion if the above passed.
	if err := s.kvsDeleteTxn(tx, idx, key, entMeta); err != nil {
		return false, err
	}
	return true, nil
}

// KVSSetCAS is used to do a check-and-set operation on a KV entry. The
// ModifyIndex in the provided entry is used to determine if we should
// write the entry to the state store or bail. Returns a bool indicating
// if a write happened and any error.
func (s *Store) KVSSetCAS(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	set, err := kvsSetCASTxn(tx, idx, entry)
	if !set || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// kvsSetCASTxn is the inner method used to do a CAS inside an existing
// transaction.
func kvsSetCASTxn(tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	existing, err := tx.First(tableKVs, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if entry.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if entry.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	e, ok := existing.(*structs.DirEntry)
	if ok && entry.ModifyIndex != 0 && entry.ModifyIndex != e.ModifyIndex {
		return false, nil
	}

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(tx, idx, entry, false); err != nil {
		return false, err
	}
	return true, nil
}

// KVSDeleteTree is used to do a recursive delete on a key prefix
// in the state store. If any keys are modified, the last index is
// set, otherwise this is a no-op.
func (s *Store) KVSDeleteTree(idx uint64, prefix string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.kvsDeleteTreeTxn(tx, idx, prefix, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// KVSLockDelay returns the expiration time for any lock delay associated with
// the given key.
func (s *Store) KVSLockDelay(key string, entMeta *acl.EnterpriseMeta) time.Time {
	return s.lockDelay.GetExpiration(key, entMeta)
}

// KVSLock is similar to KVSSet but only performs the set if the lock can be
// acquired.
func (s *Store) KVSLock(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	locked, err := kvsLockTxn(tx, idx, entry)
	if !locked || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// kvsLockTxn is the inner method that does a lock inside an existing
// transaction.
func kvsLockTxn(tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	// Verify that a session is present.
	if entry.Session == "" {
		return false, fmt.Errorf("missing session")
	}

	// Verify that the session exists.
	sess, err := tx.First(tableSessions, indexID, Query{Value: entry.Session, EnterpriseMeta: entry.EnterpriseMeta})
	if err != nil {
		return false, fmt.Errorf("failed session lookup: %s", err)
	}
	if sess == nil {
		return false, fmt.Errorf("invalid session %#v", entry.Session)
	}

	existing, err := tx.First(tableKVs, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Set up the entry, using the existing entry if present.
	if existing != nil {
		e := existing.(*structs.DirEntry)
		if e.Session == entry.Session {
			// We already hold this lock, good to go.
			entry.CreateIndex = e.CreateIndex
			entry.LockIndex = e.LockIndex
		} else if e.Session != "" {
			// Bail out, someone else holds this lock.
			return false, nil
		} else {
			// Set up a new lock with this session.
			entry.CreateIndex = e.CreateIndex
			entry.LockIndex = e.LockIndex + 1
		}
	} else {
		entry.CreateIndex = idx
		entry.LockIndex = 1
	}
	entry.ModifyIndex = idx

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(tx, idx, entry, true); err != nil {
		return false, err
	}
	return true, nil
}

// KVSUnlock is similar to KVSSet but only performs the set if the lock can be
// unlocked (the key must already exist and be locked).
func (s *Store) KVSUnlock(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	unlocked, err := kvsUnlockTxn(tx, idx, entry)
	if !unlocked || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// kvsUnlockTxn is the inner method that does an unlock inside an existing
// transaction.
func kvsUnlockTxn(tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	// Verify that a session is present.
	if entry.Session == "" {
		return false, fmt.Errorf("missing session")
	}

	existing, err := tx.First(tableKVs, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Bail if there's no existing key.
	if existing == nil {
		return false, nil
	}

	// Make sure the given session is the lock holder.
	e := existing.(*structs.DirEntry)
	if e.Session != entry.Session {
		return false, nil
	}

	// Clear the lock and update the entry.
	entry.Session = ""
	entry.LockIndex = e.LockIndex
	entry.CreateIndex = e.CreateIndex
	entry.ModifyIndex = idx

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(tx, idx, entry, true); err != nil {
		return false, err
	}
	return true, nil
}

// kvsCheckSessionTxn checks to see if the given session matches the current
// entry for a key.
func kvsCheckSessionTxn(tx WriteTxn,
	key string, session string, entMeta *acl.EnterpriseMeta) (*structs.DirEntry, error) {

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	entry, err := tx.First(tableKVs, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil, fmt.Errorf("failed to check session, key %q doesn't exist", key)
	}

	e := entry.(*structs.DirEntry)
	if e.Session != session {
		return nil, fmt.Errorf("failed session check for key %q, current session %q != %q", key, e.Session, session)
	}

	return e, nil
}

// kvsCheckIndexTxn checks to see if the given modify index matches the current
// entry for a key.
func kvsCheckIndexTxn(tx WriteTxn,
	key string, cidx uint64, entMeta acl.EnterpriseMeta) (*structs.DirEntry, error) {

	entry, err := tx.First(tableKVs, indexID, Query{Value: key, EnterpriseMeta: entMeta})
	if err != nil {
		return nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil, fmt.Errorf("failed to check index, key %q doesn't exist", key)
	}

	e := entry.(*structs.DirEntry)
	if e.ModifyIndex != cidx {
		return nil, fmt.Errorf("failed index check for key %q, current modify index %d != %d", key, e.ModifyIndex, cidx)
	}

	return e, nil
}
