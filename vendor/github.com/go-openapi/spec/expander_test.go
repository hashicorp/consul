// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/swag"
	"github.com/stretchr/testify/assert"
)

func jsonDoc(path string) (json.RawMessage, error) {
	data, err := swag.LoadFromFileOrHTTP(path)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// tests that paths are normalized correctly
func TestNormalizePaths(t *testing.T) {
	type testNormalizePathsTestCases []struct {
		refPath   string
		base      string
		expOutput string
	}
	testCases := func() testNormalizePathsTestCases {
		testCases := testNormalizePathsTestCases{
			{
				// http basePath, absolute refPath
				refPath:   "http://www.anotherexample.com/another/base/path/swagger.json#/definitions/Pet",
				base:      "http://www.example.com/base/path/swagger.json",
				expOutput: "http://www.anotherexample.com/another/base/path/swagger.json#/definitions/Pet",
			},
			{
				// http basePath, relative refPath
				refPath:   "another/base/path/swagger.json#/definitions/Pet",
				base:      "http://www.example.com/base/path/swagger.json",
				expOutput: "http://www.example.com/base/path/another/base/path/swagger.json#/definitions/Pet",
			},
		}
		if runtime.GOOS == "windows" {
			testCases = append(testCases, testNormalizePathsTestCases{
				{
					// file basePath, absolute refPath, no fragment
					refPath:   `C:\another\base\path.json`,
					base:      `C:\base\path.json`,
					expOutput: `C:\another\base\path.json`,
				},
				{
					// file basePath, absolute refPath
					refPath:   `C:\another\base\path.json#/definitions/Pet`,
					base:      `C:\base\path.json`,
					expOutput: `C:\another\base\path.json#/definitions/Pet`,
				},
				{
					// file basePath, relative refPath
					refPath:   `another\base\path.json#/definitions/Pet`,
					base:      `C:\base\path.json`,
					expOutput: `C:\base\another\base\path.json#/definitions/Pet`,
				},
			}...)
			return testCases
		}
		// linux case
		testCases = append(testCases, testNormalizePathsTestCases{
			{
				// file basePath, absolute refPath, no fragment
				refPath:   "/another/base/path.json",
				base:      "/base/path.json",
				expOutput: "/another/base/path.json",
			},
			{
				// file basePath, absolute refPath
				refPath:   "/another/base/path.json#/definitions/Pet",
				base:      "/base/path.json",
				expOutput: "/another/base/path.json#/definitions/Pet",
			},
			{
				// file basePath, relative refPath
				refPath:   "another/base/path.json#/definitions/Pet",
				base:      "/base/path.json",
				expOutput: "/base/another/base/path.json#/definitions/Pet",
			},
		}...)
		return testCases
	}()

	for _, tcase := range testCases {
		out := normalizePaths(tcase.refPath, tcase.base)
		assert.Equal(t, tcase.expOutput, out)
	}
}

func TestExpandsKnownRef(t *testing.T) {
	schema := RefProperty("http://json-schema.org/draft-04/schema#")
	if assert.NoError(t, ExpandSchema(schema, nil, nil)) {
		assert.Equal(t, "Core schema meta-schema", schema.Description)
	}
}

func TestExpandResponseSchema(t *testing.T) {
	fp := "./fixtures/local_expansion/spec.json"
	b, err := jsonDoc(fp)
	if assert.NoError(t, err) {
		var spec Swagger
		if err := json.Unmarshal(b, &spec); assert.NoError(t, err) {
			err := ExpandSpec(&spec, &ExpandOptions{RelativeBase: fp})
			if assert.NoError(t, err) {
				sch := spec.Paths.Paths["/item"].Get.Responses.StatusCodeResponses[200].Schema
				if assert.NotNil(t, sch) {
					assert.Empty(t, sch.Ref.String())
					assert.Contains(t, sch.Type, "object")
					assert.Len(t, sch.Properties, 2)
				}
			}
		}
	}
}

func TestSpecExpansion(t *testing.T) {
	spec := new(Swagger)
	// resolver, err := defaultSchemaLoader(spec, nil, nil)
	// assert.NoError(t, err)

	err := ExpandSpec(spec, nil)
	assert.NoError(t, err)

	specDoc, err := jsonDoc("fixtures/expansion/all-the-things.json")
	assert.NoError(t, err)

	specPath, _ := absPath("fixtures/expansion/all-the-things.json")
	opts := &ExpandOptions{
		RelativeBase: specPath,
	}

	spec = new(Swagger)
	err = json.Unmarshal(specDoc, spec)
	assert.NoError(t, err)

	pet := spec.Definitions["pet"]
	errorModel := spec.Definitions["errorModel"]
	petResponse := spec.Responses["petResponse"]
	petResponse.Schema = &pet
	stringResponse := spec.Responses["stringResponse"]
	tagParam := spec.Parameters["tag"]
	idParam := spec.Parameters["idParam"]

	err = ExpandSpec(spec, opts)
	assert.NoError(t, err)

	assert.Equal(t, tagParam, spec.Parameters["query"])
	assert.Equal(t, petResponse, spec.Responses["petResponse"])
	assert.Equal(t, petResponse, spec.Responses["anotherPet"])
	assert.Equal(t, pet, *spec.Responses["petResponse"].Schema)
	assert.Equal(t, stringResponse, *spec.Paths.Paths["/"].Get.Responses.Default)
	assert.Equal(t, petResponse, spec.Paths.Paths["/"].Get.Responses.StatusCodeResponses[200])
	assert.Equal(t, pet, *spec.Paths.Paths["/pets"].Get.Responses.StatusCodeResponses[200].Schema.Items.Schema)
	assert.Equal(t, errorModel, *spec.Paths.Paths["/pets"].Get.Responses.Default.Schema)
	assert.Equal(t, pet, spec.Definitions["petInput"].AllOf[0])
	assert.Equal(t, spec.Definitions["petInput"], *spec.Paths.Paths["/pets"].Post.Parameters[0].Schema)
	assert.Equal(t, petResponse, spec.Paths.Paths["/pets"].Post.Responses.StatusCodeResponses[200])
	assert.Equal(t, errorModel, *spec.Paths.Paths["/pets"].Post.Responses.Default.Schema)
	pi := spec.Paths.Paths["/pets/{id}"]
	assert.Equal(t, idParam, pi.Get.Parameters[0])
	assert.Equal(t, petResponse, pi.Get.Responses.StatusCodeResponses[200])
	assert.Equal(t, errorModel, *pi.Get.Responses.Default.Schema)
	assert.Equal(t, idParam, pi.Delete.Parameters[0])
	assert.Equal(t, errorModel, *pi.Delete.Responses.Default.Schema)
}

func TestResolveRef(t *testing.T) {
	var root interface{}
	err := json.Unmarshal([]byte(PetStore20), &root)
	assert.NoError(t, err)
	ref, err := NewRef("#/definitions/Category")
	assert.NoError(t, err)
	sch, err := ResolveRef(root, &ref)
	assert.NoError(t, err)
	b, _ := sch.MarshalJSON()
	assert.JSONEq(t, `{"id":"Category","properties":{"id":{"type":"integer","format":"int64"},"name":{"type":"string"}}}`, string(b))
}

func TestResponseExpansion(t *testing.T) {
	specDoc, err := jsonDoc("fixtures/expansion/all-the-things.json")
	assert.NoError(t, err)

	basePath, err := absPath("fixtures/expansion/all-the-things.json")
	assert.NoError(t, err)

	spec := new(Swagger)
	err = json.Unmarshal(specDoc, spec)
	assert.NoError(t, err)

	resolver, err := defaultSchemaLoader(spec, nil, nil)
	assert.NoError(t, err)

	resp := spec.Responses["anotherPet"]
	r := spec.Responses["petResponse"]
	err = expandResponse(&r, resolver, basePath)
	assert.NoError(t, err)
	expected := r

	err = expandResponse(&resp, resolver, basePath)
	// b, _ := resp.MarshalJSON()
	// log.Printf(string(b))
	// b, _ = expected.MarshalJSON()
	// log.Printf(string(b))
	assert.NoError(t, err)
	assert.Equal(t, expected, resp)

	resp2 := spec.Paths.Paths["/"].Get.Responses.Default
	expected = spec.Responses["stringResponse"]

	err = expandResponse(resp2, resolver, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, *resp2)

	resp = spec.Paths.Paths["/"].Get.Responses.StatusCodeResponses[200]
	expected = spec.Responses["petResponse"]

	err = expandResponse(&resp, resolver, basePath)
	assert.NoError(t, err)
	// assert.Equal(t, expected, resp)
}

// test the exported version of ExpandResponse
func TestExportedResponseExpansion(t *testing.T) {
	specDoc, err := jsonDoc("fixtures/expansion/all-the-things.json")
	assert.NoError(t, err)

	basePath, err := absPath("fixtures/expansion/all-the-things.json")
	assert.NoError(t, err)

	spec := new(Swagger)
	err = json.Unmarshal(specDoc, spec)
	assert.NoError(t, err)

	resp := spec.Responses["anotherPet"]
	r := spec.Responses["petResponse"]
	err = ExpandResponse(&r, basePath)
	assert.NoError(t, err)
	expected := r

	err = ExpandResponse(&resp, basePath)
	// b, _ := resp.MarshalJSON()
	// log.Printf(string(b))
	// b, _ = expected.MarshalJSON()
	// log.Printf(string(b))
	assert.NoError(t, err)
	assert.Equal(t, expected, resp)

	resp2 := spec.Paths.Paths["/"].Get.Responses.Default
	expected = spec.Responses["stringResponse"]

	err = ExpandResponse(resp2, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, *resp2)

	resp = spec.Paths.Paths["/"].Get.Responses.StatusCodeResponses[200]
	expected = spec.Responses["petResponse"]

	err = ExpandResponse(&resp, basePath)
	assert.NoError(t, err)
	// assert.Equal(t, expected, resp)
}

func TestIssue3(t *testing.T) {
	spec := new(Swagger)
	specDoc, err := jsonDoc("fixtures/expansion/overflow.json")
	assert.NoError(t, err)

	specPath, _ := absPath("fixtures/expansion/overflow.json")
	opts := &ExpandOptions{
		RelativeBase: specPath,
	}

	err = json.Unmarshal(specDoc, spec)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		err = ExpandSpec(spec, opts)
		assert.NoError(t, err)
	}, "Calling expand spec with circular refs, should not panic!")
}

