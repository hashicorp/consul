package state

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/testing/golden"
)

// TODO: once TestNewDBSchema_Indexers has test cases for all tables and indexes
// it is probably safe to remove this test
func TestNewDBSchema(t *testing.T) {
	schema := newDBSchema()
	require.NoError(t, schema.Validate())

	_, err := memdb.NewMemDB(schema)
	require.NoError(t, err)

	actual, err := repr(schema)
	require.NoError(t, err)

	expected := golden.Get(t, actual, stateStoreSchemaExpected)
	require.Equal(t, expected, actual)
}

func repr(schema *memdb.DBSchema) (string, error) {
	tables := make([]string, 0, len(schema.Tables))
	for name := range schema.Tables {
		tables = append(tables, name)
	}
	sort.Strings(tables)

	buf := new(bytes.Buffer)
	for _, name := range tables {
		fmt.Fprintf(buf, "table=%v\n", name)

		indexes := indexNames(schema.Tables[name])
		for _, i := range indexes {
			index := schema.Tables[name].Indexes[i]
			fmt.Fprintf(buf, "  index=%v", i)
			if index.Unique {
				buf.WriteString(" unique")
			}
			if index.AllowMissing {
				buf.WriteString(" allow-missing")
			}
			buf.WriteString("\n")
			buf.WriteString("    indexer=")
			formatIndexer(buf, index.Indexer)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return buf.String(), nil
}

func formatIndexer(buf *bytes.Buffer, indexer memdb.Indexer) {
	v := reflect.Indirect(reflect.ValueOf(indexer))
	typ := v.Type()
	buf.WriteString(typ.PkgPath() + "." + typ.Name())
	for i := 0; i < typ.NumField(); i++ {
		fmt.Fprintf(buf, " %v=", typ.Field(i).Name)

		formatField(buf, v.Field(i))
	}
}

func formatField(buf *bytes.Buffer, field reflect.Value) {
	switch field.Type().Kind() {
	case reflect.Slice:
		buf.WriteString("[")
		for j := 0; j < field.Len(); j++ {
			if j != 0 {
				buf.WriteString(", ")
			}
			// TODO: handle other types of slices
			formatIndexer(buf, field.Index(j).Interface().(memdb.Indexer))
		}
		buf.WriteString("]")
	case reflect.Func:
		// Functions are printed as pointer addresses, which change frequently.
		// Instead use the name.
		buf.WriteString(runtime.FuncForPC(field.Pointer()).Name())
	case reflect.Interface:
		formatField(buf, field.Elem())
	default:
		fmt.Fprintf(buf, "%v", field)
	}
}

func indexNames(table *memdb.TableSchema) []string {
	indexes := make([]string, 0, len(table.Indexes))
	for name := range table.Indexes {
		indexes = append(indexes, name)
	}

	sort.Strings(indexes)
	return indexes
}

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
		tableChecks:   testIndexerTableChecks,
		tableServices: testIndexerTableServices,
		tableNodes:    testIndexerTableNodes,
	}

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
