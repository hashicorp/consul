package protohcl

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/protohcl/testproto"
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

// TODO - test invalid timestamp format
// TODO - test Timestamp lower/upper bounds
// TODO - test invalid duration format

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

func TestStruct(t *testing.T) {
	hcl := `
		struct_val = {
			"foo" = "bar"
			"baz" = 1.234
			"nested" = {
				"foo" = 12,
				"bar" = "something"
			}
			"list" = [
				1,
				2,
				3,
			]
			"list_maps" = [
				{
					"arrr" = "matey"
				},
				{
					"hoist" = "the colors"
				}
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
		"foo": "bar",
		"baz": 1.234,
		"nested": {
			"foo": 12,
			"bar": "something"
		},
		"list": [
			1,
			2,
			3
		],
		"list_maps": [
			{
				"arrr": "matey"
			},
			{
				"hoist": "the colors"
			}
		]
	}
	`
	require.JSONEq(t, expected, string(jsonVal))
}
