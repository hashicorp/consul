package state

import (
	"fmt"
	"strings"

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
		freeVirtualIPTableSchema,
		gatewayServicesTableSchema,
		indexTableSchema,
		intentionsTableSchema,
		kindServiceNameTableSchema,
		kvsTableSchema,
		meshTopologyTableSchema,
		nodesTableSchema,
		peeringTableSchema,
		peeringTrustBundlesTableSchema,
		policiesTableSchema,
		preparedQueriesTableSchema,
		rolesTableSchema,
		servicesTableSchema,
		serviceVirtualIPTableSchema,
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

// IndexEntry keeps a record of the last index of a table or entity within a table.
type IndexEntry struct {
	Key   string
	Value uint64
}

const (
	tableIndex   = "index"
	indexDeleted = "deleted"
)

// indexTableSchema returns a new table schema used for tracking various the
// latest raft index for a table or entities within a table.
//
// The index table is necessary for tables that do not use tombstones. If the latest
// items in the table are deleted, the max index of a table would appear to go
// backwards. With the index table we can keep track of the latest update to a
// table, even when that update is a delete of the most recent item.
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableIndex,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle{
					readIndex:  indexFromString,
					writeIndex: indexNameFromIndexEntry,
				},
			},
		},
	}
}

func indexNameFromIndexEntry(raw interface{}) ([]byte, error) {
	p, ok := raw.(*IndexEntry)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for IndexEntry index", raw)
	}

	if p.Key == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.Key))
	return b.Bytes(), nil
}

func indexFromString(raw interface{}) ([]byte, error) {
	q, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for string prefix query", raw)
	}

	var b indexBuilder
	b.String(strings.ToLower(q))
	return b.Bytes(), nil
}

func indexDeletedFromBoolQuery(raw interface{}) ([]byte, error) {
	q, ok := raw.(BoolQuery)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for BoolQuery index", raw)
	}

	var b indexBuilder
	b.Bool(q.Value)
	return b.Bytes(), nil
}
