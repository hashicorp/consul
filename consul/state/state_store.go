package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-memdb"
)

var (
	// ErrMissingNode is the error returned when trying an operation
	// which requires a node registration but none exists.
	ErrMissingNode = errors.New("Missing node registration")

	// ErrMissingService is the error we return if trying an
	// operation which requires a service but none exists.
	ErrMissingService = errors.New("Missing service registration")

	// ErrMissingSessionID is returned when a session registration
	// is attempted with an empty session ID.
	ErrMissingSessionID = errors.New("Missing session ID")

	// ErrMissingACLID is returned when an ACL set is called on
	// an ACL with an empty ID.
	ErrMissingACLID = errors.New("Missing ACL ID")

	// ErrMissingQueryID is returned when a Query set is called on
	// a Query with an empty ID.
	ErrMissingQueryID = errors.New("Missing Query ID")
)

const (
	// watchLimit is used as a soft limit to cap how many watches we allow
	// for a given blocking query. If this is exceeded, then we will use a
	// higher-level watch that's less fine-grained. This isn't as bad as it
	// seems since we have made the main culprits (nodes and services) more
	// efficient by diffing before we update via register requests.
	//
	// Given the current size of aFew == 32 in memdb's watch_few.go, this
	// will allow for up to ~64 goroutines per blocking query.
	watchLimit = 2048
)

// StateStore is where we store all of Consul's state, including
// records of node registrations, services, checks, key/value
// pairs and more. The DB is entirely in-memory and is constructed
// from the Raft log through the FSM.
type StateStore struct {
	schema *memdb.DBSchema
	db     *memdb.MemDB

	// tableWatches holds all the full table watches, indexed by table name.
	tableWatches map[string]*FullTableWatch

	// kvsWatch holds the special prefix watch for the key value store.
	kvsWatch *PrefixWatchManager

	// kvsGraveyard manages tombstones for the key value store.
	kvsGraveyard *Graveyard

	// lockDelay holds expiration times for locks associated with keys.
	lockDelay *Delay
}

// StateSnapshot is used to provide a point-in-time snapshot. It
// works by starting a read transaction against the whole state store.
type StateSnapshot struct {
	store     *StateStore
	tx        *memdb.Txn
	lastIndex uint64
}

// StateRestore is used to efficiently manage restoring a large amount of
// data to a state store.
type StateRestore struct {
	store   *StateStore
	tx      *memdb.Txn
	watches *DumbWatchManager
}

// IndexEntry keeps a record of the last index per-table.
type IndexEntry struct {
	Key   string
	Value uint64
}

// sessionCheck is used to create a many-to-one table such that
// each check registered by a session can be mapped back to the
// session table. This is only used internally in the state
// store and thus it is not exported.
type sessionCheck struct {
	Node    string
	CheckID types.CheckID
	Session string
}

// NewStateStore creates a new in-memory state storage layer.
func NewStateStore(gc *TombstoneGC) (*StateStore, error) {
	// Create the in-memory DB.
	schema := stateStoreSchema()
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, fmt.Errorf("Failed setting up state store: %s", err)
	}

	// Build up the all-table watches.
	tableWatches := make(map[string]*FullTableWatch)
	for table, _ := range schema.Tables {
		if table == "kvs" || table == "tombstones" {
			continue
		}

		tableWatches[table] = NewFullTableWatch()
	}

	// Create and return the state store.
	s := &StateStore{
		schema:       schema,
		db:           db,
		tableWatches: tableWatches,
		kvsWatch:     NewPrefixWatchManager(),
		kvsGraveyard: NewGraveyard(gc),
		lockDelay:    NewDelay(),
	}
	return s, nil
}

// Snapshot is used to create a point-in-time snapshot of the entire db.
func (s *StateStore) Snapshot() *StateSnapshot {
	tx := s.db.Txn(false)

	var tables []string
	for table, _ := range s.schema.Tables {
		tables = append(tables, table)
	}
	idx := maxIndexTxn(tx, tables...)

	return &StateSnapshot{s, tx, idx}
}

// LastIndex returns that last index that affects the snapshotted data.
func (s *StateSnapshot) LastIndex() uint64 {
	return s.lastIndex
}

// Close performs cleanup of a state snapshot.
func (s *StateSnapshot) Close() {
	s.tx.Abort()
}