func TestParameterExpansion(t *testing.T) {
	paramDoc, err := jsonDoc("fixtures/expansion/params.json")
	assert.NoError(t, err)

	spec := new(Swagger)
	err = json.Unmarshal(paramDoc, spec)
	assert.NoError(t, err)

	basePath, err := absPath("fixtures/expansion/params.json")
	assert.NoError(t, err)

	resolver, err := defaultSchemaLoader(spec, nil, nil)
	assert.NoError(t, err)

	param := spec.Parameters["query"]
	expected := spec.Parameters["tag"]

	err = expandParameter(&param, resolver, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, param)

	param = spec.Paths.Paths["/cars/{id}"].Parameters[0]
	expected = spec.Parameters["id"]

	err = expandParameter(&param, resolver, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, param)
}

func TestExportedParameterExpansion(t *testing.T) {
	paramDoc, err := jsonDoc("fixtures/expansion/params.json")
	assert.NoError(t, err)

	spec := new(Swagger)
	err = json.Unmarshal(paramDoc, spec)
	assert.NoError(t, err)

	basePath, err := absPath("fixtures/expansion/params.json")
	assert.NoError(t, err)

	param := spec.Parameters["query"]
	expected := spec.Parameters["tag"]

	err = ExpandParameter(&param, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, param)

	param = spec.Paths.Paths["/cars/{id}"].Parameters[0]
	expected = spec.Parameters["id"]

	err = ExpandParameter(&param, basePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, param)
}

