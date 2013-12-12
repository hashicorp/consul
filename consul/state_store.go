package consul

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type namedQuery uint8

const (
	queryEnsureNode namedQuery = iota
	queryNode
	queryNodes
	queryEnsureService
	queryNodeServices
	queryDeleteNodeService
	queryDeleteNode
)

// NoodeServices maps the Service name to a tag and port
type ServiceEntry struct {
	Tag  string
	Port int
}
type NodeServices map[string]ServiceEntry

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
		"pragma foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := s.db.Exec(p); err != nil {
			return fmt.Errorf("Failed to set '%s': %v", p, err)
		}
	}

	// Create the tables
	tables := []string{
		`CREATE TABLE nodes (name text unique, address text);`,
		`CREATE TABLE services (node text REFERENCES nodes(name) ON DELETE CASCADE, service text, tag text, port integer);`,
		`CREATE INDEX servName ON services(service);`,
		`CREATE INDEX nodeName ON services(node);`,
	}
	for _, t := range tables {
		if _, err := s.db.Exec(t); err != nil {
			return fmt.Errorf("Failed to call '%s': %v", t, err)
		}
	}

	// Prepare the queries
	queries := map[namedQuery]string{
		queryEnsureNode:        "INSERT OR REPLACE INTO nodes (name, address) VALUES (?, ?)",
		queryNode:              "SELECT address FROM nodes where name=?",
		queryNodes:             "SELECT * FROM nodes",
		queryEnsureService:     "INSERT OR REPLACE INTO services (node, service, tag, port) VALUES (?, ?, ?, ?)",
		queryNodeServices:      "SELECT service, tag, port from services where node=?",
		queryDeleteNodeService: "DELETE FROM services WHERE node=? AND service=?",
		queryDeleteNode:        "DELETE FROM nodes WHERE name=?",
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

func (s *StateStore) checkSet(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("Failed to set row")
	}
	return nil
}

func (s *StateStore) checkDelete(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	_, err = res.RowsAffected()
	if err != nil {
		return err
	}
	return nil
}

// EnsureNode is used to ensure a given node exists, with the provided address
func (s *StateStore) EnsureNode(name string, address string) error {
	stmt := s.prepared[queryEnsureNode]
	return s.checkSet(stmt.Exec(name, address))
}

// GetNode returns all the address of the known and if it was found
func (s *StateStore) GetNode(name string) (bool, string) {
	stmt := s.prepared[queryNode]
	row := stmt.QueryRow(name)

	var addr string
	if err := row.Scan(&addr); err != nil {
		if err == sql.ErrNoRows {
			return false, addr
		} else {
			panic(fmt.Errorf("Failed to get node: %v", err))
		}
	}
	return true, addr
}

// GetNodes returns all the known nodes, the slice alternates between
// the node name and address
func (s *StateStore) Nodes() []string {
	stmt := s.prepared[queryNodes]
	rows, err := stmt.Query()
	if err != nil {
		panic(fmt.Errorf("Failed to get nodes: %v", err))
	}

	data := make([]string, 0, 32)
	var name, address string
	for rows.Next() {
		if err := rows.Scan(&name, &address); err != nil {
			panic(fmt.Errorf("Failed to get nodes: %v", err))
		}
		data = append(data, name, address)
	}
	return data
}

// EnsureService is used to ensure a given node exposes a service
func (s *StateStore) EnsureService(name, service, tag string, port int) error {
	stmt := s.prepared[queryEnsureService]
	return s.checkSet(stmt.Exec(name, service, tag, port))
}

// NodeServices is used to return all the services of a given node
func (s *StateStore) NodeServices(name string) NodeServices {
	stmt := s.prepared[queryNodeServices]
	rows, err := stmt.Query(name)
	if err != nil {
		panic(fmt.Errorf("Failed to get node services: %v", err))
	}

	services := NodeServices(make(map[string]ServiceEntry))
	var service string
	var entry ServiceEntry
	for rows.Next() {
		if err := rows.Scan(&service, &entry.Tag, &entry.Port); err != nil {
			panic(fmt.Errorf("Failed to get node services: %v", err))
		}
		services[service] = entry
	}

	return services
}

// DeleteNodeService is used to delete a node service
func (s *StateStore) DeleteNodeService(node, service string) error {
	stmt := s.prepared[queryDeleteNodeService]
	return s.checkDelete(stmt.Exec(node, service))
}

// DeleteNode is used to delete a node and all it's services
func (s *StateStore) DeleteNode(node string) error {
	stmt := s.prepared[queryDeleteNode]
	return s.checkDelete(stmt.Exec(node))
}
