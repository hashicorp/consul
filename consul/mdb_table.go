package consul

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/armon/gomdb"
)

var (
	noIndex       = fmt.Errorf("undefined index")
	tooManyFields = fmt.Errorf("number of fields exceeds index arity")
)

const (
	// lastIndexRowID is a special RowID used to represent the
	// last Raft index that affected the table. The index value
	// is not used by MDBTable, but is stored so that the client can map
	// back to the Raft index number
	lastIndexRowID = 0

	// deadlockTimeout is a heuristic to detect a potential MDB deadlock.
	// If we have a transaction that is left open indefinitely, it can
	// prevent new transactions from making progress and deadlocking
	// the system. If we fail to start a transaction after this long,
	// assume a potential deadlock and panic.
	deadlockTimeout = 30 * time.Second
)

/*
  An MDB table is a logical representation of a table, which is a
  generic row store. It provides a simple mechanism to store rows
  using a row id, while maintaining any number of secondary indexes.
*/
type MDBTable struct {
	// Last used rowID. Must be first to avoid 64bit alignment issues.
	lastRowID uint64

	Env     *mdb.Env
	Name    string // This is the name of the table, must be unique
	Indexes map[string]*MDBIndex
	Encoder func(interface{}) []byte
	Decoder func([]byte) interface{}
}

// MDBTables is used for when we have a collection of tables
type MDBTables []*MDBTable

// An Index is named, and uses a series of column values to
// map to the row-id containing the table
type MDBIndex struct {
	AllowBlank      bool      // Can fields be blank
	Unique          bool      // Controls if values are unique
	Fields          []string  // Fields are used to build the index
	IdxFunc         IndexFunc // Can be used to provide custom indexing
	Virtual         bool      // Virtual index does not exist, but can be used for queries
	RealIndex       string    // Virtual indexes use a RealIndex for iteration
	CaseInsensitive bool      // Controls if values are case-insensitive

	table     *MDBTable
	name      string
	dbiName   string
	realIndex *MDBIndex
}

// MDBTxn is used to wrap an underlying transaction
type MDBTxn struct {
	readonly bool
	tx       *mdb.Txn
	dbis     map[string]mdb.DBI
	after    []func()
}

// Abort is used to close the transaction
func (t *MDBTxn) Abort() {
	if t != nil && t.tx != nil {
		t.tx.Abort()
	}
}

// Commit is used to commit a transaction
func (t *MDBTxn) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return err
	}
	for _, f := range t.after {
		f()
	}
	t.after = nil
	return nil
}

// Defer is used to defer a function call until a successful commit
func (t *MDBTxn) Defer(f func()) {
	t.after = append(t.after, f)
}

type IndexFunc func(*MDBIndex, []string) string

// DefaultIndexFunc is used if no IdxFunc is provided. It joins
// the columns using '||' which is reasonably unlikely to occur.
// We also prefix with a byte to ensure we never have a zero length
// key
func DefaultIndexFunc(idx *MDBIndex, parts []string) string {
	if len(parts) == 0 {
		return "_"
	}
	prefix := "_" + strings.Join(parts, "||") + "||"
	return prefix
}

// DefaultIndexPrefixFunc can be used with DefaultIndexFunc to scan
// for index prefix values. This should only be used as part of a
// virtual index.
func DefaultIndexPrefixFunc(idx *MDBIndex, parts []string) string {
	if len(parts) == 0 {
		return "_"
	}
	prefix := "_" + strings.Join(parts, "||")
	return prefix
}

// Init is used to initialize the MDBTable and ensure it's ready
func (t *MDBTable) Init() error {
	if t.Env == nil {
		return fmt.Errorf("Missing mdb env")
	}
	if t.Name == "" {
		return fmt.Errorf("Missing table name")
	}
	if t.Indexes == nil {
		return fmt.Errorf("Missing table indexes")
	}

	// Ensure we have a unique id index
	id, ok := t.Indexes["id"]
	if !ok {
		return fmt.Errorf("Missing id index")
	}
	if !id.Unique {
		return fmt.Errorf("id index must be unique")
	}
	if id.AllowBlank {
		return fmt.Errorf("id index must not allow blanks")
	}
	if id.Virtual {
		return fmt.Errorf("id index cannot be virtual")
	}

	// Create the table
	if err := t.createTable(); err != nil {
		return fmt.Errorf("table create failed: %v", err)
	}

	// Initialize the indexes
	for name, index := range t.Indexes {
		if err := index.init(t, name); err != nil {
			return fmt.Errorf("index %s error: %s", name, err)
		}
	}

	// Get the maximum row id
	if err := t.restoreLastRowID(); err != nil {
		return fmt.Errorf("error scanning table: %s", err)
	}

	return nil
}

