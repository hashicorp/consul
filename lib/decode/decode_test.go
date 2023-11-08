// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
)

func TestHookTranslateKeys(t *testing.T) {
	var testcases = []struct {
		name     string
		data     interface{}
		expected interface{}
	}{
		{
			name: "target of type struct, with struct receiver",
			data: map[string]interface{}{
				"S": map[string]interface{}{
					"None":   "no translation",
					"OldOne": "value1",
					"oldtwo": "value2",
				},
			},
			expected: Config{
				S: TypeStruct{
					One:  "value1",
					Two:  "value2",
					None: "no translation",
				},
			},
		},
		{
			name: "target of type ptr, with struct receiver",
			data: map[string]interface{}{
				"PS": map[string]interface{}{
					"None":   "no translation",
					"OldOne": "value1",
					"oldtwo": "value2",
				},
			},
			expected: Config{
				PS: &TypeStruct{
					One:  "value1",
					Two:  "value2",
					None: "no translation",
				},
			},
		},
		{
			name: "target of type ptr, with ptr receiver",
			data: map[string]interface{}{
				"PTR": map[string]interface{}{
					"None":      "no translation",
					"old_THREE": "value3",
					"oldfour":   "value4",
				},
			},
			expected: Config{
				PTR: &TypePtrToStruct{
					Three: "value3",
					Four:  "value4",
					None:  "no translation",
				},
			},
		},
		{
			name: "target of type ptr, with struct receiver",
			data: map[string]interface{}{
				"PTRS": map[string]interface{}{
					"None":      "no translation",
					"old_THREE": "value3",
					"old_four":  "value4",
				},
			},
			expected: Config{
				PTRS: TypePtrToStruct{
					Three: "value3",
					Four:  "value4",
					None:  "no translation",
				},
			},
		},
		{
			name: "target of type map",
			data: map[string]interface{}{
				"Blob": map[string]interface{}{
					"one": 1,
					"two": 2,
				},
			},
			expected: Config{
				Blob: map[string]interface{}{
					"one": 1,
					"two": 2,
				},
			},
		},
		{
			name: "value already exists for canonical key",
			data: map[string]interface{}{
				"PS": map[string]interface{}{
					"OldOne": "value1",
					"One":    "original1",
					"oldTWO": "value2",
					"two":    "original2",
				},
			},
			expected: Config{
				PS: &TypeStruct{
					One: "original1",
					Two: "original2",
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{}
			md := new(mapstructure.Metadata)
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				DecodeHook: HookTranslateKeys,
				Metadata:   md,
				Result:     &cfg,
			})
			require.NoError(t, err)

			require.NoError(t, decoder.Decode(tc.data))
			require.Equal(t, cfg, tc.expected, "decode metadata: %#v", md)
		})
	}
}

type Config struct {
	S    TypeStruct
	PS   *TypeStruct
	PTR  *TypePtrToStruct
	PTRS TypePtrToStruct
	Blob map[string]interface{}
}

type TypeStruct struct {
	One  string `alias:"oldone"`
	Two  string `alias:"oldtwo"`
	None string
}

type TypePtrToStruct struct {
	Three string `alias:"old_three"`
	Four  string `alias:"old_four,oldfour"`
	None  string
}

func TestHookTranslateKeys_TargetStructHasPointerReceiver(t *testing.T) {
	target := &TypePtrToStruct{}
	md := new(mapstructure.Metadata)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookTranslateKeys,
		Metadata:   md,
		Result:     target,
	})
	require.NoError(t, err)

	data := map[string]interface{}{
		"None":      "no translation",
		"Old_Three": "value3",
		"OldFour":   "value4",
	}
	expected := &TypePtrToStruct{
		None:  "no translation",
		Three: "value3",
		Four:  "value4",
	}
	require.NoError(t, decoder.Decode(data))
	require.Equal(t, expected, target, "decode metadata: %#v", md)
}

func TestHookTranslateKeys_DoesNotModifySourceData(t *testing.T) {
	raw := map[string]interface{}{
		"S": map[string]interface{}{
			"None":   "no translation",
			"OldOne": "value1",
			"oldtwo": "value2",
		},
	}

	cfg := Config{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookTranslateKeys,
		Result:     &cfg,
	})
	require.NoError(t, err)
	require.NoError(t, decoder.Decode(raw))

	expected := map[string]interface{}{
		"S": map[string]interface{}{
			"None":   "no translation",
			"OldOne": "value1",
			"oldtwo": "value2",
		},
	}
	require.Equal(t, raw, expected)
}

type translateExample struct {
	FieldDefaultCanonical        string `alias:"first"`
	FieldWithMapstructureTag     string `alias:"second" mapstructure:"field_with_mapstruct_tag"`
	FieldWithMapstructureTagOmit string `mapstructure:"field_with_mapstruct_omit,omitempty" alias:"third"`
	FieldWithEmptyTag            string `mapstructure:"" alias:"forth"`
	EmbeddedStruct               `mapstructure:",squash"`
	*PtrEmbeddedStruct           `mapstructure:",squash"`
	BadField                     string `mapstructure:",squash"`
}

type EmbeddedStruct struct {
	NextField string `alias:"next"`
}

type PtrEmbeddedStruct struct {
	OtherNextField string `alias:"othernext"`
}

