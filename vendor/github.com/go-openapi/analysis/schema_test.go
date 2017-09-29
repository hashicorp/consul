package analysis

import (
	"encoding/json"
	"fmt"
	"path"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

var knownSchemas = []*spec.Schema{
	spec.BoolProperty(),                  // 0
	spec.StringProperty(),                // 1
	spec.Int8Property(),                  // 2
	spec.Int16Property(),                 // 3
	spec.Int32Property(),                 // 4
	spec.Int64Property(),                 // 5
	spec.Float32Property(),               // 6
	spec.Float64Property(),               // 7
	spec.DateProperty(),                  // 8
	spec.DateTimeProperty(),              // 9
	(&spec.Schema{}),                     // 10
	(&spec.Schema{}).Typed("object", ""), // 11
	(&spec.Schema{}).Typed("", ""),       // 12
	(&spec.Schema{}).Typed("", "uuid"),   // 13
}

func newCObj() *spec.Schema {
	return (&spec.Schema{}).Typed("object", "").SetProperty("id", *spec.Int64Property())
}

var complexObject = newCObj()

var complexSchemas = []*spec.Schema{
	complexObject,
	spec.ArrayProperty(complexObject),
	spec.MapProperty(complexObject),
}

func knownRefs(base string) []spec.Ref {
	urls := []string{"bool", "string", "integer", "float", "date", "object", "format"}

	var result []spec.Ref
	for _, u := range urls {
		result = append(result, spec.MustCreateRef(fmt.Sprintf("%s/%s", base, path.Join("known", u))))
	}
	return result
}

func complexRefs(base string) []spec.Ref {
	urls := []string{"object", "array", "map"}

	var result []spec.Ref
	for _, u := range urls {
		result = append(result, spec.MustCreateRef(fmt.Sprintf("%s/%s", base, path.Join("complex", u))))
	}
	return result
}

func refServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/known/bool", schemaHandler(knownSchemas[0]))
	mux.Handle("/known/string", schemaHandler(knownSchemas[1]))
	mux.Handle("/known/integer", schemaHandler(knownSchemas[5]))
	mux.Handle("/known/float", schemaHandler(knownSchemas[6]))
	mux.Handle("/known/date", schemaHandler(knownSchemas[8]))
	mux.Handle("/known/object", schemaHandler(knownSchemas[11]))
	mux.Handle("/known/format", schemaHandler(knownSchemas[13]))

	mux.Handle("/complex/object", schemaHandler(complexSchemas[0]))
	mux.Handle("/complex/array", schemaHandler(complexSchemas[1]))
	mux.Handle("/complex/map", schemaHandler(complexSchemas[2]))

	return httptest.NewServer(mux)
}

func refSchema(ref spec.Ref) *spec.Schema {
	return &spec.Schema{SchemaProps: spec.SchemaProps{Ref: ref}}
}

func schemaHandler(schema *spec.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, schema)
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	if err := enc.Encode(data); err != nil {
		panic(err)
	}
}

func TestSchemaAnalysis_KnownTypes(t *testing.T) {
	for i, v := range knownSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsKnownType, "item at %d should be a known type", i)
		}
	}
	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.False(t, sch.IsKnownType, "item at %d should not be a known type", i)
		}
	}

	serv := refServer()
	defer serv.Close()

	for i, ref := range knownRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: refSchema(ref)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsKnownType, "item at %d should be a known type", i)
		}
	}
	for i, ref := range complexRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: refSchema(ref)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.False(t, sch.IsKnownType, "item at %d should not be a known type", i)
		}
	}
}

func TestSchemaAnalysis_Array(t *testing.T) {
	for i, v := range append(knownSchemas, (&spec.Schema{}).Typed("array", "")) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsArray, "item at %d should be an array type", i)
			assert.True(t, sch.IsSimpleArray, "item at %d should be a simple array type", i)
		}
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsArray, "item at %d should be an array type", i)
			assert.False(t, sch.IsSimpleArray, "item at %d should not be a simple array type", i)
		}
	}

	serv := refServer()
	defer serv.Close()

	for i, ref := range knownRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(refSchema(ref))})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsArray, "item at %d should be an array type", i)
			assert.True(t, sch.IsSimpleArray, "item at %d should be a simple array type", i)
		}
	}
	for i, ref := range complexRefs(serv.URL) {
		sch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(refSchema(ref))})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.False(t, sch.IsKnownType, "item at %d should not be a known type", i)
			assert.True(t, sch.IsArray, "item at %d should be an array type", i)
			assert.False(t, sch.IsSimpleArray, "item at %d should not be a simple array type", i)
		}
	}

}