// createTable is used to ensure the table exists
func (t *MDBTable) createTable() error {
	tx, err := t.Env.BeginTxn(nil, 0)
	if err != nil {
		return err
	}
	if _, err := tx.DBIOpen(t.Name, mdb.CREATE); err != nil {
		tx.Abort()
		return err
	}
	return tx.Commit()
}

// restoreLastRowID is used to set the last rowID that we've used
func (t *MDBTable) restoreLastRowID() error {
	tx, err := t.StartTxn(true, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	cursor, err := tx.tx.CursorOpen(tx.dbis[t.Name])
	if err != nil {
		return err
	}
	defer cursor.Close()

	key, _, err := cursor.Get(nil, mdb.LAST)
	if err == mdb.NotFound {
		t.lastRowID = 0
		return nil
	} else if err != nil {
		return err
	}

	// Set the last row id
	t.lastRowID = bytesToUint64(key)
	return nil
}

// nextRowID returns the next usable row id
func (t *MDBTable) nextRowID() uint64 {
	return atomic.AddUint64(&t.lastRowID, 1)
}

// startTxn is used to start a transaction
func (t *MDBTable) StartTxn(readonly bool, mdbTxn *MDBTxn) (*MDBTxn, error) {
	var txFlags uint = 0
	var tx *mdb.Txn
	var err error

	// Panic if we deadlock acquiring a transaction
	timeout := time.AfterFunc(deadlockTimeout, func() {
		panic("Timeout starting MDB transaction, potential deadlock")
	})
	defer timeout.Stop()

	// Ensure the modes agree
	if mdbTxn != nil {
		if mdbTxn.readonly != readonly {
			return nil, fmt.Errorf("Cannot mix read/write transactions")
		}
		tx = mdbTxn.tx
		goto EXTEND
	}

	if readonly {
		txFlags |= mdb.RDONLY
	}

	tx, err = t.Env.BeginTxn(nil, txFlags)
	if err != nil {
		return nil, err
	}

	mdbTxn = &MDBTxn{
		readonly: readonly,
		tx:       tx,
		dbis:     make(map[string]mdb.DBI),
	}
EXTEND:
	dbi, err := tx.DBIOpen(t.Name, 0)
	if err != nil {
		tx.Abort()
		return nil, err
	}
	mdbTxn.dbis[t.Name] = dbi

	for _, index := range t.Indexes {
		if index.Virtual {
			continue
		}
		dbi, err := index.openDBI(tx)
		if err != nil {
			tx.Abort()
			return nil, err
		}
		mdbTxn.dbis[index.dbiName] = dbi
	}

	return mdbTxn, nil
}

// objIndexKeys builds the indexes for a given object
func (t *MDBTable) objIndexKeys(obj interface{}) (map[string][]byte, error) {
	// Construct the indexes keys
	indexes := make(map[string][]byte)
	for name, index := range t.Indexes {
		if index.Virtual {
			continue
		}
		key, err := index.keyFromObject(obj)
		if err != nil {
			return nil, err
		}
		indexes[name] = key
	}
	return indexes, nil
}

// Insert is used to insert or update an object
func (t *MDBTable) Insert(obj interface{}) error {
	// Start a new txn
	tx, err := t.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := t.InsertTxn(tx, obj); err != nil {
		return err
	}
	return tx.Commit()
}

// Insert is used to insert or update an object within
// a given transaction
func (t *MDBTable) InsertTxn(tx *MDBTxn, obj interface{}) error {
	var n int
	// Construct the indexes keys
	indexes, err := t.objIndexKeys(obj)
	if err != nil {
		return err
	}

	// Encode the obj
	raw := t.Encoder(obj)

	// Scan and check if this primary key already exists
	primaryDbi := tx.dbis[t.Indexes["id"].dbiName]
	_, err = tx.tx.Get(primaryDbi, indexes["id"])
	if err == mdb.NotFound {
		goto AFTER_DELETE
	}

	// Delete the existing row
	n, err = t.deleteWithIndex(tx, t.Indexes["id"], indexes["id"])
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("unexpected number of updates: %d", n)
	}

AFTER_DELETE:
	// Insert with a new row ID
	rowId := t.nextRowID()
	encRowId := uint64ToBytes(rowId)
	table := tx.dbis[t.Name]
	if err := tx.tx.Put(table, encRowId, raw, 0); err != nil {
		return err
	}

	// Insert the new indexes
	for name, index := range t.Indexes {
		if index.Virtual {
			continue
		}
		dbi := tx.dbis[index.dbiName]
		if err := tx.tx.Put(dbi, indexes[name], encRowId, 0); err != nil {
			return err
		}
	}
	return nil
}

