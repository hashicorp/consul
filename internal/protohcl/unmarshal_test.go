package protohcl

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/protohcl/testproto"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestPrimitives(t *testing.T) {
	hcl := `
		double_val = 1.234
  		float_val = 2.345
  		int32_val = 536870912
  		int64_val = 25769803776
  		uint32_val = 2148532224
  		uint64_val = 9223372041149743104
  		sint32_val = 536870912
  		sint64_val = 25769803776
  		fixed32_val = 2148532224
  		fixed64_val = 9223372041149743104
  		sfixed32_val = 536870912
  		sfixed64_val = 25769803776
  		bool_val = true
  		string_val = "foo"
		// This is base64 encoded "bar"
  		byte_val = "YmFy"
	`

	var out testproto.Primitives

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)

	require.Equal(t, out.DoubleVal, float64(1.234))
	require.Equal(t, out.FloatVal, float32(2.345))
	require.Equal(t, out.Int32Val, int32(536870912))
	require.Equal(t, out.Int64Val, int64(25769803776))
	require.Equal(t, out.Uint32Val, uint32(2148532224))
	require.Equal(t, out.Uint64Val, uint64(9223372041149743104))
	require.Equal(t, out.Sint32Val, int32(536870912))
	require.Equal(t, out.Sint64Val, int64(25769803776))
	require.Equal(t, out.Fixed32Val, uint32(2148532224))
	require.Equal(t, out.Fixed64Val, uint64(9223372041149743104))
	require.Equal(t, out.Sfixed32Val, int32(536870912))
	require.Equal(t, out.Sfixed64Val, int64(25769803776))
	require.Equal(t, out.BoolVal, true)
	require.Equal(t, out.StringVal, "foo")
	require.Equal(t, out.ByteVal, []byte("bar"))
}

func TestNestedAndCollections(t *testing.T) {
	hcl := `
		primitives {
			uint32_val = 42
		}
		
		primitives_map "foo" {
			uint32_val = 42
		}
		
		protocol_map = {
			"foo" = "PROTOCOL_TCP"
		}
		
		primitives_list {
			uint32_val = 42
		}
		
		primitives_list {
			uint32_val = 56
		}
		
		int_list = [
			1,
			2
		]
	
	`

	var out testproto.NestedAndCollections

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)

	require.NotNil(t, out.Primitives)
	require.Equal(t, out.Primitives.Uint32Val, uint32(42))
	require.NotNil(t, out.PrimitivesMap)
	require.Equal(t, out.PrimitivesMap["foo"].Uint32Val, uint32(42))
	require.NotNil(t, out.ProtocolMap)
	require.Equal(t, out.ProtocolMap["foo"], testproto.Protocol_PROTOCOL_TCP)
	require.Len(t, out.PrimitivesList, 2)
	require.Equal(t, out.PrimitivesList[0].Uint32Val, uint32(42))
	require.Equal(t, out.PrimitivesList[1].Uint32Val, uint32(56))
	require.Len(t, out.IntList, 2)
	require.Equal(t, out.IntList[1], int32(2))
}

func TestPrimitiveWrappers(t *testing.T) {
	hcl := `
		double_val = 1.234
  		float_val = 2.345
  		int32_val = 536870912
  		int64_val = 25769803776
  		uint32_val = 2148532224
  		uint64_val = 9223372041149743104
  		bool_val = true
  		string_val = "foo"
		// This is base64 encoded "bar"
  		bytes_val = "YmFy"
	`
	var out testproto.Wrappers

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.Equal(t, out.DoubleVal.Value, float64(1.234))
	require.Equal(t, out.FloatVal.Value, float32(2.345))
	require.Equal(t, out.Int32Val.Value, int32(536870912))
	require.Equal(t, out.Int64Val.Value, int64(25769803776))
	require.Equal(t, out.Uint32Val.Value, uint32(2148532224))
	require.Equal(t, out.Uint64Val.Value, uint64(9223372041149743104))
	require.Equal(t, out.BoolVal.Value, true)
	require.Equal(t, out.StringVal.Value, "foo")
	require.Equal(t, out.BytesVal.Value, []byte("bar"))
}

func TestNonDynamicWellKnown(t *testing.T) {
	hcl := `
		empty_val = {}
		timestamp_val = "2023-02-27T12:34:56.789Z"
		duration_val = "12s"
	`
	var out testproto.NonDynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.EmptyVal)
	require.NotNil(t, out.TimestampVal)
	require.Equal(t, out.TimestampVal.AsTime(), time.Date(2023, 2, 27, 12, 34, 56, 789000000, time.UTC))
	require.NotNil(t, out.DurationVal)
	require.Equal(t, out.DurationVal.AsDuration(), time.Second*12)
}

