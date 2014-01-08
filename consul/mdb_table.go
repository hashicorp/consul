package consul

import (
	"bytes"
	"fmt"
	"github.com/armon/gomdb"
	"reflect"
	"strings"
	"sync/atomic"
)

var (
	noIndex       = fmt.Errorf("undefined index")
	tooManyFields = fmt.Errorf("number of fields exceeds index arity")
)

/*
  An MDB table is a logical representation of a table, which is a
  generic row store. It provides a simple mechanism to store rows
  using a "row id", but then accesses can be done using any number
  of named indexes
*/
type MDBTable struct {
	lastRowID uint64 // Last used rowID
	Env       *mdb.Env
	Name      string // This is the name of the table, must be unique
	Indexes   map[string]*MDBIndex
	Encoder   func(interface{}) []byte
	Decoder   func([]byte) interface{}
}

// An Index is named, and uses a series of column values to
// map to the row-id containing the table
type MDBIndex struct {
	AllowBlank bool      // Can fields be blank
	Unique     bool      // Controls if values are unique
	Fields     []string  // Fields are used to build the index
	IdxFunc    IndexFunc // Can be used to provide custom indexing

	table   *MDBTable
	name    string
	dbiName string
}

// MDBTxn is used to wrap an underlying transaction
type MDBTxn struct {
	readonly bool
	tx       *mdb.Txn
	dbis     map[string]mdb.DBI
}

// Abort is used to close the transaction
func (t *MDBTxn) Abort() {
	t.tx.Abort()
}

// Commit is used to commit a transaction
func (t *MDBTxn) Commit() error {
	return t.tx.Commit()
}

type RowID uint64
type IndexFunc func([]string) string

// DefaultIndexFunc is used if no IdxFunc is provided. It joins
// the columns using '||' which is reasonably unlikely to occur.
// We also prefix with a byte to ensure we never have a zero length
// key
func DefaultIndexFunc(parts []string) string {
	return "_" + strings.Join(parts, "||")
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
	tx, err := t.StartTxn(true)
	if err != nil {
		return err
	}
	defer tx.Abort()

	cursor, err := tx.tx.CursorOpen(tx.dbis[t.Name])
	if err != nil {
		return err
	}

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
func (t *MDBTable) StartTxn(readonly bool) (*MDBTxn, error) {
	var txFlags uint = 0
	if readonly {
		txFlags |= mdb.RDONLY
	}

	tx, err := t.Env.BeginTxn(nil, txFlags)
	if err != nil {
		return nil, err
	}

	mdbTxn := &MDBTxn{
		readonly: readonly,
		tx:       tx,
		dbis:     make(map[string]mdb.DBI),
	}

	dbi, err := tx.DBIOpen(t.Name, 0)
	if err != nil {
		tx.Abort()
		return nil, err
	}
	mdbTxn.dbis[t.Name] = dbi

	for _, index := range t.Indexes {
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
	// Construct the indexes keys
	indexes, err := t.objIndexKeys(obj)
	if err != nil {
		return err
	}

	// Encode the obj
	raw := t.Encoder(obj)

	// Start a new txn
	tx, err := t.StartTxn(false)
	if err != nil {
		return err
	}
	defer tx.Abort()

	// TODO: Handle updates

	// Insert with a new row ID
	rowId := t.nextRowID()
	encRowId := uint64ToBytes(rowId)
	table := tx.dbis[t.Name]
	if err := tx.tx.Put(table, encRowId, raw, 0); err != nil {
		return err
	}

	// Insert the new indexes
	for name, index := range t.Indexes {
		dbi := tx.dbis[index.dbiName]
		if err := tx.tx.Put(dbi, indexes[name], encRowId, 0); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Get is used to lookup one or more rows. An index an appropriate
// fields are specified. The fields can be a prefix of the index.
func (t *MDBTable) Get(index string, parts ...string) ([]interface{}, error) {
	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return nil, err
	}

	// Start a readonly txn
	tx, err := t.StartTxn(true)
	if err != nil {
		return nil, err
	}
	defer tx.Abort()

	// Accumulate the results
	var results []interface{}
	err = idx.iterate(tx, key, func(encRowId, res []byte) bool {
		obj := t.Decoder(res)
		results = append(results, obj)
		return false
	})

	return results, err
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

	// Construct the key
	key := []byte(idx.IdxFunc(parts))
	return idx, key, nil
}

// Delete is used to delete one or more rows. An index an appropriate
// fields are specified. The fields can be a prefix of the index.
// Returns the rows deleted or an error.
func (t *MDBTable) Delete(index string, parts ...string) (num int, err error) {
	// Get the associated index
	idx, key, err := t.getIndex(index, parts)
	if err != nil {
		return 0, err
	}

	// Start a write txn
	tx, err := t.StartTxn(false)
	if err != nil {
		return 0, err
	}
	defer tx.Abort()

	// Handle an error while deleting
	defer func() {
		if r := recover(); r != nil {
			num = 0
			err = err
		}
	}()

	// Delete everything as we iterate
	err = idx.iterate(tx, key, func(encRowId, res []byte) bool {
		// Get the object
		obj := t.Decoder(res)

		// Build index values
		indexes, err := t.objIndexKeys(obj)
		if err != nil {
			panic(err)
		}

		// Delete the indexes we are not iterating
		for name, otherIdx := range t.Indexes {
			if name == index {
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
		return true
	})
	if err != nil {
		return 0, err
	}

	// Return the deleted count
	return num, tx.Commit()
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
	return nil
}

// createIndex is used to ensure the index exists
func (i *MDBIndex) createIndex() error {
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
		parts = append(parts, val)
	}
	key := i.IdxFunc(parts)
	return []byte(key), nil
}

// iterate is used to iterate over keys matching the prefix,
// and invoking the cb with each row. We dereference the rowid,
// and only return the object row
func (i *MDBIndex) iterate(tx *MDBTxn, prefix []byte,
	cb func(encRowId, res []byte) bool) error {
	table := tx.dbis[i.table.Name]
	dbi := tx.dbis[i.dbiName]

	cursor, err := tx.tx.CursorOpen(dbi)
	if err != nil {
		return err
	}

	var key, encRowId, objBytes []byte
	first := true
	shouldDelete := false
	for {
		if first && len(prefix) > 0 {
			first = false
			key, encRowId, err = cursor.Get(prefix, mdb.SET_RANGE)
		} else if shouldDelete {
			key, encRowId, err = cursor.Get(nil, 0)
			shouldDelete = false
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
		if shouldDelete = cb(encRowId, objBytes); shouldDelete {
			if err := cursor.Del(0); err != nil {
				return fmt.Errorf("delete failed: %v", err)
			}
		}
	}
	return nil
}
