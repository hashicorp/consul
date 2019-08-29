package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/types"
	memdb "github.com/hashicorp/go-memdb"
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

	// ErrMissingACLTokenSecret is returned when a token set is called on a
	// token with an empty SecretID.
	ErrMissingACLTokenSecret = errors.New("Missing ACL Token SecretID")

	// ErrMissingACLTokenAccessor is returned when a token set is called on a
	// token with an empty AccessorID.
	ErrMissingACLTokenAccessor = errors.New("Missing ACL Token AccessorID")

	// ErrTokenHasNoPrivileges is returned when a token set is called on a
	// token with no policies, roles, or service identities and the caller
	// requires at least one to be set.
	ErrTokenHasNoPrivileges = errors.New("Token has no privileges")

	// ErrMissingACLPolicyID is returned when a policy set is called on a
	// policy with an empty ID.
	ErrMissingACLPolicyID = errors.New("Missing ACL Policy ID")

	// ErrMissingACLPolicyName is returned when a policy set is called on a
	// policy with an empty Name.
	ErrMissingACLPolicyName = errors.New("Missing ACL Policy Name")

	// ErrMissingACLRoleID is returned when a role set is called on
	// a role with an empty ID.
	ErrMissingACLRoleID = errors.New("Missing ACL Role ID")

	// ErrMissingACLRoleName is returned when a role set is called on
	// a role with an empty Name.
	ErrMissingACLRoleName = errors.New("Missing ACL Role Name")

	// ErrMissingACLBindingRuleID is returned when a binding rule set
	// is called on a binding rule with an empty ID.
	ErrMissingACLBindingRuleID = errors.New("Missing ACL Binding Rule ID")

	// ErrMissingACLBindingRuleAuthMethod is returned when a binding rule set
	// is called on a binding rule with an empty AuthMethod.
	ErrMissingACLBindingRuleAuthMethod = errors.New("Missing ACL Binding Rule Auth Method")

	// ErrMissingACLAuthMethodName is returned when an auth method set is
	// called on an auth method with an empty Name.
	ErrMissingACLAuthMethodName = errors.New("Missing ACL Auth Method Name")

	// ErrMissingACLAuthMethodType is returned when an auth method set is
	// called on an auth method with an empty Type.
	ErrMissingACLAuthMethodType = errors.New("Missing ACL Auth Method Type")

	// ErrMissingQueryID is returned when a Query set is called on
	// a Query with an empty ID.
	ErrMissingQueryID = errors.New("Missing Query ID")

	// ErrMissingCARootID is returned when an CARoot set is called
	// with an CARoot with an empty ID.
	ErrMissingCARootID = errors.New("Missing CA Root ID")

	// ErrMissingIntentionID is returned when an Intention set is called
	// with an Intention with an empty ID.
	ErrMissingIntentionID = errors.New("Missing Intention ID")
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

// Store is where we store all of Consul's state, including
// records of node registrations, services, checks, key/value
// pairs and more. The DB is entirely in-memory and is constructed
// from the Raft log through the FSM.
type Store struct {
	schema *memdb.DBSchema
	db     *memdb.MemDB

	// abandonCh is used to signal watchers that this state store has been
	// abandoned (usually during a restore). This is only ever closed.
	abandonCh chan struct{}

	// kvsGraveyard manages tombstones for the key value store.
	kvsGraveyard *Graveyard

	// lockDelay holds expiration times for locks associated with keys.
	lockDelay *Delay
}

// Snapshot is used to provide a point-in-time snapshot. It
// works by starting a read transaction against the whole state store.
type Snapshot struct {
	store     *Store
	tx        *memdb.Txn
	lastIndex uint64
}

// Restore is used to efficiently manage restoring a large amount of
// data to a state store.
type Restore struct {
	store *Store
	tx    *memdb.Txn
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
func NewStateStore(gc *TombstoneGC) (*Store, error) {
	// Create the in-memory DB.
	schema := stateStoreSchema()
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, fmt.Errorf("Failed setting up state store: %s", err)
	}

	// Create and return the state store.
	s := &Store{
		schema:       schema,
		db:           db,
		abandonCh:    make(chan struct{}),
		kvsGraveyard: NewGraveyard(gc),
		lockDelay:    NewDelay(),
	}
	return s, nil
}

// Snapshot is used to create a point-in-time snapshot of the entire db.
func (s *Store) Snapshot() *Snapshot {
	tx := s.db.Txn(false)

	var tables []string
	for table := range s.schema.Tables {
		tables = append(tables, table)
	}
	idx := maxIndexTxn(tx, tables...)

	return &Snapshot{s, tx, idx}
}

// LastIndex returns that last index that affects the snapshotted data.
func (s *Snapshot) LastIndex() uint64 {
	return s.lastIndex
}

func (s *Snapshot) Indexes() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("index", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// IndexRestore is used to restore an index
func (s *Restore) IndexRestore(idx *IndexEntry) error {
	if err := s.tx.Insert("index", idx); err != nil {
		return fmt.Errorf("index insert failed: %v", err)
	}
	return nil
}

// Close performs cleanup of a state snapshot.
func (s *Snapshot) Close() {
	s.tx.Abort()
}

// Restore is used to efficiently manage restoring a large amount of data into
// the state store. It works by doing all the restores inside of a single
// transaction.
func (s *Store) Restore() *Restore {
	tx := s.db.Txn(true)
	return &Restore{s, tx}
}

// Abort abandons the changes made by a restore. This or Commit should always be
// called.
func (s *Restore) Abort() {
	s.tx.Abort()
}

// Commit commits the changes made by a restore. This or Abort should always be
// called.
func (s *Restore) Commit() {
	s.tx.Commit()
}

// AbandonCh returns a channel you can wait on to know if the state store was
// abandoned.
func (s *Store) AbandonCh() <-chan struct{} {
	return s.abandonCh
}

// Abandon is used to signal that the given state store has been abandoned.
// Calling this more than one time will panic.
func (s *Store) Abandon() {
	close(s.abandonCh)
}

// maxIndex is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func (s *Store) maxIndex(tables ...string) uint64 {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return maxIndexTxn(tx, tables...)
}

// maxIndexTxn is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func maxIndexTxn(tx *memdb.Txn, tables ...string) uint64 {
	return maxIndexWatchTxn(tx, nil, tables...)
}

func maxIndexWatchTxn(tx *memdb.Txn, ws memdb.WatchSet, tables ...string) uint64 {
	var lindex uint64
	for _, table := range tables {
		ch, ti, err := tx.FirstWatch("index", "id", table)
		if err != nil {
			panic(fmt.Sprintf("unknown index: %s err: %s", table, err))
		}
		if idx, ok := ti.(*IndexEntry); ok && idx.Value > lindex {
			lindex = idx.Value
		}
		ws.Add(ch)
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
