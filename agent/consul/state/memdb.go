package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// ReadTxn is implemented by memdb.Txn to perform read operations.
type ReadTxn interface {
	Get(table, index string, args ...interface{}) (memdb.ResultIterator, error)
	First(table, index string, args ...interface{}) (interface{}, error)
	FirstWatch(table, index string, args ...interface{}) (<-chan struct{}, interface{}, error)
}

// AbortTxn is a ReadTxn that can also be aborted to end the transaction.
type AbortTxn interface {
	ReadTxn
	Abort()
}

// ReadDB is a DB that provides read-only transactions.
type ReadDB interface {
	ReadTxn() AbortTxn
}

// WriteTxn is implemented by memdb.Txn to perform write operations.
type WriteTxn interface {
	ReadTxn
	Defer(func())
	Delete(table string, obj interface{}) error
	DeleteAll(table, index string, args ...interface{}) (int, error)
	DeletePrefix(table string, index string, prefix string) (bool, error)
	Insert(table string, obj interface{}) error
}

// Changes wraps a memdb.Changes to include the index at which these changes
// were made.
type Changes struct {
	// Index is the latest index at the time these changes were committed.
	Index   uint64
	Changes memdb.Changes
}

// changeTrackerDB is a thin wrapper around memdb.DB which enables TrackChanges on
// all write transactions. When the transaction is committed the changes are:
// 1. Used to update our internal usage tracking
// 2. Sent to the eventPublisher which will create and emit change events
type changeTrackerDB struct {
	db             *memdb.MemDB
	publisher      EventPublisher
	processChanges func(ReadTxn, Changes) ([]stream.Event, error)
}

type EventPublisher interface {
	Publish([]stream.Event)
	RegisterHandler(stream.Topic, stream.SnapshotFunc, bool) error
	Subscribe(*stream.SubscribeRequest) (*stream.Subscription, error)
}

// Txn exists to maintain backwards compatibility with memdb.DB.Txn. Preexisting
// code may use it to create a read-only transaction, but it will panic if called
// with write=true.
//
// Deprecated: use either ReadTxn, or WriteTxn.
func (c *changeTrackerDB) Txn(write bool) *memdb.Txn {
	if write {
		panic("don't use db.Txn(true), use db.WriteTxn(idx uin64)")
	}
	return c.ReadTxn()
}

// ReadTxn returns a read-only transaction.
func (c *changeTrackerDB) ReadTxn() *memdb.Txn {
	return c.db.Txn(false)
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

func (c *changeTrackerDB) publish(tx ReadTxn, changes Changes) error {
	events, err := c.processChanges(tx, changes)
	if err != nil {
		return fmt.Errorf("failed generating events from changes: %v", err)
	}
	c.publisher.Publish(events)
	return nil
}

// WriteTxnRestore returns a wrapped RW transaction that should only be used in
// Restore where we need to replace the entire contents of the Store.
// WriteTxnRestore uses a zero index since the whole restore doesn't really
// occur at one index - the effect is to write many values that were previously
// written across many indexes. WriteTxnRestore also does not publish any
// change events to subscribers.
func (c *changeTrackerDB) WriteTxnRestore() *txn {
	t := &txn{
		Txn:   c.db.Txn(true),
		Index: 0,
	}

	// We enable change tracking so that usage data is correctly populated.
	t.Txn.TrackChanges()
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
	// read-only, or WriteTxnRestore transaction.
	// Index is stored so that it may be passed along to any subscribers as part
	// of a change event.
	Index   uint64
	publish func(tx ReadTxn, changes Changes) error
}

// Commit first pushes changes to EventPublisher, then calls Commit on the
// underlying transaction.
//
// Note that this function, unlike memdb.Txn, returns an error which must be checked
// by the caller. A non-nil error indicates that a commit failed and was not
// applied.
func (tx *txn) Commit() error {
	changes := Changes{
		Index:   tx.Index,
		Changes: tx.Txn.Changes(),
	}

	if len(changes.Changes) > 0 {
		if err := updateUsage(tx, changes); err != nil {
			return err
		}
	}

	// publish may be nil if this is a read-only or WriteTxnRestore transaction.
	// In those cases changes should also be empty, and there will be nothing
	// to publish.
	if tx.publish != nil {
		if err := tx.publish(tx.Txn, changes); err != nil {
			return err
		}
	}

	tx.Txn.Commit()
	return nil
}

type readDB memdb.MemDB

func (db *readDB) ReadTxn() AbortTxn {
	return (*memdb.MemDB)(db).Txn(false)
}

var (
	EventTopicServiceHealth        = pbsubscribe.Topic_ServiceHealth
	EventTopicServiceHealthConnect = pbsubscribe.Topic_ServiceHealthConnect
	EventTopicMeshConfig           = pbsubscribe.Topic_MeshConfig
	EventTopicServiceResolver      = pbsubscribe.Topic_ServiceResolver
	EventTopicIngressGateway       = pbsubscribe.Topic_IngressGateway
	EventTopicServiceIntentions    = pbsubscribe.Topic_ServiceIntentions
	EventTopicServiceList          = pbsubscribe.Topic_ServiceList
)

func processDBChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	fns := []func(tx ReadTxn, changes Changes) ([]stream.Event, error){
		aclChangeUnsubscribeEvent,
		caRootsChangeEvents,
		ServiceHealthEventsFromChanges,
		ServiceListUpdateEventsFromChanges,
		ConfigEntryEventsFromChanges,
		// TODO: add other table handlers here.
	}
	for _, fn := range fns {
		e, err := fn(tx, changes)
		if err != nil {
			return nil, err
		}
		events = append(events, e...)
	}
	return events, nil
}