// Restore is used to efficiently manage restoring a large amount of data into
// the state store. It works by doing all the restores inside of a single
// transaction.
func (s *StateStore) Restore() *StateRestore {
	tx := s.db.Txn(true)
	watches := NewDumbWatchManager(s.tableWatches)
	return &StateRestore{s, tx, watches}
}

// Abort abandons the changes made by a restore. This or Commit should always be
// called.
func (s *StateRestore) Abort() {
	s.tx.Abort()
}

// Commit commits the changes made by a restore. This or Abort should always be
// called.
func (s *StateRestore) Commit() {
	// Fire off a single KVS watch instead of a zillion prefix ones, and use
	// a dumb watch manager to single-fire all the full table watches.
	s.tx.Defer(func() { s.store.kvsWatch.Notify("", true) })
	s.tx.Defer(func() { s.watches.Notify() })

	s.tx.Commit()
}

// maxIndex is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func (s *StateStore) maxIndex(tables ...string) uint64 {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return maxIndexTxn(tx, tables...)
}

// maxIndexTxn is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func maxIndexTxn(tx *memdb.Txn, tables ...string) uint64 {
	var lindex uint64
	for _, table := range tables {
		ti, err := tx.First("index", "id", table)
		if err != nil {
			panic(fmt.Sprintf("unknown index: %s err: %s", table, err))
		}
		if idx, ok := ti.(*IndexEntry); ok && idx.Value > lindex {
			lindex = idx.Value
		}
	}
	return lindex
}

// indexUpdateMaxTxn is used when restoring entries and sets the table's index to
// the given idx only if it's greater than the current index.
func indexUpdateMaxTxn(tx *memdb.Txn, idx uint64, table string) error {
	ti, err := tx.First("index", "id", table)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing index: %s", err)
	}

	// Always take the first update, otherwise do the > check.
	if ti == nil {
		if err := tx.Insert("index", &IndexEntry{table, idx}); err != nil {
			return fmt.Errorf("failed updating index %s", err)
		}
	} else if cur, ok := ti.(*IndexEntry); ok && idx > cur.Value {
		if err := tx.Insert("index", &IndexEntry{table, idx}); err != nil {
			return fmt.Errorf("failed updating index %s", err)
		}
	}

	return nil
}

// getWatchTables returns the list of tables that should be watched and used for
// max index calculations for the given query method. This is used for all
// methods except for KVS. This will panic if the method is unknown.
func (s *StateStore) getWatchTables(method string) []string {
	switch method {
	case "GetNode", "Nodes":
		return []string{"nodes"}
	case "Services":
		return []string{"services"}
	case "NodeService", "NodeServices", "ServiceNodes":
		return []string{"nodes", "services"}
	case "NodeCheck", "NodeChecks", "ServiceChecks", "ChecksInState":
		return []string{"checks"}
	case "ChecksInStateByNodeMeta", "ServiceChecksByNodeMeta":
		return []string{"nodes", "checks"}
	case "CheckServiceNodes", "NodeInfo", "NodeDump":
		return []string{"nodes", "services", "checks"}
	case "SessionGet", "SessionList", "NodeSessions":
		return []string{"sessions"}
	case "ACLGet", "ACLList":
		return []string{"acls"}
	case "Coordinates":
		return []string{"coordinates"}
	case "PreparedQueryGet", "PreparedQueryResolve", "PreparedQueryList":
		return []string{"prepared-queries"}
	}

	panic(fmt.Sprintf("Unknown method %s", method))
}

// getTableWatch returns a full table watch for the given table. This will panic
// if the table doesn't have a full table watch.
func (s *StateStore) getTableWatch(table string) Watch {
	if watch, ok := s.tableWatches[table]; ok {
		return watch
	}

	panic(fmt.Sprintf("Unknown watch for table %s", table))
}

// GetQueryWatch returns a watch for the given query method. This is
// used for all methods except for KV; you should call GetKVSWatch instead.
// This will panic if the method is unknown.
func (s *StateStore) GetQueryWatch(method string) Watch {
	tables := s.getWatchTables(method)
	if len(tables) == 1 {
		return s.getTableWatch(tables[0])
	}

	var watches []Watch
	for _, table := range tables {
		watches = append(watches, s.getTableWatch(table))
	}
	return NewMultiWatch(watches...)
}

// GetKVSWatch returns a watch for the given prefix in the key value store.
func (s *StateStore) GetKVSWatch(prefix string) Watch {
	return s.kvsWatch.NewPrefixWatch(prefix)
}