func TestSchemaAnalysis_Map(t *testing.T) {
	for i, v := range append(knownSchemas, spec.MapProperty(nil)) {
		sch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsMap, "item at %d should be a map type", i)
			assert.True(t, sch.IsSimpleMap, "item at %d should be a simple map type", i)
		}
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsMap, "item at %d should be a map type", i)
			assert.False(t, sch.IsSimpleMap, "item at %d should not be a simple map type", i)
		}
	}
}

func TestSchemaAnalysis_ExtendedObject(t *testing.T) {
	for i, v := range knownSchemas {
		wex := spec.MapProperty(v).SetProperty("name", *spec.StringProperty())
		sch, err := Schema(SchemaOpts{Schema: wex})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsExtendedObject, "item at %d should be an extended map object type", i)
			assert.False(t, sch.IsMap, "item at %d should not be a map type", i)
			assert.False(t, sch.IsSimpleMap, "item at %d should not be a simple map type", i)
		}
	}
}

func TestSchemaAnalysis_Tuple(t *testing.T) {
	at := spec.ArrayProperty(nil)
	at.Items = &spec.SchemaOrArray{}
	at.Items.Schemas = append(at.Items.Schemas, *spec.StringProperty(), *spec.Int64Property())

	sch, err := Schema(SchemaOpts{Schema: at})
	if assert.NoError(t, err) {
		assert.True(t, sch.IsTuple)
		assert.False(t, sch.IsTupleWithExtra)
		assert.False(t, sch.IsKnownType)
		assert.False(t, sch.IsSimpleSchema)
	}
}

func TestSchemaAnalysis_TupleWithExtra(t *testing.T) {
	at := spec.ArrayProperty(nil)
	at.Items = &spec.SchemaOrArray{}
	at.Items.Schemas = append(at.Items.Schemas, *spec.StringProperty(), *spec.Int64Property())
	at.AdditionalItems = &spec.SchemaOrBool{Allows: true}
	at.AdditionalItems.Schema = spec.Int32Property()

	sch, err := Schema(SchemaOpts{Schema: at})
	if assert.NoError(t, err) {
		assert.False(t, sch.IsTuple)
		assert.True(t, sch.IsTupleWithExtra)
		assert.False(t, sch.IsKnownType)
		assert.False(t, sch.IsSimpleSchema)
	}
}

func TestSchemaAnalysis_BaseType(t *testing.T) {
	cl := (&spec.Schema{}).Typed("object", "").SetProperty("type", *spec.StringProperty()).WithDiscriminator("type")

	sch, err := Schema(SchemaOpts{Schema: cl})
	if assert.NoError(t, err) {
		assert.True(t, sch.IsBaseType)
		assert.False(t, sch.IsKnownType)
		assert.False(t, sch.IsSimpleSchema)
	}
}

func TestSchemaAnalysis_SimpleSchema(t *testing.T) {
	for i, v := range append(knownSchemas, spec.ArrayProperty(nil), spec.MapProperty(nil)) {
		sch, err := Schema(SchemaOpts{Schema: v})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.True(t, sch.IsSimpleSchema, "item at %d should be a simple schema", i)
		}

		asch, err := Schema(SchemaOpts{Schema: spec.ArrayProperty(v)})
		if assert.NoError(t, err, "failed to analyze array schema at %d: %v", i, err) {
			assert.True(t, asch.IsSimpleSchema, "array item at %d should be a simple schema", i)
		}

		msch, err := Schema(SchemaOpts{Schema: spec.MapProperty(v)})
		if assert.NoError(t, err, "failed to analyze map schema at %d: %v", i, err) {
			assert.True(t, msch.IsSimpleSchema, "map item at %d should be a simple schema", i)
		}
	}

	for i, v := range complexSchemas {
		sch, err := Schema(SchemaOpts{Schema: v})
		if assert.NoError(t, err, "failed to analyze schema at %d: %v", i, err) {
			assert.False(t, sch.IsSimpleSchema, "item at %d should not be a simple schema", i)
		}
	}

}
