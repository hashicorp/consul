package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// schemaFn is an interface function used to create and return
// new memdb schema structs for constructing an in-memory db.
type schemaFn func() *memdb.TableSchema

// schemas is used to register schemas with the state store.
var schemas []schemaFn

// registerSchema registers a new schema with the state store. This should
// get called at package init() time.
func registerSchema(fn schemaFn) {
	schemas = append(schemas, fn)
}

// stateStoreSchema is used to return the combined schema for
// the state store.
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
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

// indexTableSchema returns a new table schema used for tracking various indexes
// for the Raft log.
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

func init() {
	registerSchema(indexTableSchema)
}
