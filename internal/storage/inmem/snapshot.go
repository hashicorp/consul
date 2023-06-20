// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inmem

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Snapshot obtains a point-in-time snapshot of the store that can later be
// persisted and restored later.
func (s *Store) Snapshot() (*Snapshot, error) {
	tx := s.txn(false)

	iter, err := tx.Get(tableNameResources, indexNameID)
	if err != nil {
		return nil, err
	}

	return &Snapshot{iter: iter}, nil
}

// Snapshot is a point-in-time snapshot of a store.
type Snapshot struct {
	iter memdb.ResultIterator
}

// Next returns the next resource in the snapshot. nil will be returned when
// the end of the snapshot has been reached.
func (s *Snapshot) Next() *pbresource.Resource {
	v := s.iter.Next()
	if v == nil {
		return nil
	}
	return v.(*pbresource.Resource)
}

// Restore starts the process of restoring a snapshot.
//
// Callers *must* call Abort or Commit when done, to free resources.
func (s *Store) Restore() (*Restoration, error) {
	db, err := newDB()
	if err != nil {
		return nil, err
	}
	return &Restoration{
		s:  s,
		db: db,
		tx: db.Txn(true),
	}, nil
}

// Restoration is a handle that can be used to restore a snapshot.
type Restoration struct {
	s  *Store
	db *memdb.MemDB
	tx *memdb.Txn
}

// Apply the given resource to the store.
func (r *Restoration) Apply(res *pbresource.Resource) error {
	return r.tx.Insert(tableNameResources, res)
}

// Commit the restoration. Replaces the in-memory database wholesale and closes
// any watches.
func (r *Restoration) Commit() {
	r.tx.Commit()

	r.s.mu.Lock()
	defer r.s.mu.Unlock()

	r.s.db = r.db
	r.s.pub.RefreshTopic(eventTopic)
}

// Abort the restoration. It's safe to always call this in a defer statement
// because aborting a committed restoration is a no-op.
func (r *Restoration) Abort() { r.tx.Abort() }
