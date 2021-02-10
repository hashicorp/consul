package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
)

// newDBSchema creates and returns the memdb schema for the Store.
func newDBSchema() *memdb.DBSchema {
	db := &memdb.DBSchema{Tables: make(map[string]*memdb.TableSchema)}

	addTableSchemas(db,
		authMethodsTableSchema,
		autopilotConfigTableSchema,
		bindingRulesTableSchema,
		caBuiltinProviderTableSchema,
		caConfigTableSchema,
		caRootTableSchema,
		checksTableSchema,
		configTableSchema,
		coordinatesTableSchema,
		federationStateTableSchema,
		gatewayServicesTableSchema,
		indexTableSchema,
		intentionsTableSchema,
		kvsTableSchema,
		meshTopologyTableSchema,
		nodesTableSchema,
		policiesTableSchema,
		preparedQueriesTableSchema,
		rolesTableSchema,
		servicesTableSchema,
		sessionChecksTableSchema,
		sessionsTableSchema,
		systemMetadataTableSchema,
		tokensTableSchema,
		tombstonesTableSchema,
		usageTableSchema,
	)
	withEnterpriseSchema(db)
	return db
}

func addTableSchemas(db *memdb.DBSchema, schemas ...func() *memdb.TableSchema) {
	for _, fn := range schemas {
		schema := fn()
		if _, ok := db.Tables[schema.Name]; ok {
			panic(fmt.Sprintf("duplicate table name: %s", schema.Name))
		}
		db.Tables[schema.Name] = schema
	}
}

// indexTableSchema returns a new table schema used for tracking various indexes
// for the Raft log.
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "index",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
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
