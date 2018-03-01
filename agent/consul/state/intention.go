package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

const (
	intentionsTableName = "connect-intentions"
)

// intentionsTableSchema returns a new table schema used for storing
// intentions for Connect.
func intentionsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: intentionsTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"destination": &memdb.IndexSchema{
				Name:         "destination",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "DestinationNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "DestinationName",
							Lowercase: true,
						},
					},
				},
			},
			"source": &memdb.IndexSchema{
				Name:         "source",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "SourceNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "SourceName",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

func init() {
	registerSchema(intentionsTableSchema)
}

// Intentions returns the list of all intentions.
func (s *Store) Intentions(ws memdb.WatchSet) (uint64, structs.Intentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the index
	idx := maxIndexTxn(tx, intentionsTableName)

	// Get all intentions
	iter, err := tx.Get(intentionsTableName, "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed intention lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results structs.Intentions
	for ixn := iter.Next(); ixn != nil; ixn = iter.Next() {
		results = append(results, ixn.(*structs.Intention))
	}
	return idx, results, nil
}

// IntentionSet creates or updates an intention.
func (s *Store) IntentionSet(idx uint64, ixn *structs.Intention) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.intentionSetTxn(tx, idx, ixn); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// intentionSetTxn is the inner method used to insert an intention with
// the proper indexes into the state store.
func (s *Store) intentionSetTxn(tx *memdb.Txn, idx uint64, ixn *structs.Intention) error {
	// ID is required
	if ixn.ID == "" {
		return ErrMissingIntentionID
	}

	// Check for an existing intention
	existing, err := tx.First(intentionsTableName, "id", ixn.ID)
	if err != nil {
		return fmt.Errorf("failed intention looup: %s", err)
	}
	if existing != nil {
		ixn.CreateIndex = existing.(*structs.Intention).CreateIndex
	} else {
		ixn.CreateIndex = idx
	}
	ixn.ModifyIndex = idx

	// Insert
	if err := tx.Insert(intentionsTableName, ixn); err != nil {
		return err
	}
	if err := tx.Insert("index", &IndexEntry{intentionsTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// IntentionGet returns the given intention by ID.
func (s *Store) IntentionGet(ws memdb.WatchSet, id string) (uint64, *structs.Intention, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, intentionsTableName)

	// Look up by its ID.
	watchCh, intention, err := tx.FirstWatch(intentionsTableName, "id", id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed intention lookup: %s", err)
	}
	ws.Add(watchCh)

	// Convert the interface{} if it is non-nil
	var result *structs.Intention
	if intention != nil {
		result = intention.(*structs.Intention)
	}

	return idx, result, nil
}

// IntentionDelete deletes the given intention by ID.
func (s *Store) IntentionDelete(idx uint64, id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.intentionDeleteTxn(tx, idx, id); err != nil {
		return fmt.Errorf("failed intention delete: %s", err)
	}

	tx.Commit()
	return nil
}

// intentionDeleteTxn is the inner method used to delete a intention
// with the proper indexes into the state store.
func (s *Store) intentionDeleteTxn(tx *memdb.Txn, idx uint64, queryID string) error {
	// Pull the query.
	wrapped, err := tx.First(intentionsTableName, "id", queryID)
	if err != nil {
		return fmt.Errorf("failed intention lookup: %s", err)
	}
	if wrapped == nil {
		return nil
	}

	// Delete the query and update the index.
	if err := tx.Delete(intentionsTableName, wrapped); err != nil {
		return fmt.Errorf("failed intention delete: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{intentionsTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}
