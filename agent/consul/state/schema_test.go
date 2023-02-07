package state

import (
	"sort"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto/pbpeering"
)

type indexerTestCase struct {
	read       indexValue
	write      indexValue
	prefix     []indexValue
	writeMulti indexValueMulti
	// extra test cases can be added if the indexer has special handling for
	// specific cases.
	extra []indexerTestCase
}

type indexValue struct {
	source   interface{}
	expected []byte
	// expectedIndexMissing indicates that this test case should not produce an
	// expected value. The indexer should report a required value was missing.
	// This field is only relevant for the writeIndex.
	expectedIndexMissing bool
}

type indexValueMulti struct {
	source   interface{}
	expected [][]byte
}

func TestNewDBSchema_Indexers(t *testing.T) {
	schema := newDBSchema()
	require.NoError(t, schema.Validate())

	var testcases = map[string]func() map[string]indexerTestCase{
		// acl
		tableACLBindingRules: testIndexerTableACLBindingRules,
		tableACLPolicies:     testIndexerTableACLPolicies,
		tableACLRoles:        testIndexerTableACLRoles,
		tableACLTokens:       testIndexerTableACLTokens,
		// catalog
		tableChecks:            testIndexerTableChecks,
		tableServices:          testIndexerTableServices,
		tableNodes:             testIndexerTableNodes,
		tableCoordinates:       testIndexerTableCoordinates,
		tableMeshTopology:      testIndexerTableMeshTopology,
		tableGatewayServices:   testIndexerTableGatewayServices,
		tableServiceVirtualIPs: testIndexerTableServiceVirtualIPs,
		tableKindServiceNames:  testIndexerTableKindServiceNames,
		// KV
		tableKVs:        testIndexerTableKVs,
		tableTombstones: testIndexerTableTombstones,
		// config
		tableConfigEntries: testIndexerTableConfigEntries,
		// peerings
		tablePeering:            testIndexerTablePeering,
		tablePeeringSecrets:     testIndexerTablePeeringSecrets,
		tablePeeringSecretUUIDs: testIndexerTablePeeringSecretUUIDs,
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
					tc.run(t, indexer)
				})
			}
		})
	}
}

func (tc indexerTestCase) run(t *testing.T, indexer memdb.Indexer) {
	args := []interface{}{tc.read.source}
	if s, ok := tc.read.source.([]interface{}); ok {
		// Indexes using memdb.CompoundIndex must be expanded to multiple args
		args = s
	}

	if tc.read.source != nil {
		t.Run("readIndex", func(t *testing.T) {
			actual, err := indexer.FromArgs(args...)
			require.NoError(t, err)
			require.Equal(t, tc.read.expected, actual)
		})
	}

	if i, ok := indexer.(memdb.SingleIndexer); ok {
		t.Run("writeIndex", func(t *testing.T) {
			valid, actual, err := i.FromObject(tc.write.source)
			require.NoError(t, err)
			if tc.write.expectedIndexMissing {
				require.False(t, valid, "expected the indexer to produce no index value")
			} else {
				require.True(t, valid, "indexer was missing a required value")
				require.Equal(t, tc.write.expected, actual)
			}
		})
	}

	if i, ok := indexer.(memdb.PrefixIndexer); ok {
		for _, c := range tc.prefix {
			t.Run("prefixIndex", func(t *testing.T) {
				actual, err := i.PrefixFromArgs(c.source)
				require.NoError(t, err)
				require.Equal(t, c.expected, actual)
			})
		}
	}

	sortMultiByteSlice := func(v [][]byte) {
		sort.Slice(v, func(i, j int) bool {
			return string(v[i]) < string(v[j])
		})
	}

	if i, ok := indexer.(memdb.MultiIndexer); ok {
		t.Run("writeIndexMulti", func(t *testing.T) {
			valid, actual, err := i.FromObject(tc.writeMulti.source)
			require.NoError(t, err)
			require.True(t, valid)
			sortMultiByteSlice(actual)
			sortMultiByteSlice(tc.writeMulti.expected)
			require.ElementsMatch(t, tc.writeMulti.expected, actual)
		})
	}

	for _, extra := range tc.extra {
		t.Run("extra", func(t *testing.T) {
			extra.run(t, indexer)
		})
	}
}

func testIndexerTablePeeringSecrets() map[string]indexerTestCase {
	peerID := "b560e87b-934c-491a-9771-16b9d9ce41f8"
	encodedPeerID := []byte{0xb5, 0x60, 0xe8, 0x7b, 0x93, 0x4c, 0x49, 0x1a, 0x97, 0x71, 0x16, 0xb9, 0xd9, 0xce, 0x41, 0xf8}

	obj := &pbpeering.PeeringSecrets{
		PeerID: peerID,
		Establishment: &pbpeering.PeeringSecrets_Establishment{
			SecretID: "432feb2f-5476-4ae2-b33c-e43640ca0e86",
		},
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   peerID,
				expected: encodedPeerID,
			},
			write: indexValue{
				source:   obj,
				expected: encodedPeerID,
			},
		},
	}
}

func testIndexerTablePeeringSecretUUIDs() map[string]indexerTestCase {
	secretID := "432feb2f-5476-4ae2-b33c-e43640ca0e86"
	encodedSecretID := []byte{0x43, 0x2f, 0xeb, 0x2f, 0x54, 0x76, 0x4a, 0xe2, 0xb3, 0x3c, 0xe4, 0x36, 0x40, 0xca, 0xe, 0x86}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   secretID,
				expected: encodedSecretID,
			},
			write: indexValue{
				source:   secretID,
				expected: encodedSecretID,
			},
		},
	}
}