func TestCircularRefsExpansion(t *testing.T) {
	carsDoc, err := jsonDoc("fixtures/expansion/circularRefs.json")
	assert.NoError(t, err)

	basePath, _ := absPath("fixtures/expansion/circularRefs.json")

	spec := new(Swagger)
	err = json.Unmarshal(carsDoc, spec)
	assert.NoError(t, err)

	resolver, err := defaultSchemaLoader(spec, nil, nil)
	assert.NoError(t, err)
	schema := spec.Definitions["car"]

	assert.NotPanics(t, func() {
		_, err = expandSchema(schema, []string{"#/definitions/car"}, resolver, basePath)
		assert.NoError(t, err)
	}, "Calling expand schema with circular refs, should not panic!")
}

func TestContinueOnErrorExpansion(t *testing.T) {
	missingRefDoc, err := jsonDoc("fixtures/expansion/missingRef.json")
	assert.NoError(t, err)

	specPath, _ := absPath("fixtures/expansion/missingRef.json")

	testCase := struct {
		Input    *Swagger `json:"input"`
		Expected *Swagger `json:"expected"`
	}{}
	err = json.Unmarshal(missingRefDoc, &testCase)
	assert.NoError(t, err)

	opts := &ExpandOptions{
		ContinueOnError: true,
		RelativeBase:    specPath,
	}
	err = ExpandSpec(testCase.Input, opts)
	assert.NoError(t, err)
	// b, _ := testCase.Input.MarshalJSON()
	// log.Printf(string(b))
	assert.Equal(t, testCase.Input, testCase.Expected, "Should continue expanding spec when a definition can't be found.")

	doc, err := jsonDoc("fixtures/expansion/missingItemRef.json")
	spec := new(Swagger)
	err = json.Unmarshal(doc, spec)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		err = ExpandSpec(spec, opts)
		assert.NoError(t, err)
	}, "Array of missing refs should not cause a panic, and continue to expand spec.")
}

func TestIssue415(t *testing.T) {
	doc, err := jsonDoc("fixtures/expansion/clickmeter.json")
	assert.NoError(t, err)

	specPath, _ := absPath("fixtures/expansion/clickmeter.json")

	opts := &ExpandOptions{
		RelativeBase: specPath,
	}

	spec := new(Swagger)
	err = json.Unmarshal(doc, spec)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		err = ExpandSpec(spec, opts)
		assert.NoError(t, err)
	}, "Calling expand spec with response schemas that have circular refs, should not panic!")
}

func TestCircularSpecExpansion(t *testing.T) {
	doc, err := jsonDoc("fixtures/expansion/circularSpec.json")
	assert.NoError(t, err)

	specPath, _ := absPath("fixtures/expansion/circularSpec.json")

	opts := &ExpandOptions{
		RelativeBase: specPath,
	}

	spec := new(Swagger)
	err = json.Unmarshal(doc, spec)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		err = ExpandSpec(spec, opts)
		assert.NoError(t, err)
	}, "Calling expand spec with circular refs, should not panic!")
}

