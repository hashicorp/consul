package state

import (
	"github.com/hashicorp/go-memdb"
)

// ReadTxn is implemented by memdb.Txn to perform read operations.
type ReadTxn interface {
	Get(table, index string, args ...interface{}) (memdb.ResultIterator, error)
	Abort()
}

// changeTrackerDB is a thin wrapper around memdb.DB which enables TrackChanges on
// all write transactions. When the transaction is committed the changes are
// sent to the eventPublisher which will create and emit change events.
type changeTrackerDB struct {
	db        *memdb.MemDB
	publisher changePublisher
}

type changePublisher interface {
	PublishChanges(tx *txn, changes memdb.Changes) error
}

// Txn exists to maintain backwards compatibility with memdb.DB.Txn. Preexisting
// code may use it to create a read-only transaction, but it will panic if called
// with write=true.
//
// Deprecated: use either ReadTxn, or WriteTxn.
func (db *changeTrackerDB) Txn(write bool) *txn {
	if write {
		panic("don't use db.Txn(true), use db.WriteTxn(idx uin64)")
	}
	return db.ReadTxn()
}

// ReadTxn returns a read-only transaction which behaves exactly the same as
// memdb.Txn
func (db *changeTrackerDB) ReadTxn() *txn {
	return &txn{Txn: db.db.Txn(false)}
}

// WriteTxn returns a wrapped memdb.Txn suitable for writes to the state store.
// It will track changes and publish events for the changes when Commit
// is called.
//
// The idx argument must be the index of the current Raft operation. Almost
// all mutations to state should happen as part of a raft apply so the index of
// the log being applied can be passed to WriteTxn.
// The exceptional cases are transactions that are executed on an empty
// memdb.DB as part of Restore, and those executed by tests where we insert
// data directly into the DB. These cases may use WriteTxnRestore.
func (db *changeTrackerDB) WriteTxn(idx uint64) *txn {
	t := &txn{
		Txn:       db.db.Txn(true),
		Index:     idx,
		publisher: db.publisher,
	}
	t.Txn.TrackChanges()
	return t
}

// WriteTxnRestore returns a wrapped RW transaction that does NOT have change
// tracking enabled. This should only be used in Restore where we need to
// replace the entire contents of the Store without a need to track the changes.
// WriteTxnRestore uses a zero index since the whole restore doesn't really occur
// at one index - the effect is to write many values that were previously
// written across many indexes.
func (db *changeTrackerDB) WriteTxnRestore() *txn {
	t := &txn{
		Txn:   db.db.Txn(true),
		Index: 0,
	}
	return t
}

// txn wraps a memdb.Txn to capture changes and send them to the EventPublisher.
//
// This can not be done with txn.Defer because the callback passed to Defer is
// invoked after commit completes, and because the callback can not return an
// error. Any errors from the callback would be lost,  which would result in a
// missing change event, even though the state store had changed.
type txn struct {
	*memdb.Txn
	// Index in raft where the write is occurring. The value is zero for a
	// read-only transaction, and for a WriteTxnRestore transaction.
	// Index is stored so that it may be passed along to any subscribers as part
	// of a change event.
	Index     uint64
	publisher changePublisher
}

// Commit first pushes changes to EventPublisher, then calls Commit on the
// underlying transaction.
//
// Note that this function, unlike memdb.Txn, returns an error which must be checked
// by the caller. A non-nil error indicates that a commit failed and was not
// applied.
func (tx *txn) Commit() error {
	// publisher may be nil if this is a read-only or WriteTxnRestore transaction.
	// In those cases changes should also be empty.
	if tx.publisher != nil {
		if err := tx.publisher.PublishChanges(tx, tx.Txn.Changes()); err != nil {
			return err
		}
	}

	tx.Txn.Commit()
	return nil
}
