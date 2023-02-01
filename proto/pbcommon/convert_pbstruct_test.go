package pbcommon

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestEnvoyExtensionsToStructs(t *testing.T) {
	input := []*EnvoyExtension{
		{
			Name:     "ext1",
			Required: true,
			Arguments: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"field1": {Kind: &structpb.Value_StringValue{StringValue: "value1"}},
					"field2": {Kind: &structpb.Value_NumberValue{NumberValue: 3.14}},
				},
			},
		},
		{
			Name:     "ext2",
			Required: false,
			Arguments: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"field3": {Kind: &structpb.Value_StringValue{StringValue: "value2"}},
					"field4": {Kind: &structpb.Value_NumberValue{NumberValue: 2.718}},
				},
			},
		},
	}
	expected := []structs.EnvoyExtension{
		{
			Name:     "ext1",
			Required: true,
			Arguments: map[string]interface{}{
				"field1": "value1",
				"field2": 3.14,
			},
		},
		{
			Name:     "ext2",
			Required: false,
			Arguments: map[string]interface{}{
				"field3": "value2",
				"field4": 2.718,
			},
		},
	}

	result := EnvoyExtensionsToStructs(input)
	assert.Equal(t, expected, result)
}

func TestEnvoyExtensionsFromStructs(t *testing.T) {
	input := []structs.EnvoyExtension{
		{
			Name:     "ext1",
			Required: true,
			Arguments: map[string]interface{}{
				"field1": "value1",
				"field2": 3.14,
			},
		},
		{
			Name:     "ext2",
			Required: false,
			Arguments: map[string]interface{}{
				"field3": "value2",
				"field4": 2.718,
			},
		},
	}
	expected := []*EnvoyExtension{
		{
			Name:     "ext1",
			Required: true,
			Arguments: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"field1": {Kind: &structpb.Value_StringValue{StringValue: "value1"}},
					"field2": {Kind: &structpb.Value_NumberValue{NumberValue: 3.14}},
				},
			},
		},
		{
			Name:     "ext2",
			Required: false,
			Arguments: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"field3": {Kind: &structpb.Value_StringValue{StringValue: "value2"}},
					"field4": {Kind: &structpb.Value_NumberValue{NumberValue: 2.718}},
				},
			},
		},
	}

	result := EnvoyExtensionsFromStructs(input)
	assert.Equal(t, expected, result)
}

func TestSliceToPBListValue(t *testing.T) {
	s := []interface{}{1, 2, 3}
	expected, _ := structpb.NewList(s)
	tests := []struct {
		input    []interface{}
		expected *structpb.ListValue
	}{
		{
			[]interface{}{1, 2, 3},
			expected,
		},
		{
			[]interface{}{},
			nil,
		},
		{
			nil,
			nil,
		},
	}

	for _, tc := range tests {
		result := SliceToPBListValue(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}
