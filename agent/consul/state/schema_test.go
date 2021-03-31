package state

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

type indexerTestCase struct {
	read       indexValue
	write      indexValue
	prefix     []indexValue
	writeMulti indexValueMulti
}

type indexValue struct {
	source   interface{}
	expected []byte
}

type indexValueMulti struct {
	source   interface{}
	expected [][]byte
}

func TestNewDBSchema_Indexers(t *testing.T) {
	schema := newDBSchema()
	require.NoError(t, schema.Validate())

	var testcases = map[string]func() map[string]indexerTestCase{
		tableACLPolicies:     testIndexerTableACLPolicies,
		tableACLRoles:        testIndexerTableACLRoles,
		tableChecks:          testIndexerTableChecks,
		tableServices:        testIndexerTableServices,
		tableNodes:           testIndexerTableNodes,
		tableConfigEntries:   testIndexerTableConfigEntries,
		tableMeshTopology:    testIndexerTableMeshTopology,
		tableGatewayServices: testIndexerTableGatewayServices,
	}
	addEnterpriseIndexerTestCases(testcases)

	for _, table := range schema.Tables {
		if testcases[table.Name] == nil {
			continue
		}
		t.Run(table.Name, func(t *testing.T) {
			tableTCs := testcases[table.Name]()

			for _, index := range table.Indexes {
				t.Run(index.Name, func(t *testing.T) {
					indexer := index.Indexer
					tc, ok := tableTCs[index.Name]
					if !ok {
						t.Skip("TODO: missing test case")
					}

					args := []interface{}{tc.read.source}
					if s, ok := tc.read.source.([]interface{}); ok {
						// Indexes using memdb.CompoundIndex must be expanded to multiple args
						args = s
					}

					actual, err := indexer.FromArgs(args...)
					require.NoError(t, err)
					require.Equal(t, tc.read.expected, actual)

					if i, ok := indexer.(memdb.SingleIndexer); ok {
						valid, actual, err := i.FromObject(tc.write.source)
						require.NoError(t, err)
						require.True(t, valid)
						require.Equal(t, tc.write.expected, actual)
					}

					if i, ok := indexer.(memdb.PrefixIndexer); ok {
						for _, c := range tc.prefix {
							t.Run("", func(t *testing.T) {
								actual, err := i.PrefixFromArgs(c.source)
								require.NoError(t, err)
								require.Equal(t, c.expected, actual)
							})
						}
					}

					if i, ok := indexer.(memdb.MultiIndexer); ok {
						valid, actual, err := i.FromObject(tc.writeMulti.source)
						require.NoError(t, err)
						require.True(t, valid)
						require.Equal(t, tc.writeMulti.expected, actual)
					}
				})
			}
		})
	}
}
