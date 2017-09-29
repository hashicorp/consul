package analysis

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

func TestSaveDefinition(t *testing.T) {
	sp := &spec.Swagger{}
	saveSchema(sp, "theName", spec.StringProperty())
	assert.Contains(t, sp.Definitions, "theName")
}

func TestNameFromRef(t *testing.T) {
	values := []struct{ Source, Expected string }{
		{"#/definitions/errorModel", "errorModel"},
		{"http://somewhere.com/definitions/errorModel", "errorModel"},
		{"http://somewhere.com/definitions/errorModel.json", "errorModel"},
		{"/definitions/errorModel", "errorModel"},
		{"/definitions/errorModel.json", "errorModel"},
		{"http://somewhere.com", "somewhereCom"},
		{"#", ""},
	}

	for _, v := range values {
		assert.Equal(t, v.Expected, nameFromRef(spec.MustCreateRef(v.Source)))
	}
}

func TestDefinitionName(t *testing.T) {
	values := []struct {
		Source, Expected string
		Definitions      spec.Definitions
	}{
		{"#/definitions/errorModel", "errorModel", map[string]spec.Schema(nil)},
		{"http://somewhere.com/definitions/errorModel", "errorModel", map[string]spec.Schema(nil)},
		{"#/definitions/errorModel", "errorModel", map[string]spec.Schema{"apples": *spec.StringProperty()}},
		{"#/definitions/errorModel", "errorModelOAIGen", map[string]spec.Schema{"errorModel": *spec.StringProperty()}},
		{"#/definitions/errorModel", "errorModelOAIGen1", map[string]spec.Schema{"errorModel": *spec.StringProperty(), "errorModelOAIGen": *spec.StringProperty()}},
		{"#", "oaiGen", nil},
	}

	for _, v := range values {
		assert.Equal(t, v.Expected, uniqifyName(v.Definitions, nameFromRef(spec.MustCreateRef(v.Source))))
	}
}

func TestUpdateRef(t *testing.T) {
	bp := filepath.Join("fixtures", "external_definitions.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		values := []struct {
			Key string
			Ref spec.Ref
		}{
			{"#/parameters/someParam/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/parameters/1/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/responses/someResponse/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/default/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/200/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/namedAgain", spec.MustCreateRef("#/definitions/named")},
			{"#/definitions/datedTag/allOf/1", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/datedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/additionalItems", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/otherRecords/items", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/tags/additionalProperties", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/namedThing/properties/name", spec.MustCreateRef("#/definitions/named")},
		}

		for _, v := range values {
			err := updateRef(sp, v.Key, v.Ref)
			if assert.NoError(t, err) {
				ptr, err := jsonpointer.New(v.Key[1:])
				if assert.NoError(t, err) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String())
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String())
						case *spec.SchemaOrBool:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String())
						case *spec.SchemaOrArray:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String())
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}
	}
}

func TestImportExternalReferences(t *testing.T) {
	bp := filepath.Join(".", "fixtures", "external_definitions.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		values := []struct {
			Key string
			Ref spec.Ref
		}{
			{"#/parameters/someParam/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/parameters/1/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/responses/someResponse/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/default/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/200/schema", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/namedAgain", spec.MustCreateRef("#/definitions/named")},
			{"#/definitions/datedTag/allOf/1", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/datedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/additionalItems", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/otherRecords/items", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/tags/additionalProperties", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/namedThing/properties/name", spec.MustCreateRef("#/definitions/named")},
		}
		for _, v := range values {
			// technically not necessary to run for each value, but if things go right
			// this is idempotent, so having it repeat shouldn't matter
			// this validates that behavior
			err := importExternalReferences(&FlattenOpts{
				Spec:     New(sp),
				BasePath: bp,
			})

			if assert.NoError(t, err) {

				ptr, err := jsonpointer.New(v.Key[1:])
				if assert.NoError(t, err) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "for %s", v.Key)
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "for %s", v.Key)
						case *spec.SchemaOrBool:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "for %s", v.Key)
						case *spec.SchemaOrArray:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "for %s", v.Key)
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}
		assert.Len(t, sp.Definitions, 11)
		assert.Contains(t, sp.Definitions, "tag")
		assert.Contains(t, sp.Definitions, "named")
		assert.Contains(t, sp.Definitions, "record")
	}
}

