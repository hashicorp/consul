package state

import (
	"github.com/hashicorp/go-memdb"
)

// memDBWrapper is a thin shim over memdb.DB which forces all new tranactions to
// report changes and for those changes to automatically deliver the changed
// objects to our central event handler in case new streaming events need to be
// omitted.
type memDBWrapper struct {
	*memdb.MemDB
	// TODO: add publisher
}

// Txn intercepts MemDB.Txn(). It allows read-only transactions to pass through
// but fails write transactions as we now need to force callers to use a
// slightly different API so that change capture can happen automatically. The
// returned Txn object is wrapped with a no-op wrapper that just keeps all
// transactions in our state store the same type. The wrapper only has
// non-passthrough behavior for write transactions though.
func (db *memDBWrapper) Txn(write bool) *txnWrapper {
	if write {
		panic("don't use db.Txn(true), use db.WriteTxn(idx uin64)")
	}
	return &txnWrapper{
		Txn: db.MemDB.Txn(false),
	}
}

// WriteTxn returns a wrapped memdb.Txn suitable for writes to the state store.
// It will track changes and publish events for them automatically when Commit
// is called. The idx argument should be the index of the currently operating
// Raft operation. Almost all mutations to state should happen as part of a raft
// apply so the index of that log being applied should be plumbed through to
// here. A few exceptional cases are transactions that are only executed on a
// fresh memdb store during a Restore and a few places in tests where we insert
// data directly into the DB. These cases may use WriteTxnRestore.
func (db *memDBWrapper) WriteTxn(idx uint64) *txnWrapper {
	t := &txnWrapper{
		Txn:   db.MemDB.Txn(true),
		Index: idx,
	}
	t.Txn.TrackChanges()
	return t
}

// WriteTxnRestore returns a wrapped RW transaction that does NOT have change
// tracking enabled. This should only be used in Restore where we need to
// replace the entire contents of the Store without a need to track the changes
// made and emit events. Subscribers will all reset on a restore and start
// again. It also uses a zero index since the whole restore doesn't really occur
// at one index - the effect is to write many values that were previously
// written across many indexes.
func (db *memDBWrapper) WriteTxnRestore() *txnWrapper {
	t := &txnWrapper{
		Txn:   db.MemDB.Txn(true),
		Index: 0,
	}
	return t
}

// txnWrapper wraps a memdb.Txn to automatically capture changes and process
// events for the write as it commits. This can't be done just with txn.Defer
// because errors the callback is invoked after commit completes so errors
// during event publishing would cause silent dropped events while the state
// store still changed and the write looked successful from the outside.
type txnWrapper struct {
	// Index stores the index the write is occuring at in raft if this is a write
	// transaction. If it's zero it means this is a read transaction.
	Index uint64
	*memdb.Txn

	store *Store
}

// Commit overrides Commit on the underlying memdb.Txn and causes events to be
// published for any changes. Note that it has a different signature to
// memdb.Txn - returning an error that should be checked since an error implies
// that something prevented the commit from completing.
//
// TODO: currently none of the callers check error, should they all be changed?
func (tx *txnWrapper) Commit() error {
	//changes := tx.Txn.Changes()
	// TODO: publish changes

	tx.Txn.Commit()
	return nil
}