func TestInvalidTimestamp(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := map[string]struct {
		hcl       string
		expectXDS bool
	}{
		"invalid": {
			hcl: `
				timestamp_val = "Sat Jun 12 2023 14:59:57 GMT+0200"
			`,
		},
		"range error": {
			hcl: `
				timestamp_val = "2023-02-27T25:34:56.789Z"
			`,
		},
	}

	for name, tc := range cases {
		tc := tc
		var out testproto.NonDynamicWellKnown
		t.Run(name, func(t *testing.T) {

			err := Unmarshal([]byte(tc.hcl), &out)
			require.Error(t, err)
			require.Nil(t, out.TimestampVal)
			require.ErrorContains(t, err, "error parsing timestamp")
		})
	}
}

func TestInvalidDuration(t *testing.T) {
	hcl := `
		duration_val = "abc"
	`
	var out testproto.NonDynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.ErrorContains(t, err, "error parsing string duration:")
	require.Nil(t, out.DurationVal)
}

func TestOneOf(t *testing.T) {
	hcl1 := `
		int32_val = 3
	`

	hcl2 := `
		primitives {
			int32_val = 3
		}
	`

	hcl3 := `
		int32_val = 3
		primitives {
			int32_val = 4
		}
	`

	var out testproto.OneOf

	err := Unmarshal([]byte(hcl1), &out)
	require.NoError(t, err)
	require.Equal(t, out.GetInt32Val(), int32(3))

	err = Unmarshal([]byte(hcl2), &out)
	require.NoError(t, err)
	primitives := out.GetPrimitives()
	require.NotNil(t, primitives)
	require.Equal(t, primitives.Int32Val, int32(3))

	err = Unmarshal([]byte(hcl3), &out)
	require.Error(t, err)
}

func TestAny(t *testing.T) {
	hcl := `
		any_val {
		    type_url = "hashicorp.consul.internal.protohcl.testproto.Primitives"
		    uint32_val = 42
		}

		any_list = [
			{
				type_url = "hashicorp.consul.internal.protohcl.testproto.Primitives"
				uint32_val = 123
			},
			{
				type_url = "hashicorp.consul.internal.protohcl.testproto.Wrappers"
				uint32_val = 321
			}
		]
	`
	var out testproto.DynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.AnyVal)
	require.Equal(t, out.AnyVal.TypeUrl, "hashicorp.consul.internal.protohcl.testproto.Primitives")

	raw, err := anypb.UnmarshalNew(out.AnyVal, proto.UnmarshalOptions{})
	require.NoError(t, err)
	require.NotNil(t, raw)

	primitives, ok := raw.(*testproto.Primitives)
	require.True(t, ok)
	require.Equal(t, primitives.Uint32Val, uint32(42))
}

func TestAnyTypeDynamicWellKnown(t *testing.T) {
	hcl := `
		any_val {
			type_url = "hashicorp.consul.internal.protohcl.testproto.DynamicWellKnown"
		    any_val {
				type_url = "hashicorp.consul.internal.protohcl.testproto.Primitives"
				uint32_val = 42
			}
		}
	`
	var out testproto.DynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.AnyVal)
	require.Equal(t, out.AnyVal.TypeUrl, "hashicorp.consul.internal.protohcl.testproto.DynamicWellKnown")

	raw, err := anypb.UnmarshalNew(out.AnyVal, proto.UnmarshalOptions{})
	require.NoError(t, err)
	require.NotNil(t, raw)

	anyVal, ok := raw.(*testproto.DynamicWellKnown)
	require.True(t, ok)

	res, err := anypb.UnmarshalNew(anyVal.AnyVal, proto.UnmarshalOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	primitives, ok := res.(*testproto.Primitives)
	require.True(t, ok)
	require.Equal(t, primitives.Uint32Val, uint32(42))
}

func TestAnyTypeNestedAndCollections(t *testing.T) {
	hcl := `
		any_val {
			type_url = "hashicorp.consul.internal.protohcl.testproto.NestedAndCollections"
		    primitives {
				uint32_val = 42
			}
		}
	`
	var out testproto.DynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.AnyVal)
	require.Equal(t, out.AnyVal.TypeUrl, "hashicorp.consul.internal.protohcl.testproto.NestedAndCollections")

	raw, err := anypb.UnmarshalNew(out.AnyVal, proto.UnmarshalOptions{})
	require.NoError(t, err)
	require.NotNil(t, raw)

	nestedCollections, ok := raw.(*testproto.NestedAndCollections)
	require.True(t, ok)
	require.NotNil(t, nestedCollections.Primitives)
	require.Equal(t, nestedCollections.Primitives.Uint32Val, uint32(42))
}

