package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

const (
	caRootTableName = "connect-ca-roots"
)

// caRootTableSchema returns a new table schema used for storing
// CA roots for Connect.
func caRootTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: caRootTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
		},
	}
}

func init() {
	registerSchema(caRootTableSchema)
}

// CARoots returns the list of all CA roots.
func (s *Store) CARoots(ws memdb.WatchSet) (uint64, structs.CARoots, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, caRootTableName)

	// Get all
	iter, err := tx.Get(caRootTableName, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed CA root lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.CARoots
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(*structs.CARoot))
	}
	return idx, results, nil
}

// CARootActive returns the currently active CARoot.
func (s *Store) CARootActive(ws memdb.WatchSet) (uint64, *structs.CARoot, error) {
	// Get all the roots since there should never be that many and just
	// do the filtering in this method.
	var result *structs.CARoot
	idx, roots, err := s.CARoots(ws)
	if err == nil {
		for _, r := range roots {
			if r.Active {
				result = r
				break
			}
		}
	}

	return idx, result, err
}

// CARootSet creates or updates a CA root.
//
// NOTE(mitchellh): I have a feeling we'll want a CARootMultiSetCAS to
// perform a check-and-set on the entire set of CARoots versus an individual
// set, since we'll want to modify them atomically during events such as
// rotation.
func (s *Store) CARootSet(idx uint64, v *structs.CARoot) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.caRootSetTxn(tx, idx, v); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// caRootSetTxn is the inner method used to insert or update a CA root with
// the proper indexes into the state store.
func (s *Store) caRootSetTxn(tx *memdb.Txn, idx uint64, v *structs.CARoot) error {
	// ID is required
	if v.ID == "" {
		return ErrMissingCARootID
	}

	// Check for an existing value
	existing, err := tx.First(caRootTableName, "id", v.ID)
	if err != nil {
		return fmt.Errorf("failed CA root lookup: %s", err)
	}
	if existing != nil {
		old := existing.(*structs.CARoot)
		v.CreateIndex = old.CreateIndex
	} else {
		v.CreateIndex = idx
	}
	v.ModifyIndex = idx

	// Insert
	if err := tx.Insert(caRootTableName, v); err != nil {
		return err
	}
	if err := tx.Insert("index", &IndexEntry{caRootTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}