func TestRewriteSchemaRef(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		values := []struct {
			Key string
			Ref spec.Ref
		}{
			{"#/parameters/someParam/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/parameters/1/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/responses/someResponse/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/default/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/paths/~1some~1where~1{id}/get/responses/200/schema", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/namedAgain", spec.MustCreateRef("#/definitions/named")},
			{"#/definitions/datedTag/allOf/1", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/datedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/items/1", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/datedTaggedRecords/additionalItems", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/otherRecords/items", spec.MustCreateRef("#/definitions/record")},
			{"#/definitions/tags/additionalProperties", spec.MustCreateRef("#/definitions/tag")},
			{"#/definitions/namedThing/properties/name", spec.MustCreateRef("#/definitions/named")},
		}

		for i, v := range values {
			err := rewriteSchemaToRef(sp, v.Key, v.Ref)
			if assert.NoError(t, err) {
				ptr, err := jsonpointer.New(v.Key[1:])
				if assert.NoError(t, err) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrBool:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrArray:
							assert.Equal(t, v.Ref.String(), tv.Schema.Ref.String(), "at %d for %s", i, v.Key)
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}
	}
}

func TestSplitKey(t *testing.T) {

	type KeyFlag uint64

	const (
		isOperation KeyFlag = 1 << iota
		isDefinition
		isSharedOperationParam
		isOperationParam
		isOperationResponse
		isDefaultResponse
		isStatusCodeResponse
	)

	values := []struct {
		Key         string
		Flags       KeyFlag
		PathItemRef spec.Ref
		PathRef     spec.Ref
		Name        string
	}{
		{
			"#/paths/~1some~1where~1{id}/parameters/1/schema",
			isOperation | isSharedOperationParam,
			spec.Ref{},
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/2/schema",
			isOperation | isOperationParam,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/default/schema",
			isOperation | isOperationResponse | isDefaultResponse,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"Default",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			isOperation | isOperationResponse | isStatusCodeResponse,
			spec.MustCreateRef("#/paths/~1some~1where~1{id}/GET"),
			spec.MustCreateRef("#/paths/~1some~1where~1{id}"),
			"OK",
		},
		{
			"#/definitions/namedAgain",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"namedAgain",
		},
		{
			"#/definitions/datedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedRecords",
		},
		{
			"#/definitions/datedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedRecords",
		},
		{
			"#/definitions/datedTaggedRecords/items/1",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedTaggedRecords",
		},
		{
			"#/definitions/datedTaggedRecords/additionalItems",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"datedTaggedRecords",
		},
		{
			"#/definitions/otherRecords/items",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"otherRecords",
		},
		{
			"#/definitions/tags/additionalProperties",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"tags",
		},
		{
			"#/definitions/namedThing/properties/name",
			isDefinition,
			spec.Ref{},
			spec.Ref{},
			"namedThing",
		},
	}

	for i, v := range values {
		parts := keyParts(v.Key)
		pref := parts.PathRef()
		piref := parts.PathItemRef()
		assert.Equal(t, v.PathRef.String(), pref.String(), "pathRef: %s at %d", v.Key, i)
		assert.Equal(t, v.PathItemRef.String(), piref.String(), "pathItemRef: %s at %d", v.Key, i)

		if v.Flags&isOperation != 0 {
			assert.True(t, parts.IsOperation(), "isOperation: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperation(), "isOperation: %s at %d", v.Key, i)
		}
		if v.Flags&isDefinition != 0 {
			assert.True(t, parts.IsDefinition(), "isDefinition: %s at %d", v.Key, i)
			assert.Equal(t, v.Name, parts.DefinitionName(), "definition name: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsDefinition(), "isDefinition: %s at %d", v.Key, i)
			if v.Name != "" {
				assert.Equal(t, v.Name, parts.ResponseName(), "response name: %s at %d", v.Key, i)
			}
		}
		if v.Flags&isOperationParam != 0 {
			assert.True(t, parts.IsOperationParam(), "isOperationParam: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperationParam(), "isOperationParam: %s at %d", v.Key, i)
		}
		if v.Flags&isSharedOperationParam != 0 {
			assert.True(t, parts.IsSharedOperationParam(), "isSharedOperationParam: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsSharedOperationParam(), "isSharedOperationParam: %s at %d", v.Key, i)
		}
		if v.Flags&isOperationResponse != 0 {
			assert.True(t, parts.IsOperationResponse(), "isOperationResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsOperationResponse(), "isOperationResponse: %s at %d", v.Key, i)
		}
		if v.Flags&isDefaultResponse != 0 {
			assert.True(t, parts.IsDefaultResponse(), "isDefaultResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsDefaultResponse(), "isDefaultResponse: %s at %d", v.Key, i)
		}
		if v.Flags&isStatusCodeResponse != 0 {
			assert.True(t, parts.IsStatusCodeResponse(), "isStatusCodeResponse: %s at %d", v.Key, i)
		} else {
			assert.False(t, parts.IsStatusCodeResponse(), "isStatusCodeResponse: %s at %d", v.Key, i)
		}
	}
}

func definitionPtr(key string) string {
	if !strings.HasPrefix(key, "#/definitions") {
		return key
	}
	return strings.Join(strings.Split(key, "/")[:3], "/")
}

func TestNamesFromKey(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	if assert.NoError(t, err) {

		values := []struct {
			Key   string
			Names []string
		}{
			{"#/paths/~1some~1where~1{id}/parameters/1/schema", []string{"GetSomeWhereID params body", "PostSomeWhereID params body"}},
			{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", []string{"GetSomeWhereID params body"}},
			{"#/paths/~1some~1where~1{id}/get/responses/default/schema", []string{"GetSomeWhereID Default body"}},
			{"#/paths/~1some~1where~1{id}/get/responses/200/schema", []string{"GetSomeWhereID OK body"}},
			{"#/definitions/namedAgain", []string{"namedAgain"}},
			{"#/definitions/datedTag/allOf/1", []string{"datedTag allOf 1"}},
			{"#/definitions/datedRecords/items/1", []string{"datedRecords tuple 1"}},
			{"#/definitions/datedTaggedRecords/items/1", []string{"datedTaggedRecords tuple 1"}},
			{"#/definitions/datedTaggedRecords/additionalItems", []string{"datedTaggedRecords tuple additionalItems"}},
			{"#/definitions/otherRecords/items", []string{"otherRecords items"}},
			{"#/definitions/tags/additionalProperties", []string{"tags additionalProperties"}},
			{"#/definitions/namedThing/properties/name", []string{"namedThing name"}},
		}

		for i, v := range values {
			ptr, err := jsonpointer.New(definitionPtr(v.Key)[1:])
			if assert.NoError(t, err) {
				vv, _, err := ptr.Get(sp)
				if assert.NoError(t, err) {
					switch tv := vv.(type) {
					case *spec.Schema:
						aschema, err := Schema(SchemaOpts{Schema: tv, Root: sp, BasePath: bp})
						if assert.NoError(t, err) {
							names := namesFromKey(keyParts(v.Key), aschema, opRefsByRef(gatherOperations(New(sp), nil)))
							assert.Equal(t, v.Names, names, "for %s at %d", v.Key, i)
						}
					case spec.Schema:
						aschema, err := Schema(SchemaOpts{Schema: &tv, Root: sp, BasePath: bp})
						if assert.NoError(t, err) {
							names := namesFromKey(keyParts(v.Key), aschema, opRefsByRef(gatherOperations(New(sp), nil)))
							assert.Equal(t, v.Names, names, "for %s at %d", v.Key, i)
						}
					default:
						assert.Fail(t, "unknown type", "got %T", vv)
					}
				}
			}
		}
	}
}

func TestDepthFirstSort(t *testing.T) {
	bp := filepath.Join("fixtures", "inline_schemas.yml")
	sp, err := loadSpec(bp)
	values := []string{
		"#/paths/~1some~1where~1{id}/parameters/1/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/parameters/1/schema",
		"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/get/parameters/2/schema",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/id",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/value",
		"#/paths/~1some~1where~1{id}/get/responses/200/schema",
		"#/paths/~1some~1where~1{id}/get/responses/404/schema",
		"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/createdAt",
		"#/paths/~1some~1where~1{id}/get/responses/default/schema",
		"#/definitions/datedRecords/items/1/properties/createdAt",
		"#/definitions/datedTaggedRecords/items/1/properties/createdAt",
		"#/definitions/namedThing/properties/name/properties/id",
		"#/definitions/records/items/0/properties/createdAt",
		"#/definitions/datedTaggedRecords/additionalItems/properties/id",
		"#/definitions/datedTaggedRecords/additionalItems/properties/value",
		"#/definitions/otherRecords/items/properties/createdAt",
		"#/definitions/tags/additionalProperties/properties/id",
		"#/definitions/tags/additionalProperties/properties/value",
		"#/definitions/datedRecords/items/0",
		"#/definitions/datedRecords/items/1",
		"#/definitions/datedTag/allOf/0",
		"#/definitions/datedTag/allOf/1",
		"#/definitions/datedTag/properties/id",
		"#/definitions/datedTag/properties/value",
		"#/definitions/datedTaggedRecords/items/0",
		"#/definitions/datedTaggedRecords/items/1",
		"#/definitions/namedAgain/properties/id",
		"#/definitions/namedThing/properties/name",
		"#/definitions/pneumonoultramicroscopicsilicovolcanoconiosisAntidisestablishmentarianism/properties/floccinaucinihilipilificationCreatedAt",
		"#/definitions/records/items/0",
		"#/definitions/datedTaggedRecords/additionalItems",
		"#/definitions/otherRecords/items",
		"#/definitions/tags/additionalProperties",
		"#/definitions/datedRecords",
		"#/definitions/datedTag",
		"#/definitions/datedTaggedRecords",
		"#/definitions/namedAgain",
		"#/definitions/namedThing",
		"#/definitions/otherRecords",
		"#/definitions/pneumonoultramicroscopicsilicovolcanoconiosisAntidisestablishmentarianism",
		"#/definitions/records",
		"#/definitions/tags",
	}
	if assert.NoError(t, err) {
		a := New(sp)
		result := sortDepthFirst(a.allSchemas)
		assert.Equal(t, values, result)
	}
}

func TestNameInlinedSchemas(t *testing.T) {
	bp := filepath.Join(".", "fixtures", "nested_inline_schemas.yml")
	sp, err := loadSpec(bp)
	values := []struct {
		Key      string
		Location string
		Ref      spec.Ref
	}{
		{"#/paths/~1some~1where~1{id}/parameters/1/schema/items", "#/definitions/postSomeWhereIdParamsBody/items", spec.MustCreateRef("#/definitions/postSomeWhereIdParamsBodyItems")},
		{"#/paths/~1some~1where~1{id}/parameters/1/schema", "#/paths/~1some~1where~1{id}/parameters/1/schema", spec.MustCreateRef("#/definitions/postSomeWhereIdParamsBody")},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/2/properties/name", "#/definitions/getSomeWhereIdParamsBodyRecordItems2/properties/name", spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2Name")},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/1", "#/definitions/getSomeWhereIdParamsBodyRecord/items/1", spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems1")},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record/items/2", "#/definitions/getSomeWhereIdParamsBodyRecord/items/2", spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2")},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema/properties/record", "#/definitions/getSomeWhereIdParamsBodyOAIGen/properties/record", spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecord")},
		{"#/paths/~1some~1where~1{id}/get/parameters/2/schema", "#/paths/~1some~1where~1{id}/get/parameters/2/schema", spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyOAIGen")},
		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/2/properties/name", "#/definitions/getSomeWhereIdOKBodyRecordItems2/properties/name", spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems2Name")},
		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/1", "#/definitions/getSomeWhereIdOKBodyRecord/items/1", spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems1")},
		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record/items/2", "#/definitions/getSomeWhereIdOKBodyRecord/items/2", spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecordItems2")},
		{"#/paths/~1some~1where~1{id}/get/responses/200/schema/properties/record", "#/definitions/getSomeWhereIdOKBody/properties/record", spec.MustCreateRef("#/definitions/getSomeWhereIdOKBodyRecord")},
		{"#/paths/~1some~1where~1{id}/get/responses/200/schema", "#/paths/~1some~1where~1{id}/get/responses/200/schema", spec.MustCreateRef("#/definitions/getSomeWhereIdOKBody")},
		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/2/properties/name", "#/definitions/getSomeWhereIdDefaultBodyRecordItems2/properties/name", spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems2Name")},
		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/1", "#/definitions/getSomeWhereIdDefaultBodyRecord/items/1", spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems1")},
		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record/items/2", "#/definitions/getSomeWhereIdDefaultBodyRecord/items/2", spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecordItems2")},
		{"#/paths/~1some~1where~1{id}/get/responses/default/schema/properties/record", "#/definitions/getSomeWhereIdDefaultBody/properties/record", spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBodyRecord")},
		{"#/paths/~1some~1where~1{id}/get/responses/default/schema", "#/paths/~1some~1where~1{id}/get/responses/default/schema", spec.MustCreateRef("#/definitions/getSomeWhereIdDefaultBody")},
		{"#/definitions/nestedThing/properties/record/items/2/allOf/1/additionalProperties", "#/definitions/nestedThingRecordItems2AllOf1/additionalProperties", spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1AdditionalProperties")},
		{"#/definitions/nestedThing/properties/record/items/2/allOf/1", "#/definitions/nestedThingRecordItems2/allOf/1", spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1")},
		{"#/definitions/nestedThing/properties/record/items/2/properties/name", "#/definitions/nestedThingRecordItems2/properties/name", spec.MustCreateRef("#/definitions/nestedThingRecordItems2Name")},
		{"#/definitions/nestedThing/properties/record/items/1", "#/definitions/nestedThingRecord/items/1", spec.MustCreateRef("#/definitions/nestedThingRecordItems1")},
		{"#/definitions/nestedThing/properties/record/items/2", "#/definitions/nestedThingRecord/items/2", spec.MustCreateRef("#/definitions/nestedThingRecordItems2")},
		{"#/definitions/datedRecords/items/1", "#/definitions/datedRecords/items/1", spec.MustCreateRef("#/definitions/datedRecordsItems1")},
		{"#/definitions/datedTaggedRecords/items/1", "#/definitions/datedTaggedRecords/items/1", spec.MustCreateRef("#/definitions/datedTaggedRecordsItems1")},
		{"#/definitions/namedThing/properties/name", "#/definitions/namedThing/properties/name", spec.MustCreateRef("#/definitions/namedThingName")},
		{"#/definitions/nestedThing/properties/record", "#/definitions/nestedThing/properties/record", spec.MustCreateRef("#/definitions/nestedThingRecord")},
		{"#/definitions/records/items/0", "#/definitions/records/items/0", spec.MustCreateRef("#/definitions/recordsItems0")},
		{"#/definitions/datedTaggedRecords/additionalItems", "#/definitions/datedTaggedRecords/additionalItems", spec.MustCreateRef("#/definitions/datedTaggedRecordsItemsAdditionalItems")},
		{"#/definitions/otherRecords/items", "#/definitions/otherRecords/items", spec.MustCreateRef("#/definitions/otherRecordsItems")},
		{"#/definitions/tags/additionalProperties", "#/definitions/tags/additionalProperties", spec.MustCreateRef("#/definitions/tagsAdditionalProperties")},
	}
	if assert.NoError(t, err) {
		err := nameInlinedSchemas(&FlattenOpts{
			Spec:     New(sp),
			BasePath: bp,
		})

		if assert.NoError(t, err) {
			for i, v := range values {
				ptr, err := jsonpointer.New(v.Location[1:])
				if assert.NoError(t, err, "at %d for %s", i, v.Key) {
					vv, _, err := ptr.Get(sp)

					if assert.NoError(t, err, "at %d for %s", i, v.Key) {
						switch tv := vv.(type) {
						case *spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case spec.Schema:
							assert.Equal(t, v.Ref.String(), tv.Ref.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrBool:
							var sRef spec.Ref
							if tv != nil && tv.Schema != nil {
								sRef = tv.Schema.Ref
							}
							assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
						case *spec.SchemaOrArray:
							var sRef spec.Ref
							if tv != nil && tv.Schema != nil {
								sRef = tv.Schema.Ref
							}
							assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
						default:
							assert.Fail(t, "unknown type", "got %T", vv)
						}
					}
				}
			}
		}

		for k, rr := range New(sp).allSchemas {
			if !strings.HasPrefix(k, "#/responses") && !strings.HasPrefix(k, "#/parameters") {
				if rr.Schema != nil && rr.Schema.Ref.String() == "" && !rr.TopLevel {
					asch, err := Schema(SchemaOpts{Schema: rr.Schema, Root: sp, BasePath: bp})
					if assert.NoError(t, err, "for key: %s", k) {
						if !asch.IsSimpleSchema {
							assert.Fail(t, "not a top level schema", "for key: %s", k)
						}
					}
				}
			}
		}
	}
}

func TestFlatten(t *testing.T) {
	bp := filepath.Join(".", "fixtures", "flatten.yml")
	sp, err := loadSpec(bp)
	values := []struct {
		Key      string
		Location string
		Ref      spec.Ref
		Expected interface{}
	}{
		{
			"#/responses/notFound/schema",
			"#/responses/notFound/schema",
			spec.MustCreateRef("#/definitions/error"),
			nil,
		},
		{
			"#/paths/~1some~1where~1{id}/parameters/0",
			"#/paths/~1some~1where~1{id}/parameters/0/name",
			spec.Ref{},
			"id",
		},
		{
			"#/paths/~1other~1place",
			"#/paths/~1other~1place/get/operationId",
			spec.Ref{},
			"modelOp",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/0",
			"#/paths/~1some~1where~1{id}/get/parameters/0/name",
			spec.Ref{},
			"limit",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/1",
			"#/paths/~1some~1where~1{id}/get/parameters/1/name",
			spec.Ref{},
			"some",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/2",
			"#/paths/~1some~1where~1{id}/get/parameters/2/name",
			spec.Ref{},
			"other",
		},
		{
			"#/paths/~1some~1where~1{id}/get/parameters/3",
			"#/paths/~1some~1where~1{id}/get/parameters/3/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBody"),
			"",
		},
		{
			"#/paths/~1some~1where~1{id}/get/responses/200",
			"#/paths/~1some~1where~1{id}/get/responses/200/schema",
			spec.MustCreateRef("#/definitions/getSomeWhereIdOKBody"),
			"",
		},
		{
			"#/definitions/namedAgain",
			"",
			spec.MustCreateRef("#/definitions/named"),
			"",
		},
		{
			"#/definitions/namedThing/properties/name",
			"",
			spec.MustCreateRef("#/definitions/named"),
			"",
		},
		{
			"#/definitions/namedThing/properties/namedAgain",
			"",
			spec.MustCreateRef("#/definitions/namedAgain"),
			"",
		},
		{
			"#/definitions/datedRecords/items/1",
			"",
			spec.MustCreateRef("#/definitions/record"),
			"",
		},
		{
			"#/definitions/otherRecords/items",
			"",
			spec.MustCreateRef("#/definitions/record"),
			"",
		},
		{
			"#/definitions/tags/additionalProperties",
			"",
			spec.MustCreateRef("#/definitions/tag"),
			"",
		},
		{
			"#/definitions/datedTag/allOf/1",
			"",
			spec.MustCreateRef("#/definitions/tag"),
			"",
		},
		{
			"#/definitions/nestedThingRecordItems2/allOf/1",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2AllOf1"),
			"",
		},
		{
			"#/definitions/nestedThingRecord/items/1",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems1"),
			"",
		},
		{
			"#/definitions/nestedThingRecord/items/2",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecordItems2"),
			"",
		},
		{
			"#/definitions/nestedThing/properties/record",
			"",
			spec.MustCreateRef("#/definitions/nestedThingRecord"),
			"",
		},
		{
			"#/definitions/named",
			"#/definitions/named/type",
			spec.Ref{},
			spec.StringOrArray{"string"},
		},
		{
			"#/definitions/error",
			"#/definitions/error/properties/id/type",
			spec.Ref{},
			spec.StringOrArray{"integer"},
		},
		{
			"#/definitions/record",
			"#/definitions/record/properties/createdAt/format",
			spec.Ref{},
			"date-time",
		},
		{
			"#/definitions/getSomeWhereIdOKBody",
			"#/definitions/getSomeWhereIdOKBody/properties/record",
			spec.MustCreateRef("#/definitions/nestedThing"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBody",
			"#/definitions/getSomeWhereIdParamsBody/properties/record",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecord"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecord",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/1",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems1"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecord",
			"#/definitions/getSomeWhereIdParamsBodyRecord/items/2",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2"),
			nil,
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2/allOf/0/format",
			spec.Ref{},
			"date",
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2Name",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2Name/properties/createdAt/format",
			spec.Ref{},
			"date-time",
		},
		{
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2",
			"#/definitions/getSomeWhereIdParamsBodyRecordItems2/properties/name",
			spec.MustCreateRef("#/definitions/getSomeWhereIdParamsBodyRecordItems2Name"),
			"date",
		},
	}
	if assert.NoError(t, err) {
		err := Flatten(FlattenOpts{Spec: New(sp), BasePath: bp})
		if assert.NoError(t, err) {
			for i, v := range values {
				pk := v.Key[1:]
				if v.Location != "" {
					pk = v.Location[1:]
				}
				ptr, err := jsonpointer.New(pk)
				if assert.NoError(t, err, "at %d for %s", i, v.Key) {
					d, _, err := ptr.Get(sp)
					if assert.NoError(t, err) {
						if v.Ref.String() != "" {
							switch s := d.(type) {
							case *spec.Schema:
								assert.Equal(t, v.Ref.String(), s.Ref.String(), "at %d for %s", i, v.Key)
							case spec.Schema:
								assert.Equal(t, v.Ref.String(), s.Ref.String(), "at %d for %s", i, v.Key)
							case *spec.SchemaOrArray:
								var sRef spec.Ref
								if s != nil && s.Schema != nil {
									sRef = s.Schema.Ref
								}
								assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
							case *spec.SchemaOrBool:
								var sRef spec.Ref
								if s != nil && s.Schema != nil {
									sRef = s.Schema.Ref
								}
								assert.Equal(t, v.Ref.String(), sRef.String(), "at %d for %s", i, v.Key)
							default:
								assert.Fail(t, "unknown type", "got %T at %d for %s", d, i, v.Key)
							}
						} else {
							assert.Equal(t, v.Expected, d)
						}
					}
				}
			}
		}
	}
}