func TestAnyTypeErrors(t *testing.T) {
	type testCase struct {
		description string
		hcl         string
		error       string
	}
	testCases := []testCase{
		{
			description: "type_url is expected",
			hcl: `
			  any_val {
				uint32_val = 42
			}
			`,
			error: "type_url field is required to decode Any",
		},
		{
			description: "type_url is unknown",
			hcl: `
			  any_val {
				type_url = "hashicorp.consul.internal.protohcl.testproto.Integer"
				uint32_val = 42
			}
			`,
			error: "error looking up type information for hashicorp.consul.internal.protohcl.testproto.Integer",
		},
		{
			description: "unknown field",
			hcl: `
			  any_val {
				type_url = "hashicorp.consul.internal.protohcl.testproto.Primitives"
				int_val = 42
			}
			`,
			error: "Unsupported argument; An argument named \"int_val\" is not expected here",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			var out testproto.DynamicWellKnown

			err := Unmarshal([]byte(tc.hcl), &out)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.error)
		})
	}
}

func TestStruct(t *testing.T) {
	hcl := `
		struct_val = {
			"null"= null
			"bool"= true
			"foo" = "bar"
			"baz" = 1.234
			"nested" = {
				"foo" = 12,
				"bar" = "something"
			}
		}
	`

	var out testproto.DynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.StructVal)

	valMap := out.StructVal.AsMap()
	jsonVal, err := json.Marshal(valMap)
	require.NoError(t, err)

	expected := `{
		"null": null,
		"bool": true,
		"foo": "bar",
		"baz": 1.234,
		"nested": {
			"foo": 12,
			"bar": "something"
		}
	}
	`
	require.JSONEq(t, expected, string(jsonVal))
}

func TestStructList(t *testing.T) {
	hcl := `
		struct_val = {
			"list_int" = [
				1,
				2,
				3,
			]
			"list_string": [
				"abc",
				"def"
			]
			"list_bool": [
				true,
				false
			]
			"list_maps" = [
				{
					"arrr" = "matey"
				},
				{
					"hoist" = "the colors"
				}
			]
			"list_list" = [
				[
					"hello",
					"world",
					null
				]
			]
		}
	`

	var out testproto.DynamicWellKnown

	err := Unmarshal([]byte(hcl), &out)
	require.NoError(t, err)
	require.NotNil(t, out.StructVal)

	valMap := out.StructVal.AsMap()
	jsonVal, err := json.Marshal(valMap)
	require.NoError(t, err)

	expected := `{
		"list_int": [
			1,
			2,
			3
		],
		"list_string": [
			"abc",
			"def"
		],
		"list_bool": [
			true,
			false
		],
		"list_maps": [
			{
				"arrr": "matey"
			},
			{
				"hoist": "the colors"
			}
		],
		"list_list": [
			[
				"hello",
				"world",
				null
			]
		]
	}
	`
	require.JSONEq(t, expected, string(jsonVal))
}

func TestFunctionExecution(t *testing.T) {
	hcl := `
	  id {
		type = testgvk("demo.v1.Artist")
		name = "test"
	  }`

	var out pbresource.Resource

	registry := resource.NewRegistry()
	demo.RegisterTypes(registry)

	var (
		typeType = cty.Capsule("type", reflect.TypeOf(pbresource.Type{}))

		gvk = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "Test GVK String", Type: cty.String},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				t, err := resource.ParseGVK(args[0].AsString())
				if err != nil {
					return cty.NilVal, err
				}
				return cty.CapsuleVal(typeType, t), nil
			},
		})
	)

	err := UnmarshalOptions{
		Functions: map[string]function.Function{"testgvk": gvk},
	}.Unmarshal([]byte(hcl), &out)

	require.NoError(t, err)

	require.Equal(t, "demo", out.Id.Type.Group)
	require.Equal(t, "v1", out.Id.Type.GroupVersion)
	require.Equal(t, "Artist", out.Id.Type.Kind)
	require.Equal(t, "test", out.Id.Name)
}

func TestSkipFields(t *testing.T) {

	u := UnmarshalOptions{}

	hcl := `
		any_val {
			type_url = "hashicorp.consul.internal.protohcl.testproto.Primitives"
			uint32_val = 10
		}`

	file, diags := hclparse.NewParser().ParseHCL([]byte(hcl), "")

	require.False(t, diags.HasErrors())

	decoder := u.bodyDecoder(file.Body)

	decoder = decoder.SkipFields("type_url")

	decoder = decoder.SkipFields("type_url", "uint32_val")

	expected := map[string]struct{}{
		"type_url":   {},
		"uint32_val": {},
	}

	require.Contains(t, fmt.Sprintf("%v", decoder), fmt.Sprintf("%v", expected))
}
