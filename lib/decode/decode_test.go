package decode

import (
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

type translateExample struct {
	FieldDefaultCanonical        string `alias:"first"`
	FieldWithMapstructureTag     string `alias:"second" mapstructure:"field_with_mapstruct_tag"`
	FieldWithMapstructureTagOmit string `mapstructure:"field_with_mapstruct_omit,omitempty" alias:"third"`
	FieldWithEmptyTag            string `mapstructure:"" alias:"forth"`
}

func TestTranslationsForType(t *testing.T) {
	to := reflect.TypeOf(translateExample{})
	actual := translationsForType(to)
	expected := map[string]string{
		"first":  "fielddefaultcanonical",
		"second": "field_with_mapstruct_tag",
		"third":  "field_with_mapstruct_omit",
		"forth":  "fieldwithemptytag",
	}
	require.Equal(t, expected, actual)
}

type nested struct {
	O     map[string]interface{}
	Slice []Item
	Item  Item
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
`
	target := &nested{}
	err := decodeHCLToMapStructure(source, target)
	require.NoError(t, err)

	expected := &nested{
		Slice: []Item{{Name: "first"}, {Name: "second"}},
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
	return decoder.Decode(&raw)
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
