package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// schemaFn is an interface function used to create and return
// new memdb schema structs for constructing an in-memory db.
type schemaFn func() *memdb.TableSchema

// stateStoreSchema is used to return the combined schema for
// the state store.
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
	}

	// Collect the needed schemas
	schemas := []schemaFn{
		indexTableSchema,
		nodesTableSchema,
		servicesTableSchema,
		checksTableSchema,
		kvsTableSchema,
		tombstonesTableSchema,
		sessionsTableSchema,
		sessionChecksTableSchema,
		aclsTableSchema,
	}

	// Add the tables to the root schema
	for _, fn := range schemas {
		schema := fn()
		if _, ok := db.Tables[schema.Name]; ok {
			panic(fmt.Sprintf("duplicate table name: %s", schema.Name))
		}
		db.Tables[schema.Name] = schema
	}
	return db
}

// indexTableSchema returns a new table schema used for
// tracking various indexes for the Raft log.
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "index",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: true,
				},
			},
		},
	}
}

// nodesTableSchema returns a new table schema used for
// storing node information.
func nodesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "nodes",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
		},
	}
}

// servicesTableSchema returns a new TableSchema used to
// store information about services.
func servicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "services",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "ServiceID",
							Lowercase: true,
						},
					},
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
		},
	}
}

// checksTableSchema returns a new table schema used for
// storing and indexing health check information. Health
// checks have a number of different attributes we want to
// filter by, so this table is a bit more complex.
func checksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "checks",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "CheckID",
							Lowercase: true,
						},
					},
				},
			},
			"status": &memdb.IndexSchema{
				Name:         "status",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Status",
					Lowercase: false,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
		},
	}
}

// kvsTableSchema returns a new table schema used for storing
// key/value data from consul's kv store.
//
// TODO(ryanuber): I don't *think* we need an id_prefix, since
// StringFieldIndex implements PrefixIndexer.
func kvsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "kvs",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: false,
				},
			},
			"session": &memdb.IndexSchema{
				Name:         "session",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Session",
					Lowercase: false,
				},
			},
		},
	}
}

// tombstonesTableSchema returns a new table schema used for
// storing tombstones during kvs delete operations to prevent
// the index from sliding backwards.
func tombstonesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "tombstones",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: false,
				},
			},
		},
	}
}

// sessionsTableSchema returns a new TableSchema used for
// storing session information.
func sessionsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "sessions",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ID",
					Lowercase: false,
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
		},
	}
}

// sessionChecksTableSchema returns a new table schema used
// for storing session checks.
func sessionChecksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "session_checks",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "CheckID",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Session",
							Lowercase: false,
						},
					},
				},
			},
		},
	}
}

// aclsTableSchema returns a new table schema used for
// storing ACL information.
func aclsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acls",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ID",
					Lowercase: false,
				},
			},
		},
	}
}