func TestItemsExpansion(t *testing.T) {
	carsDoc, err := jsonDoc("fixtures/expansion/schemas2.json")
	assert.NoError(t, err)

	basePath, _ := absPath("fixtures/expansion/schemas2.json")

	spec := new(Swagger)
	err = json.Unmarshal(carsDoc, spec)
	assert.NoError(t, err)

	resolver, err := defaultSchemaLoader(spec, nil, nil)
	assert.NoError(t, err)

	schema := spec.Definitions["car"]
	oldBrand := schema.Properties["brand"]
	assert.NotEmpty(t, oldBrand.Items.Schema.Ref.String())
	assert.NotEqual(t, spec.Definitions["brand"], oldBrand)

	_, err = expandSchema(schema, []string{"#/definitions/car"}, resolver, basePath)
	assert.NoError(t, err)

	newBrand := schema.Properties["brand"]
	assert.Empty(t, newBrand.Items.Schema.Ref.String())
	assert.Equal(t, spec.Definitions["brand"], *newBrand.Items.Schema)

	schema = spec.Definitions["truck"]
	assert.NotEmpty(t, schema.Items.Schema.Ref.String())

	s, err := expandSchema(schema, []string{"#/definitions/truck"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Items.Schema.Ref.String())
	assert.Equal(t, spec.Definitions["car"], *schema.Items.Schema)

	sch := new(Schema)
	_, err = expandSchema(*sch, []string{""}, resolver, basePath)
	assert.NoError(t, err)

	schema = spec.Definitions["batch"]
	s, err = expandSchema(schema, []string{"#/definitions/batch"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Items.Schema.Items.Schema.Ref.String())
	assert.Equal(t, *schema.Items.Schema.Items.Schema, spec.Definitions["brand"])

	schema = spec.Definitions["batch2"]
	s, err = expandSchema(schema, []string{"#/definitions/batch2"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Items.Schemas[0].Items.Schema.Ref.String())
	assert.Empty(t, schema.Items.Schemas[1].Items.Schema.Ref.String())
	assert.Equal(t, *schema.Items.Schemas[0].Items.Schema, spec.Definitions["brand"])
	assert.Equal(t, *schema.Items.Schemas[1].Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["allofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/allofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AllOf[0].Items.Schema.Ref.String())
	assert.Empty(t, schema.AllOf[1].Items.Schema.Ref.String())
	assert.Equal(t, *schema.AllOf[0].Items.Schema, spec.Definitions["brand"])
	assert.Equal(t, *schema.AllOf[1].Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["anyofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/anyofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AnyOf[0].Items.Schema.Ref.String())
	assert.Empty(t, schema.AnyOf[1].Items.Schema.Ref.String())
	assert.Equal(t, *schema.AnyOf[0].Items.Schema, spec.Definitions["brand"])
	assert.Equal(t, *schema.AnyOf[1].Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["oneofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/oneofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.OneOf[0].Items.Schema.Ref.String())
	assert.Empty(t, schema.OneOf[1].Items.Schema.Ref.String())
	assert.Equal(t, *schema.OneOf[0].Items.Schema, spec.Definitions["brand"])
	assert.Equal(t, *schema.OneOf[1].Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["notSomething"]
	s, err = expandSchema(schema, []string{"#/definitions/notSomething"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Not.Items.Schema.Ref.String())
	assert.Equal(t, *schema.Not.Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["withAdditional"]
	s, err = expandSchema(schema, []string{"#/definitions/withAdditional"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AdditionalProperties.Schema.Items.Schema.Ref.String())
	assert.Equal(t, *schema.AdditionalProperties.Schema.Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["withAdditionalItems"]
	s, err = expandSchema(schema, []string{"#/definitions/withAdditionalItems"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AdditionalItems.Schema.Items.Schema.Ref.String())
	assert.Equal(t, *schema.AdditionalItems.Schema.Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["withPattern"]
	s, err = expandSchema(schema, []string{"#/definitions/withPattern"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop := schema.PatternProperties["^x-ab"]
	assert.Empty(t, prop.Items.Schema.Ref.String())
	assert.Equal(t, *prop.Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["deps"]
	s, err = expandSchema(schema, []string{"#/definitions/deps"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop2 := schema.Dependencies["something"]
	assert.Empty(t, prop2.Schema.Items.Schema.Ref.String())
	assert.Equal(t, *prop2.Schema.Items.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["defined"]
	s, err = expandSchema(schema, []string{"#/definitions/defined"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop = schema.Definitions["something"]
	assert.Empty(t, prop.Items.Schema.Ref.String())
	assert.Equal(t, *prop.Items.Schema, spec.Definitions["tag"])
}

func TestSchemaExpansion(t *testing.T) {
	carsDoc, err := jsonDoc("fixtures/expansion/schemas1.json")
	assert.NoError(t, err)

	basePath, _ := absPath("fixtures/expansion/schemas1.json")

	spec := new(Swagger)
	err = json.Unmarshal(carsDoc, spec)
	assert.NoError(t, err)

	resolver, err := defaultSchemaLoader(spec, nil, nil)
	assert.NoError(t, err)

	schema := spec.Definitions["car"]
	oldBrand := schema.Properties["brand"]
	assert.NotEmpty(t, oldBrand.Ref.String())
	assert.NotEqual(t, spec.Definitions["brand"], oldBrand)

	s, err := expandSchema(schema, []string{"#/definitions/car"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)

	newBrand := schema.Properties["brand"]
	assert.Empty(t, newBrand.Ref.String())
	assert.Equal(t, spec.Definitions["brand"], newBrand)

	schema = spec.Definitions["truck"]
	assert.NotEmpty(t, schema.Ref.String())

	s, err = expandSchema(schema, []string{"#/definitions/truck"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Ref.String())
	assert.Equal(t, spec.Definitions["car"], schema)

	sch := new(Schema)
	_, err = expandSchema(*sch, []string{""}, resolver, basePath)
	assert.NoError(t, err)

	schema = spec.Definitions["batch"]
	s, err = expandSchema(schema, []string{"#/definitions/batch"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Items.Schema.Ref.String())
	assert.Equal(t, *schema.Items.Schema, spec.Definitions["brand"])

	schema = spec.Definitions["batch2"]
	s, err = expandSchema(schema, []string{"#/definitions/batch2"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Items.Schemas[0].Ref.String())
	assert.Empty(t, schema.Items.Schemas[1].Ref.String())
	assert.Equal(t, schema.Items.Schemas[0], spec.Definitions["brand"])
	assert.Equal(t, schema.Items.Schemas[1], spec.Definitions["tag"])

	schema = spec.Definitions["allofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/allofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AllOf[0].Ref.String())
	assert.Empty(t, schema.AllOf[1].Ref.String())
	assert.Equal(t, schema.AllOf[0], spec.Definitions["brand"])
	assert.Equal(t, schema.AllOf[1], spec.Definitions["tag"])

	schema = spec.Definitions["anyofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/anyofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AnyOf[0].Ref.String())
	assert.Empty(t, schema.AnyOf[1].Ref.String())
	assert.Equal(t, schema.AnyOf[0], spec.Definitions["brand"])
	assert.Equal(t, schema.AnyOf[1], spec.Definitions["tag"])

	schema = spec.Definitions["oneofBoth"]
	s, err = expandSchema(schema, []string{"#/definitions/oneofBoth"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.OneOf[0].Ref.String())
	assert.Empty(t, schema.OneOf[1].Ref.String())
	assert.Equal(t, schema.OneOf[0], spec.Definitions["brand"])
	assert.Equal(t, schema.OneOf[1], spec.Definitions["tag"])

	schema = spec.Definitions["notSomething"]
	s, err = expandSchema(schema, []string{"#/definitions/notSomething"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.Not.Ref.String())
	assert.Equal(t, *schema.Not, spec.Definitions["tag"])

	schema = spec.Definitions["withAdditional"]
	s, err = expandSchema(schema, []string{"#/definitions/withAdditional"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AdditionalProperties.Schema.Ref.String())
	assert.Equal(t, *schema.AdditionalProperties.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["withAdditionalItems"]
	s, err = expandSchema(schema, []string{"#/definitions/withAdditionalItems"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	assert.Empty(t, schema.AdditionalItems.Schema.Ref.String())
	assert.Equal(t, *schema.AdditionalItems.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["withPattern"]
	s, err = expandSchema(schema, []string{"#/definitions/withPattern"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop := schema.PatternProperties["^x-ab"]
	assert.Empty(t, prop.Ref.String())
	assert.Equal(t, prop, spec.Definitions["tag"])

	schema = spec.Definitions["deps"]
	s, err = expandSchema(schema, []string{"#/definitions/deps"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop2 := schema.Dependencies["something"]
	assert.Empty(t, prop2.Schema.Ref.String())
	assert.Equal(t, *prop2.Schema, spec.Definitions["tag"])

	schema = spec.Definitions["defined"]
	s, err = expandSchema(schema, []string{"#/definitions/defined"}, resolver, basePath)
	schema = *s
	assert.NoError(t, err)
	prop = schema.Definitions["something"]
	assert.Empty(t, prop.Ref.String())
	assert.Equal(t, prop, spec.Definitions["tag"])

}

func TestDefaultResolutionCache(t *testing.T) {

	cache := initResolutionCache()

	sch, ok := cache.Get("not there")
	assert.False(t, ok)
	assert.Nil(t, sch)

	sch, ok = cache.Get("http://swagger.io/v2/schema.json")
	assert.True(t, ok)
	assert.Equal(t, swaggerSchema, sch)

	sch, ok = cache.Get("http://json-schema.org/draft-04/schema")
	assert.True(t, ok)
	assert.Equal(t, jsonSchema, sch)

	cache.Set("something", "here")
	sch, ok = cache.Get("something")
	assert.True(t, ok)
	assert.Equal(t, "here", sch)
}

func TestRelativeBaseURI(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.Dir("fixtures/remote")))
	defer server.Close()

	spec := new(Swagger)
	// resolver, err := defaultSchemaLoader(spec, nil, nil)
	// assert.NoError(t, err)

	err := ExpandSpec(spec, nil)
	assert.NoError(t, err)

	specDoc, err := jsonDoc("fixtures/remote/all-the-things.json")
	assert.NoError(t, err)

	opts := &ExpandOptions{
		RelativeBase: server.URL + "/all-the-things.json",
	}

	spec = new(Swagger)
	err = json.Unmarshal(specDoc, spec)
	assert.NoError(t, err)

	pet := spec.Definitions["pet"]
	errorModel := spec.Definitions["errorModel"]
	petResponse := spec.Responses["petResponse"]
	petResponse.Schema = &pet
	stringResponse := spec.Responses["stringResponse"]
	tagParam := spec.Parameters["tag"]
	idParam := spec.Parameters["idParam"]

	anotherPet := spec.Responses["anotherPet"]
	anotherPet.Ref = MustCreateRef(server.URL + "/" + anotherPet.Ref.String())
	err = ExpandResponse(&anotherPet, opts.RelativeBase)
	assert.NoError(t, err)
	spec.Responses["anotherPet"] = anotherPet

	circularA := spec.Responses["circularA"]
	circularA.Ref = MustCreateRef(server.URL + "/" + circularA.Ref.String())
	err = ExpandResponse(&circularA, opts.RelativeBase)
	assert.NoError(t, err)
	spec.Responses["circularA"] = circularA

	err = ExpandSpec(spec, opts)
	assert.NoError(t, err)

	assert.Equal(t, tagParam, spec.Parameters["query"])
	assert.Equal(t, petResponse, spec.Responses["petResponse"])
	assert.Equal(t, petResponse, spec.Responses["anotherPet"])
	assert.Equal(t, pet, *spec.Responses["petResponse"].Schema)
	assert.Equal(t, stringResponse, *spec.Paths.Paths["/"].Get.Responses.Default)
	assert.Equal(t, petResponse, spec.Paths.Paths["/"].Get.Responses.StatusCodeResponses[200])
	assert.Equal(t, pet, *spec.Paths.Paths["/pets"].Get.Responses.StatusCodeResponses[200].Schema.Items.Schema)
	assert.Equal(t, errorModel, *spec.Paths.Paths["/pets"].Get.Responses.Default.Schema)
	assert.Equal(t, pet, spec.Definitions["petInput"].AllOf[0])
	assert.Equal(t, spec.Definitions["petInput"], *spec.Paths.Paths["/pets"].Post.Parameters[0].Schema)
	assert.Equal(t, petResponse, spec.Paths.Paths["/pets"].Post.Responses.StatusCodeResponses[200])
	assert.Equal(t, errorModel, *spec.Paths.Paths["/pets"].Post.Responses.Default.Schema)
	pi := spec.Paths.Paths["/pets/{id}"]
	assert.Equal(t, idParam, pi.Get.Parameters[0])
	assert.Equal(t, petResponse, pi.Get.Responses.StatusCodeResponses[200])
	assert.Equal(t, errorModel, *pi.Get.Responses.Default.Schema)
	assert.Equal(t, idParam, pi.Delete.Parameters[0])
	assert.Equal(t, errorModel, *pi.Delete.Responses.Default.Schema)
}

func resolutionContextServer() *httptest.Server {
	var servedAt string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// fmt.Println("got a request for", req.URL.String())
		if req.URL.Path == "/resolution.json" {

			b, _ := ioutil.ReadFile("fixtures/specs/resolution.json")
			var ctnt map[string]interface{}
			json.Unmarshal(b, &ctnt)
			ctnt["id"] = servedAt

			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			bb, _ := json.Marshal(ctnt)
			rw.Write(bb)
			return
		}
		if req.URL.Path == "/resolution2.json" {
			b, _ := ioutil.ReadFile("fixtures/specs/resolution2.json")
			var ctnt map[string]interface{}
			json.Unmarshal(b, &ctnt)
			ctnt["id"] = servedAt

			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			bb, _ := json.Marshal(ctnt)
			rw.Write(bb)
			return
		}

		if req.URL.Path == "/boolProp.json" {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			b, _ := json.Marshal(map[string]interface{}{
				"type": "boolean",
			})
			_, _ = rw.Write(b)
			return
		}

		if req.URL.Path == "/deeper/stringProp.json" {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			b, _ := json.Marshal(map[string]interface{}{
				"type": "string",
			})
			rw.Write(b)
			return
		}

		if req.URL.Path == "/deeper/arrayProp.json" {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			b, _ := json.Marshal(map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "file",
				},
			})
			rw.Write(b)
			return
		}

		rw.WriteHeader(http.StatusNotFound)
	}))
	servedAt = server.URL
	return server
}

func TestResolveRemoteRef_RootSame(t *testing.T) {
	specs := "fixtures/specs/"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	// the filename doesn't matter because ref will eventually point to refed.json
	specBase, _ := absPath("fixtures/specs/anyotherfile.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var result_0 Swagger
		ref_0, _ := NewRef(server.URL + "/refed.json#")
		resolver_0, _ := defaultSchemaLoader(rootDoc, nil, nil)
		if assert.NoError(t, resolver_0.Resolve(&ref_0, &result_0, "")) {
			assertSpecs(t, result_0, *rootDoc)
		}

		var result_1 Swagger
		ref_1, _ := NewRef("./refed.json")
		resolver_1, _ := defaultSchemaLoader(rootDoc, &ExpandOptions{
			RelativeBase: specBase,
		}, nil)
		if assert.NoError(t, resolver_1.Resolve(&ref_1, &result_1, specBase)) {
			assertSpecs(t, result_1, *rootDoc)
		}
	}
}

func TestResolveRemoteRef_FromFragment(t *testing.T) {
	specs := "fixtures/specs"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")

	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Schema
		ref, err := NewRef(server.URL + "/refed.json#/definitions/pet")
		if assert.NoError(t, err) {
			resolver := &schemaLoader{root: rootDoc, cache: initResolutionCache(), loadDoc: jsonDoc}
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, "")) {
				assert.Equal(t, []string{"id", "name"}, tgt.Required)
			}
		}
	}
}

func TestResolveRemoteRef_FromInvalidFragment(t *testing.T) {
	specs := "fixtures/specs"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Schema
		ref, err := NewRef(server.URL + "/refed.json#/definitions/NotThere")
		if assert.NoError(t, err) {
			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			assert.Error(t, resolver.Resolve(&ref, &tgt, ""))
		}
	}
}

func TestResolveRemoteRef_WithResolutionContext(t *testing.T) {
	server := resolutionContextServer()
	defer server.Close()

	var tgt Schema
	ref, err := NewRef(server.URL + "/resolution.json#/definitions/bool")
	if assert.NoError(t, err) {
		tgt.Ref = ref
		ExpandSchema(&tgt, nil, nil)
		assert.Equal(t, StringOrArray([]string{"boolean"}), tgt.Type)
	}

}

func TestResolveRemoteRef_WithNestedResolutionContext(t *testing.T) {
	server := resolutionContextServer()
	defer server.Close()

	var tgt Schema
	ref, err := NewRef(server.URL + "/resolution.json#/items")
	if assert.NoError(t, err) {
		tgt.Ref = ref
		ExpandSchema(&tgt, nil, nil)
		assert.Equal(t, StringOrArray([]string{"string"}), tgt.Items.Schema.Type)
	}
}

/* This next test will have to wait until we do full $ID analysis for every subschema on every file that is referenced */
/* For now, TestResolveRemoteRef_WithNestedResolutionContext replaces this next test */
// func TestResolveRemoteRef_WithNestedResolutionContext_WithParentID(t *testing.T) {
// 	server := resolutionContextServer()
// 	defer server.Close()

// 	var tgt Schema
// 	ref, err := NewRef(server.URL + "/resolution.json#/items/items")
// 	if assert.NoError(t, err) {
// 		tgt.Ref = ref
// 		ExpandSchema(&tgt, nil, nil)
// 		assert.Equal(t, StringOrArray([]string{"string"}), tgt.Type)
// 	}
// }

func TestResolveRemoteRef_WithNestedResolutionContextWithFragment(t *testing.T) {
	server := resolutionContextServer()
	defer server.Close()

	var tgt Schema
	ref, err := NewRef(server.URL + "/resolution2.json#/items")
	if assert.NoError(t, err) {
		tgt.Ref = ref
		ExpandSchema(&tgt, nil, nil)
		assert.Equal(t, StringOrArray([]string{"file"}), tgt.Items.Schema.Type)
	}

}

/* This next test will have to wait until we do full $ID analysis for every subschema on every file that is referenced */
/* For now, TestResolveRemoteRef_WithNestedResolutionContext replaces this next test */
// func TestResolveRemoteRef_WithNestedResolutionContextWithFragment_WithParentID(t *testing.T) {
// 	server := resolutionContextServer()
// 	defer server.Close()

// 	rootDoc := new(Swagger)
// 	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
// 	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
// 		var tgt Schema
// 		ref, err := NewRef(server.URL + "/resolution2.json#/items/items")
// 		if assert.NoError(t, err) {
// 			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
// 			if assert.NoError(t, resolver.Resolve(&ref, &tgt, "")) {
// 				assert.Equal(t, StringOrArray([]string{"file"}), tgt.Type)
// 			}
// 		}
// 	}
// }

func TestResolveRemoteRef_ToParameter(t *testing.T) {
	specs := "fixtures/specs"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Parameter
		ref, err := NewRef(server.URL + "/refed.json#/parameters/idParam")
		if assert.NoError(t, err) {

			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, "")) {
				assert.Equal(t, "id", tgt.Name)
				assert.Equal(t, "path", tgt.In)
				assert.Equal(t, "ID of pet to fetch", tgt.Description)
				assert.True(t, tgt.Required)
				assert.Equal(t, "integer", tgt.Type)
				assert.Equal(t, "int64", tgt.Format)
			}
		}
	}
}

func TestResolveRemoteRef_ToPathItem(t *testing.T) {
	specs := "fixtures/specs"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt PathItem
		ref, err := NewRef(server.URL + "/refed.json#/paths/" + jsonpointer.Escape("/pets/{id}"))
		if assert.NoError(t, err) {

			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, "")) {
				assert.Equal(t, rootDoc.Paths.Paths["/pets/{id}"].Get, tgt.Get)
			}
		}
	}
}

func TestResolveRemoteRef_ToResponse(t *testing.T) {
	specs := "fixtures/specs"
	fileserver := http.FileServer(http.Dir(specs))
	server := httptest.NewServer(fileserver)
	defer server.Close()

	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Response
		ref, err := NewRef(server.URL + "/refed.json#/responses/petResponse")
		if assert.NoError(t, err) {

			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, "")) {
				assert.Equal(t, rootDoc.Responses["petResponse"], tgt)
			}
		}
	}
}

func TestResolveLocalRef_SameRoot(t *testing.T) {
	rootDoc := new(Swagger)
	json.Unmarshal(PetStoreJSONMessage, rootDoc)

	result := new(Swagger)
	ref, _ := NewRef("#")
	resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
	err := resolver.Resolve(&ref, result, "")
	if assert.NoError(t, err) {
		assert.Equal(t, rootDoc, result)
	}
}

func TestResolveLocalRef_FromFragment(t *testing.T) {
	rootDoc := new(Swagger)
	json.Unmarshal(PetStoreJSONMessage, rootDoc)

	var tgt Schema
	ref, err := NewRef("#/definitions/Category")
	if assert.NoError(t, err) {
		resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
		err := resolver.Resolve(&ref, &tgt, "")
		if assert.NoError(t, err) {
			assert.Equal(t, "Category", tgt.ID)
		}
	}
}

func TestResolveLocalRef_FromInvalidFragment(t *testing.T) {
	rootDoc := new(Swagger)
	json.Unmarshal(PetStoreJSONMessage, rootDoc)

	var tgt Schema
	ref, err := NewRef("#/definitions/NotThere")
	if assert.NoError(t, err) {
		resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
		err := resolver.Resolve(&ref, &tgt, "")
		assert.Error(t, err)
	}
}

func TestResolveLocalRef_Parameter(t *testing.T) {
	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	basePath, _ := absPath("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Parameter
		ref, err := NewRef("#/parameters/idParam")
		if assert.NoError(t, err) {
			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, basePath)) {
				assert.Equal(t, "id", tgt.Name)
				assert.Equal(t, "path", tgt.In)
				assert.Equal(t, "ID of pet to fetch", tgt.Description)
				assert.True(t, tgt.Required)
				assert.Equal(t, "integer", tgt.Type)
				assert.Equal(t, "int64", tgt.Format)
			}
		}
	}
}

func TestResolveLocalRef_PathItem(t *testing.T) {
	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	basePath, _ := absPath("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt PathItem
		ref, err := NewRef("#/paths/" + jsonpointer.Escape("/pets/{id}"))
		if assert.NoError(t, err) {
			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, basePath)) {
				assert.Equal(t, rootDoc.Paths.Paths["/pets/{id}"].Get, tgt.Get)
			}
		}
	}
}

func TestResolveLocalRef_Response(t *testing.T) {
	rootDoc := new(Swagger)
	b, err := ioutil.ReadFile("fixtures/specs/refed.json")
	basePath, _ := absPath("fixtures/specs/refed.json")
	if assert.NoError(t, err) && assert.NoError(t, json.Unmarshal(b, rootDoc)) {
		var tgt Response
		ref, err := NewRef("#/responses/petResponse")
		if assert.NoError(t, err) {
			resolver, _ := defaultSchemaLoader(rootDoc, nil, nil)
			if assert.NoError(t, resolver.Resolve(&ref, &tgt, basePath)) {
				assert.Equal(t, rootDoc.Responses["petResponse"], tgt)
			}
		}
	}
}

func TestResolveForTransitiveRefs(t *testing.T) {
	var spec *Swagger
	rawSpec, err := ioutil.ReadFile("fixtures/specs/todos.json")
	assert.NoError(t, err)

	basePath, err := absPath("fixtures/specs/todos.json")
	assert.NoError(t, err)

	opts := &ExpandOptions{
		RelativeBase: basePath,
	}

	err = json.Unmarshal(rawSpec, &spec)
	assert.NoError(t, err)

	err = ExpandSpec(spec, opts)
	assert.NoError(t, err)
}

// PetStoreJSONMessage json raw message for Petstore20
var PetStoreJSONMessage = json.RawMessage([]byte(PetStore20))

// PetStore20 json doc for swagger 2.0 pet store
const PetStore20 = `{
  "swagger": "2.0",
  "info": {
    "version": "1.0.0",
    "title": "Swagger Petstore",
    "contact": {
      "name": "Wordnik API Team",
      "url": "http://developer.wordnik.com"
    },
    "license": {
      "name": "Creative Commons 4.0 International",
      "url": "http://creativecommons.org/licenses/by/4.0/"
    }
  },
  "host": "petstore.swagger.wordnik.com",
  "basePath": "/api",
  "schemes": [
    "http"
  ],
  "paths": {
    "/pets": {
      "get": {
        "security": [
          {
            "basic": []
          }
        ],
        "tags": [ "Pet Operations" ],
        "operationId": "getAllPets",
        "parameters": [
          {
            "name": "status",
            "in": "query",
            "description": "The status to filter by",
            "type": "string"
          },
          {
            "name": "limit",
            "in": "query",
            "description": "The maximum number of results to return",
            "type": "integer",
						"format": "int64"
          }
        ],
        "summary": "Finds all pets in the system",
        "responses": {
          "200": {
            "description": "Pet response",
            "schema": {
              "type": "array",
              "items": {
                "$ref": "#/definitions/Pet"
              }
            }
          },
          "default": {
            "description": "Unexpected error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      },
      "post": {
        "security": [
          {
            "basic": []
          }
        ],
        "tags": [ "Pet Operations" ],
        "operationId": "createPet",
        "summary": "Creates a new pet",
        "consumes": ["application/x-yaml"],
        "produces": ["application/x-yaml"],
        "parameters": [
          {
            "name": "pet",
            "in": "body",
            "description": "The Pet to create",
            "required": true,
            "schema": {
              "$ref": "#/definitions/newPet"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Created Pet response",
            "schema": {
              "$ref": "#/definitions/Pet"
            }
          },
          "default": {
            "description": "Unexpected error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      }
    },
    "/pets/{id}": {
      "delete": {
        "security": [
          {
            "apiKey": []
          }
        ],
        "description": "Deletes the Pet by id",
        "operationId": "deletePet",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "description": "ID of pet to delete",
            "required": true,
            "type": "integer",
            "format": "int64"
          }
        ],
        "responses": {
          "204": {
            "description": "pet deleted"
          },
          "default": {
            "description": "unexpected error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      },
      "get": {
        "tags": [ "Pet Operations" ],
        "operationId": "getPetById",
        "summary": "Finds the pet by id",
        "responses": {
          "200": {
            "description": "Pet response",
            "schema": {
              "$ref": "#/definitions/Pet"
            }
          },
          "default": {
            "description": "Unexpected error",
            "schema": {
              "$ref": "#/definitions/Error"
            }
          }
        }
      },
      "parameters": [
        {
          "name": "id",
          "in": "path",
          "description": "ID of pet",
          "required": true,
          "type": "integer",
          "format": "int64"
        }
      ]
    }
  },
  "definitions": {
    "Category": {
      "id": "Category",
      "properties": {
        "id": {
          "format": "int64",
          "type": "integer"
        },
        "name": {
          "type": "string"
        }
      }
    },
    "Pet": {
      "id": "Pet",
      "properties": {
        "category": {
          "$ref": "#/definitions/Category"
        },
        "id": {
          "description": "unique identifier for the pet",
          "format": "int64",
          "maximum": 100.0,
          "minimum": 0.0,
          "type": "integer"
        },
        "name": {
          "type": "string"
        },
        "photoUrls": {
          "items": {
            "type": "string"
          },
          "type": "array"
        },
        "status": {
          "description": "pet status in the store",
          "enum": [
            "available",
            "pending",
            "sold"
          ],
          "type": "string"
        },
        "tags": {
          "items": {
            "$ref": "#/definitions/Tag"
          },
          "type": "array"
        }
      },
      "required": [
        "id",
        "name"
      ]
    },
    "newPet": {
      "anyOf": [
        {
          "$ref": "#/definitions/Pet"
        },
        {
          "required": [
            "name"
          ]
        }
      ]
    },
    "Tag": {
      "id": "Tag",
      "properties": {
        "id": {
          "format": "int64",
          "type": "integer"
        },
        "name": {
          "type": "string"
        }
      }
    },
    "Error": {
      "required": [
        "code",
        "message"
      ],
      "properties": {
        "code": {
          "type": "integer",
          "format": "int32"
        },
        "message": {
          "type": "string"
        }
      }
    }
  },
  "consumes": [
    "application/json",
    "application/xml"
  ],
  "produces": [
    "application/json",
    "application/xml",
    "text/plain",
    "text/html"
  ],
  "securityDefinitions": {
    "basic": {
      "type": "basic"
    },
    "apiKey": {
      "type": "apiKey",
      "in": "header",
      "name": "X-API-KEY"
    }
  }
}
`