func TestTranslationsForType(t *testing.T) {
	to := reflect.TypeOf(translateExample{})
	actual := translationsForType(to)
	expected := map[string]string{
		"first":     "fielddefaultcanonical",
		"second":    "field_with_mapstruct_tag",
		"third":     "field_with_mapstruct_omit",
		"forth":     "fieldwithemptytag",
		"next":      "nextfield",
		"othernext": "othernextfield",
	}
	require.Equal(t, expected, actual)
}

type nested struct {
	O      map[string]interface{}
	Slice  []Item
	Item   Item
	OSlice []map[string]interface{}
	Sub    *nested
}

type Item struct {
	Name string
}

func TestHookWeakDecodeFromSlice_DoesNotModifySliceTargets(t *testing.T) {
	source := `
slice {
    name = "first"
}
slice {
    name = "second"
}
item {
	name = "solo"
}
sub {
	oslice {
		something = "v1"
	}
}
`
	target := &nested{}
	err := decodeHCLToMapStructure(source, target)
	require.NoError(t, err)

	expected := &nested{
		Slice: []Item{{Name: "first"}, {Name: "second"}},
		Item:  Item{Name: "solo"},
		Sub: &nested{
			OSlice: []map[string]interface{}{
				{"something": "v1"},
			},
		},
	}
	require.Equal(t, target, expected)
}

func decodeHCLToMapStructure(source string, target interface{}) error {
	raw := map[string]interface{}{}
	err := hcl.Decode(&raw, source)
	if err != nil {
		return err
	}

	md := new(mapstructure.Metadata)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookWeakDecodeFromSlice,
		Metadata:   md,
		Result:     target,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(&raw)
}

func TestHookWeakDecodeFromSlice_DoesNotModifySliceTargetsFromSliceInterface(t *testing.T) {
	raw := map[string]interface{}{
		"slice": []interface{}{map[string]interface{}{"name": "first"}},
		"item":  []interface{}{map[string]interface{}{"name": "solo"}},
		"sub": []interface{}{
			map[string]interface{}{
				"OSlice": []interface{}{
					map[string]interface{}{"something": "v1"},
				},
				"item": []interface{}{map[string]interface{}{"name": "subitem"}},
			},
		},
	}
	target := &nested{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: HookWeakDecodeFromSlice,
		Result:     target,
	})
	require.NoError(t, err)
	err = decoder.Decode(&raw)
	require.NoError(t, err)

	expected := &nested{
		Slice: []Item{{Name: "first"}},
		Item:  Item{Name: "solo"},
		Sub: &nested{
			OSlice: []map[string]interface{}{
				{"something": "v1"},
			},
			Item: Item{Name: "subitem"},
		},
	}
	require.Equal(t, target, expected)
}

func TestHookWeakDecodeFromSlice_ErrorsWithMultipleNestedBlocks(t *testing.T) {
	source := `
item {
    name = "first"
}
item {
    name = "second"
}
`
	target := &nested{}
	err := decodeHCLToMapStructure(source, target)
	require.Error(t, err)
	require.Contains(t, err.Error(), "'Item' expected a map, got 'slice'")
}

func TestHookWeakDecodeFromSlice_UnpacksNestedBlocks(t *testing.T) {
	source := `
item {
    name = "first"
}
`
	target := &nested{}
	err := decodeHCLToMapStructure(source, target)
	require.NoError(t, err)

	expected := &nested{
		Item: Item{Name: "first"},
	}
	require.Equal(t, target, expected)
}

func TestHookWeakDecodeFromSlice_NestedOpaqueConfig(t *testing.T) {
	source := `
service {
  proxy {
    config {
      envoy_gateway_bind_addresses {
        all-interfaces {
          address = "0.0.0.0"
          port = 8443
        }
      }
    }
  }
}`

	target := map[string]interface{}{}
	err := decodeHCLToMapStructure(source, &target)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"service": map[string]interface{}{
			"proxy": map[string]interface{}{
				"config": map[string]interface{}{
					"envoy_gateway_bind_addresses": map[string]interface{}{
						"all-interfaces": map[string]interface{}{
							"address": "0.0.0.0",
							"port":    8443,
						},
					},
				},
			},
		},
	}
	require.Equal(t, target, expected)
}

func TestFieldTags(t *testing.T) {
	type testCase struct {
		tags     string
		expected mapstructureFieldTags
	}

	fn := func(t *testing.T, tc testCase) {
		tag := fmt.Sprintf(`mapstructure:"%v"`, tc.tags)
		field := reflect.StructField{
			Tag:  reflect.StructTag(tag),
			Name: "Original",
		}
		actual := fieldTags(field)
		require.Equal(t, tc.expected, actual)
	}

	var testCases = []testCase{
		{tags: "", expected: mapstructureFieldTags{name: "Original"}},
		{tags: "just-a-name", expected: mapstructureFieldTags{name: "just-a-name"}},
		{tags: "name,squash", expected: mapstructureFieldTags{name: "name", squash: true}},
		{tags: ",squash", expected: mapstructureFieldTags{name: "Original", squash: true}},
		{tags: ",omitempty,squash", expected: mapstructureFieldTags{name: "Original", squash: true}},
		{tags: "named,omitempty,squash", expected: mapstructureFieldTags{name: "named", squash: true}},
	}

	for _, tc := range testCases {
		t.Run(tc.tags, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
