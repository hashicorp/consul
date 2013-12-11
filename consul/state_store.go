package consul

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type namedQuery uint8

const (
	queryInit namedQuery = iota
)

// The StateStore is responsible for maintaining all the Consul
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. The current
// implementation uses an in-memory SQLite database. This reduced the
// GC pressure on Go, and also gives us Multi-Version Concurrency Control
// for "free".
type StateStore struct {
	db       *sql.DB
	prepared map[namedQuery]*sql.Stmt
}

// NewStateStore is used to create a new state store
func NewStateStore() (*StateStore, error) {
	// Open the db
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %v", err)
	}

	s := &StateStore{
		db:       db,
		prepared: make(map[namedQuery]*sql.Stmt),
	}

	// Ensure we can initialize
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close is used to safely shutdown the state store
func (s *StateStore) Close() error {
	return s.db.Close()
}

// initialize is used to setup the sqlite store for use
func (s *StateStore) initialize() error {
	// Set the pragma first
	pragmas := []string{
		"pragma journal_mode=memory;",
	}
	for _, p := range pragmas {
		if _, err := s.db.Exec(p); err != nil {
			return fmt.Errorf("Failed to set '%s': %v", p, err)
		}
	}

	// Create the tables
	tables := []string{
		`CREATE TABLE foo (idx integer primary key);`,
	}
	for _, t := range tables {
		if _, err := s.db.Exec(t); err != nil {
			return fmt.Errorf("Failed to call '%s': %v", t, err)
		}
	}

	// Prepare the queries
	queries := map[namedQuery]string{
		queryInit: "SELECT 1",
	}
	for name, query := range queries {
		stmt, err := s.db.Prepare(query)
		if err != nil {
			return fmt.Errorf("Failed to prepare '%s': %v", query, err)
		}
		s.prepared[name] = stmt
	}
	return nil
}
