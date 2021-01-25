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

func TestStateStoreSchema(t *testing.T) {
	schema := stateStoreSchema()
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

		field := v.Field(i)
		switch typ.Field(i).Type.Kind() {
		case reflect.Slice:
			buf.WriteString("[")
			for j := 0; j < field.Len(); j++ {
				if j != 0 {
					buf.WriteString(", ")
				}
				// TODO: handle other types of slices
				formatIndexer(buf, v.Field(i).Index(j).Interface().(memdb.Indexer))
			}
			buf.WriteString("]")
		case reflect.Func:
			// Functions are printed as pointer addresses, which change frequently.
			// Instead use the name.
			buf.WriteString(runtime.FuncForPC(field.Pointer()).Name())
		default:
			fmt.Fprintf(buf, "%v", field)
		}
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