// Get is used to lookup one or more rows. An index an appropriate
// fields are specified. The fields can be a prefix of the index.
func (t *MDBTable) Get(index string, parts ...string) (uint64, []interface{}, error) {
	// Start a readonly txn
	tx, err := t.StartTxn(true, nil)
	if err != nil {
		return 0, nil, err
	}
	defer tx.Abort()

	// Get the last associated index
	idx, err := t.LastIndexTxn(tx)
	if err != nil {
		return 0, nil, err
	}

	// Get the actual results
	res, err := t.GetTxn(tx, index, parts...)
	return idx, res, err
}

// GetTxn is like Get but it operates within a specific transaction.
// This can be used for read that span multiple tables
func (t *MDBTable) GetTxn(tx *MDBTxn, index string, parts ...string) ([]interface{}, error) {
	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return nil, err
	}

	// Accumulate the results
	var results []interface{}
	err = idx.iterate(tx, key, func(encRowId, res []byte) (bool, bool) {
		obj := t.Decoder(res)
		results = append(results, obj)
		return false, false
	})

	return results, err
}

// GetTxnLimit is like GetTxn limits the maximum number of
// rows it will return
func (t *MDBTable) GetTxnLimit(tx *MDBTxn, limit int, index string, parts ...string) ([]interface{}, error) {
	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return nil, err
	}

	// Accumulate the results
	var results []interface{}
	num := 0
	err = idx.iterate(tx, key, func(encRowId, res []byte) (bool, bool) {
		num++
		obj := t.Decoder(res)
		results = append(results, obj)
		return false, num == limit
	})

	return results, err
}

// StreamTxn is like GetTxn but it streams the results over a channel.
// This can be used if the expected data set is very large. The stream
// is always closed on return.
func (t *MDBTable) StreamTxn(stream chan<- interface{}, tx *MDBTxn, index string, parts ...string) error {
	// Always close the stream on return
	defer close(stream)

	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return err
	}

	// Stream the results
	err = idx.iterate(tx, key, func(encRowId, res []byte) (bool, bool) {
		obj := t.Decoder(res)
		stream <- obj
		return false, false
	})

	return err
}

// getIndex is used to get the proper index, and also check the arity
func (t *MDBTable) getIndex(index string, parts []string) (*MDBIndex, []byte, error) {
	// Get the index
	idx, ok := t.Indexes[index]
	if !ok {
		return nil, nil, noIndex
	}

	// Check the arity
	arity := idx.arity()
	if len(parts) > arity {
		return nil, nil, tooManyFields
	}

	if idx.CaseInsensitive {
		parts = ToLowerList(parts)
	}

	// Construct the key
	key := idx.keyFromParts(parts...)
	return idx, key, nil
}

// Delete is used to delete one or more rows. An index an appropriate
// fields are specified. The fields can be a prefix of the index.
// Returns the rows deleted or an error.
func (t *MDBTable) Delete(index string, parts ...string) (num int, err error) {
	// Start a write txn
	tx, err := t.StartTxn(false, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Abort()

	num, err = t.DeleteTxn(tx, index, parts...)
	if err != nil {
		return 0, err
	}
	return num, tx.Commit()
}

// DeleteTxn is like Delete, but occurs in a specific transaction
// that can span multiple tables.
func (t *MDBTable) DeleteTxn(tx *MDBTxn, index string, parts ...string) (int, error) {
	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return 0, err
	}

	// Delete with the index
	return t.deleteWithIndex(tx, idx, key)
}

// deleteWithIndex deletes all associated rows while scanning
// a given index for a key prefix. May perform multiple index traversals.
// This is a hack around a bug in LMDB which can cause a partial delete to
// take place. To fix this, we invoke the innerDelete until all rows are
// removed. This hack can be removed once the LMDB bug is resolved.
func (t *MDBTable) deleteWithIndex(tx *MDBTxn, idx *MDBIndex, key []byte) (int, error) {
	var total int
	var num int
	var err error
DELETE:
	num, err = t.innerDeleteWithIndex(tx, idx, key)
	total += num
	if err != nil {
		return total, err
	}
	if num > 0 {
		goto DELETE
	}
	return total, nil
}

