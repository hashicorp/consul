package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-memdb"
)

// ReadTxn is implemented by memdb.Txn to perform read operations.
type ReadTxn interface {
	Get(table, index string, args ...interface{}) (memdb.ResultIterator, error)
	Abort()
}

// Changes wraps a memdb.Changes to include the index at which these changes
// were made.
type Changes struct {
	// Index is the latest index at the time these changes were committed.
	Index   uint64
	Changes memdb.Changes
}

// changeTrackerDB is a thin wrapper around memdb.DB which enables TrackChanges on
// all write transactions. When the transaction is committed the changes are
// sent to the eventPublisher which will create and emit change events.
type changeTrackerDB struct {
	db             *memdb.MemDB
	publisher      eventPublisher
	processChanges func(ReadTxn, Changes) ([]stream.Event, error)
}

type eventPublisher interface {
	Publish(events []stream.Event)
}

// Txn exists to maintain backwards compatibility with memdb.DB.Txn. Preexisting
// code may use it to create a read-only transaction, but it will panic if called
// with write=true.
//
// Deprecated: use either ReadTxn, or WriteTxn.
func (c *changeTrackerDB) Txn(write bool) *txn {
	if write {
		panic("don't use db.Txn(true), use db.WriteTxn(idx uin64)")
	}
	return c.ReadTxn()
}

// ReadTxn returns a read-only transaction which behaves exactly the same as
// memdb.Txn
//
// TODO: this could return a regular memdb.Txn if all the state functions accepted
// the ReadTxn interface
func (c *changeTrackerDB) ReadTxn() *txn {
	return &txn{Txn: c.db.Txn(false)}
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
func (c *changeTrackerDB) WriteTxn(idx uint64) *txn {
	t := &txn{
		Txn:     c.db.Txn(true),
		Index:   idx,
		publish: c.publish,
	}
	t.Txn.TrackChanges()
	return t
}

func (c *changeTrackerDB) publish(changes Changes) error {
	readOnlyTx := c.db.Txn(false)
	defer readOnlyTx.Abort()

	events, err := c.processChanges(readOnlyTx, changes)
	if err != nil {
		return fmt.Errorf("failed generating events from changes: %v", err)
	}
	c.publisher.Publish(events)
	return nil
}

// WriteTxnRestore returns a wrapped RW transaction that does NOT have change
// tracking enabled. This should only be used in Restore where we need to
// replace the entire contents of the Store without a need to track the changes.
// WriteTxnRestore uses a zero index since the whole restore doesn't really occur
// at one index - the effect is to write many values that were previously
// written across many indexes.
func (c *changeTrackerDB) WriteTxnRestore() *txn {
	return &txn{
		Txn:   c.db.Txn(true),
		Index: 0,
	}
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
	// read-only, or WriteTxnRestore transaction.
	// Index is stored so that it may be passed along to any subscribers as part
	// of a change event.
	Index   uint64
	publish func(changes Changes) error
}

// Commit first pushes changes to EventPublisher, then calls Commit on the
// underlying transaction.
//
// Note that this function, unlike memdb.Txn, returns an error which must be checked
// by the caller. A non-nil error indicates that a commit failed and was not
// applied.
func (tx *txn) Commit() error {
	// publish may be nil if this is a read-only or WriteTxnRestore transaction.
	// In those cases changes should also be empty, and there will be nothing
	// to publish.
	if tx.publish != nil {
		changes := Changes{
			Index:   tx.Index,
			Changes: tx.Txn.Changes(),
		}
		if err := tx.publish(changes); err != nil {
			return err
		}
	}

	tx.Txn.Commit()
	return nil
}

// TODO: may be replaced by a gRPC type.
type topic string

func (t topic) String() string {
	return string(t)
}

func processDBChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	// TODO: add other table handlers here.
	return aclChangeUnsubscribeEvent(tx, changes)
}

func newSnapshotHandlers() stream.SnapshotHandlers {
	return stream.SnapshotHandlers{}
}
