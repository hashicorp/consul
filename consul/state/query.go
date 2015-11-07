package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/go-memdb"
)

// Queries is used to pull all the prepared queries from the snapshot.
func (s *StateSnapshot) Queries() (memdb.ResultIterator, error) {
	iter, err := s.tx.Get("queries", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// Query is used when restoring from a snapshot. For general inserts, use
// QuerySet.
func (s *StateRestore) Query(query *structs.PreparedQuery) error {
	if err := s.tx.Insert("queries", query); err != nil {
		return fmt.Errorf("failed restoring query: %s", err)
	}

	if err := indexUpdateMaxTxn(s.tx, query.ModifyIndex, "queries"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	s.watches.Arm("queries")
	return nil
}

// QuerySet is used to create or update a prepared query.
func (s *StateStore) QuerySet(idx uint64, query *structs.PreparedQuery) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Call set on the Query.
	if err := s.querySetTxn(tx, idx, query); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// querySetTxn is the inner method used to insert a prepared query with the
// proper indexes into the state store.
func (s *StateStore) querySetTxn(tx *memdb.Txn, idx uint64, query *structs.PreparedQuery) error {
	// Check that the ID is set.
	if query.ID == "" {
		return ErrMissingQueryID
	}

	// Check for an existing query.
	existing, err := tx.First("queries", "id", query.ID)
	if err != nil {
		return fmt.Errorf("failed query lookup: %s", err)
	}

	// Set the indexes.
	if existing != nil {
		query.CreateIndex = existing.(*structs.PreparedQuery).CreateIndex
		query.ModifyIndex = idx
	} else {
		query.CreateIndex = idx
		query.ModifyIndex = idx
	}

	// Verify that the name doesn't alias any existing ID. If we didn't do
	// this then a bad actor could steal traffic away from an existing DNS
	// entry.
	if query.Name != "" {
		existing, err := tx.First("queries", "id", query.Name)

		// This is a little unfortunate but the UUID index will complain
		// if the name isn't formatted like a UUID, so we can safely
		// ignore any UUID format-related errors.
		if err != nil && !strings.Contains(err.Error(), "UUID") {
			return fmt.Errorf("failed query lookup: %s", err)
		}
		if existing != nil {
			return fmt.Errorf("name '%s' aliases an existing query id", query.Name)
		}
	}

	// Verify that the session exists.
	if query.Session != "" {
		sess, err := tx.First("sessions", "id", query.Session)
		if err != nil {
			return fmt.Errorf("failed session lookup: %s", err)
		}
		if sess == nil {
			return fmt.Errorf("invalid session %#v", query.Session)
		}
	}

	// Verify that the service exists.
	service, err := tx.First("services", "service", query.Service.Service)
	if err != nil {
		return fmt.Errorf("failed service lookup: %s", err)
	}
	if service == nil {
		return fmt.Errorf("invalid service %#v", query.Service.Service)
	}

	// Insert the query.
	if err := tx.Insert("queries", query); err != nil {
		return fmt.Errorf("failed inserting query: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"queries", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Defer(func() { s.tableWatches["queries"].Notify() })
	return nil
}

// QueryDelete deletes the given query by ID.
func (s *StateStore) QueryDelete(idx uint64, queryID string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	watches := NewDumbWatchManager(s.tableWatches)
	if err := s.queryDeleteTxn(tx, idx, watches, queryID); err != nil {
		return fmt.Errorf("failed query delete: %s", err)
	}

	tx.Defer(func() { watches.Notify() })
	tx.Commit()
	return nil
}

// queryDeleteTxn is the inner method used to delete a prepared query with the
// proper indexes into the state store.
func (s *StateStore) queryDeleteTxn(tx *memdb.Txn, idx uint64, watches *DumbWatchManager,
	queryID string) error {
	// Pull the query.
	query, err := tx.First("queries", "id", queryID)
	if err != nil {
		return fmt.Errorf("failed query lookup: %s", err)
	}
	if query == nil {
		return nil
	}

	// Delete the query and update the index.
	if err := tx.Delete("queries", query); err != nil {
		return fmt.Errorf("failed query delete: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{"queries", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	watches.Arm("queries")
	return nil
}

// QueryGet returns the given prepared query by ID.
func (s *StateStore) QueryGet(queryID string) (uint64, *structs.PreparedQuery, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("QueryGet")...)

	// Look up the query by its ID.
	query, err := tx.First("queries", "id", queryID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed query lookup: %s", err)
	}
	if query != nil {
		return idx, query.(*structs.PreparedQuery), nil
	}
	return idx, nil, nil
}

// QueryLookup returns the given prepared query by looking up an ID or Name.
func (s *StateStore) QueryLookup(queryIDOrName string) (uint64, *structs.PreparedQuery, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("QueryLookup")...)

	// Explicitly ban an empty query. This will never match an ID and the
	// schema is set up so it will never match a query with an empty name,
	// but we check it here to be explicit about it (we'd never want to
	// return the results from the first query w/o a name).
	if queryIDOrName == "" {
		return idx, nil, ErrMissingQueryID
	}

	// Try first by ID.
	query, err := tx.First("queries", "id", queryIDOrName)

	// This is a little unfortunate but the UUID index will complain
	// if the name isn't formatted like a UUID, so we can safely
	// ignore any UUID format-related errors.
	if err != nil && !strings.Contains(err.Error(), "UUID") {
		return 0, nil, fmt.Errorf("failed query lookup: %s", err)
	}
	if query != nil {
		return idx, query.(*structs.PreparedQuery), nil
	}

	// Then try by name.
	query, err = tx.First("queries", "name", queryIDOrName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed query lookup: %s", err)
	}
	if query != nil {
		return idx, query.(*structs.PreparedQuery), nil
	}

	return idx, nil, nil
}

// QueryList returns all the prepared queries.
func (s *StateStore) QueryList() (uint64, structs.PreparedQueries, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the table index.
	idx := maxIndexTxn(tx, s.getWatchTables("QueryList")...)

	// Query all of the prepared queries in the state store.
	queries, err := tx.Get("queries", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed query lookup: %s", err)
	}

	// Go over all of the queries and build the response.
	var result structs.PreparedQueries
	for query := queries.Next(); query != nil; query = queries.Next() {
		result = append(result, query.(*structs.PreparedQuery))
	}
	return idx, result, nil
}