// innerDeleteWithIndex deletes all associated rows while scanning
// a given index for a key prefix. It only traverses the index a single time.
func (t *MDBTable) innerDeleteWithIndex(tx *MDBTxn, idx *MDBIndex, key []byte) (num int, err error) {
	// Handle an error while deleting
	defer func() {
		if r := recover(); r != nil {
			num = 0
			err = fmt.Errorf("Panic while deleting: %v", r)
		}
	}()

	// Delete everything as we iterate
	err = idx.iterate(tx, key, func(encRowId, res []byte) (bool, bool) {
		// Get the object
		obj := t.Decoder(res)

		// Build index values
		indexes, err := t.objIndexKeys(obj)
		if err != nil {
			panic(err)
		}

		// Delete the indexes we are not iterating
		for name, otherIdx := range t.Indexes {
			if name == idx.name {
				continue
			}
			if idx.Virtual && name == idx.RealIndex {
				continue
			}
			if otherIdx.Virtual {
				continue
			}
			dbi := tx.dbis[otherIdx.dbiName]
			if err := tx.tx.Del(dbi, indexes[name], encRowId); err != nil {
				panic(err)
			}
		}

		// Delete the data row
		if err := tx.tx.Del(tx.dbis[t.Name], encRowId, nil); err != nil {
			panic(err)
		}

		// Delete the object
		num++
		return true, false
	})
	if err != nil {
		return 0, err
	}

	// Return the deleted count
	return num, nil
}

// Initializes an index and returns a potential error
func (i *MDBIndex) init(table *MDBTable, name string) error {
	i.table = table
	i.name = name
	i.dbiName = fmt.Sprintf("%s_%s_idx", i.table.Name, i.name)
	if i.IdxFunc == nil {
		i.IdxFunc = DefaultIndexFunc
	}
	if len(i.Fields) == 0 {
		return fmt.Errorf("index missing fields")
	}
	if err := i.createIndex(); err != nil {
		return err
	}
	// Verify real index exists
	if i.Virtual {
		if realIndex, ok := table.Indexes[i.RealIndex]; !ok {
			return fmt.Errorf("real index '%s' missing", i.RealIndex)
		} else {
			i.realIndex = realIndex
		}
	}
	return nil
}

// createIndex is used to ensure the index exists
func (i *MDBIndex) createIndex() error {
	// Do not create if this is a virtual index
	if i.Virtual {
		return nil
	}
	tx, err := i.table.Env.BeginTxn(nil, 0)
	if err != nil {
		return err
	}
	var dbFlags uint = mdb.CREATE
	if !i.Unique {
		dbFlags |= mdb.DUPSORT
	}
	if _, err := tx.DBIOpen(i.dbiName, dbFlags); err != nil {
		tx.Abort()
		return err
	}
	return tx.Commit()
}

// openDBI is used to open a handle to the index for a transaction
func (i *MDBIndex) openDBI(tx *mdb.Txn) (mdb.DBI, error) {
	var dbFlags uint
	if !i.Unique {
		dbFlags |= mdb.DUPSORT
	}
	return tx.DBIOpen(i.dbiName, dbFlags)
}

// Returns the arity of the index
func (i *MDBIndex) arity() int {
	return len(i.Fields)
}

// keyFromObject constructs the index key from the object
func (i *MDBIndex) keyFromObject(obj interface{}) ([]byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Derefence the pointer if any
	parts := make([]string, 0, i.arity())
	for _, field := range i.Fields {
		fv := v.FieldByName(field)
		if !fv.IsValid() {
			return nil, fmt.Errorf("Field '%s' for %#v is invalid", field, obj)
		}
		val := fv.String()
		if !i.AllowBlank && val == "" {
			return nil, fmt.Errorf("Field '%s' must be set: %#v", field, obj)
		}
		if i.CaseInsensitive {
			val = strings.ToLower(val)
		}
		parts = append(parts, val)
	}
	key := i.keyFromParts(parts...)
	return key, nil
}

// keyFromParts returns the key from component parts
func (i *MDBIndex) keyFromParts(parts ...string) []byte {
	return []byte(i.IdxFunc(i, parts))
}

