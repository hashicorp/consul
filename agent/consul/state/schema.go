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
		peeringSecretsTableSchema,
		peeringSecretUUIDsTableSchema,
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
				Indexer: indexerSingle[string, *IndexEntry]{
					readIndex:  indexFromString,
					writeIndex: indexNameFromIndexEntry,
				},
			},
		},
	}
}

func indexNameFromIndexEntry(e *IndexEntry) ([]byte, error) {
	if e.Key == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(e.Key))
	return b.Bytes(), nil
}

func indexFromString(s string) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(s))
	return b.Bytes(), nil
}

func indexDeletedFromBoolQuery(q BoolQuery) ([]byte, error) {
	var b indexBuilder
	b.Bool(q.Value)
	return b.Bytes(), nil
}

type enterpriseIndexable interface {
	partitionIndexable
	namespaceIndexable
}

type partitionIndexable interface {
	PartitionOrDefault() string
}

type namespaceIndexable interface {
	NamespaceOrDefault() string
}