// iterate is used to iterate over keys matching the prefix,
// and invoking the cb with each row. We dereference the rowid,
// and only return the object row
func (i *MDBIndex) iterate(tx *MDBTxn, prefix []byte,
	cb func(encRowId, res []byte) (bool, bool)) error {
	table := tx.dbis[i.table.Name]

	// If virtual, use the correct DBI
	var dbi mdb.DBI
	if i.Virtual {
		dbi = tx.dbis[i.realIndex.dbiName]
	} else {
		dbi = tx.dbis[i.dbiName]
	}

	cursor, err := tx.tx.CursorOpen(dbi)
	if err != nil {
		return err
	}
	// Read-only cursors are NOT closed by MDB when a transaction
	// either commits or aborts, so must be closed explicitly
	if tx.readonly {
		defer cursor.Close()
	}

	var key, encRowId, objBytes []byte
	first := true
	shouldStop := false
	shouldDelete := false
	for !shouldStop {
		if first && len(prefix) > 0 {
			first = false
			key, encRowId, err = cursor.Get(prefix, mdb.SET_RANGE)
		} else if shouldDelete {
			key, encRowId, err = cursor.Get(nil, mdb.GET_CURRENT)
			shouldDelete = false

			// LMDB will return EINVAL(22) for the GET_CURRENT op if
			// there is no further keys. We treat this as no more
			// keys being found.
			if num, ok := err.(mdb.Errno); ok && num == 22 {
				err = mdb.NotFound
			}
		} else if i.Unique {
			key, encRowId, err = cursor.Get(nil, mdb.NEXT)
		} else {
			key, encRowId, err = cursor.Get(nil, mdb.NEXT_DUP)
			if err == mdb.NotFound {
				key, encRowId, err = cursor.Get(nil, mdb.NEXT)
			}
		}
		if err == mdb.NotFound {
			break
		} else if err != nil {
			return fmt.Errorf("iterate failed: %v", err)
		}

		// Bail if this does not match our filter
		if len(prefix) > 0 && !bytes.HasPrefix(key, prefix) {
			break
		}

		// Lookup the actual object
		objBytes, err = tx.tx.Get(table, encRowId)
		if err != nil {
			return fmt.Errorf("rowid lookup failed: %v (%v)", err, encRowId)
		}

		// Invoke the cb
		shouldDelete, shouldStop = cb(encRowId, objBytes)
		if shouldDelete {
			if err := cursor.Del(0); err != nil {
				return fmt.Errorf("delete failed: %v", err)
			}
		}
	}
	return nil
}

// LastIndex is get the last index that updated the table
func (t *MDBTable) LastIndex() (uint64, error) {
	// Start a readonly txn
	tx, err := t.StartTxn(true, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Abort()
	return t.LastIndexTxn(tx)
}

// LastIndexTxn is like LastIndex but it operates within a specific transaction.
func (t *MDBTable) LastIndexTxn(tx *MDBTxn) (uint64, error) {
	encRowId := uint64ToBytes(lastIndexRowID)
	val, err := tx.tx.Get(tx.dbis[t.Name], encRowId)
	if err == mdb.NotFound {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	// Return the last index
	return bytesToUint64(val), nil
}

// SetLastIndex is used to set the last index that updated the table
func (t *MDBTable) SetLastIndex(index uint64) error {
	tx, err := t.StartTxn(false, nil)
	if err != nil {
		return err
	}
	defer tx.Abort()

	if err := t.SetLastIndexTxn(tx, index); err != nil {
		return err
	}
	return tx.Commit()
}

// SetLastIndexTxn is used to set the last index within a transaction
func (t *MDBTable) SetLastIndexTxn(tx *MDBTxn, index uint64) error {
	encRowId := uint64ToBytes(lastIndexRowID)
	encIndex := uint64ToBytes(index)
	return tx.tx.Put(tx.dbis[t.Name], encRowId, encIndex, 0)
}

// SetMaxLastIndexTxn is used to set the last index within a transaction
// if it exceeds the current maximum
func (t *MDBTable) SetMaxLastIndexTxn(tx *MDBTxn, index uint64) error {
	current, err := t.LastIndexTxn(tx)
	if err != nil {
		return err
	}
	if index > current {
		return t.SetLastIndexTxn(tx, index)
	}
	return nil
}

// StartTxn is used to create a transaction that spans a list of tables
func (t MDBTables) StartTxn(readonly bool) (*MDBTxn, error) {
	var tx *MDBTxn
	for _, table := range t {
		newTx, err := table.StartTxn(readonly, tx)
		if err != nil {
			tx.Abort()
			return nil, err
		}
		tx = newTx
	}
	return tx, nil
}

// LastIndexTxn is used to get the last transaction from all of the tables
func (t MDBTables) LastIndexTxn(tx *MDBTxn) (uint64, error) {
	var index uint64
	for _, table := range t {
		idx, err := table.LastIndexTxn(tx)
		if err != nil {
			return index, err
		}
		if idx > index {
			index = idx
		}
	}
	return index, nil
}
