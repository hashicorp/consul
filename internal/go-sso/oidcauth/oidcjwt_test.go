// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidcauth

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStringMetadata(t *testing.T) {
	emptyMap := make(map[string]string)

	tests := map[string]struct {
		allClaims     map[string]interface{}
		claimMappings map[string]string
		expected      map[string]string
		errExpected   bool
	}{
		"empty": {nil, nil, emptyMap, false},
		"all": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data2": "val2",
			},
			map[string]string{
				"val1": "foo",
				"val2": "bar",
			},
			false,
		},
		"some": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data3": "val2",
			},
			map[string]string{
				"val1": "foo",
			},
			false,
		},
		"none": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data8": "val1",
				"data9": "val2",
			},
			emptyMap,
			false,
		},

		"nested data": {
			map[string]interface{}{
				"data1": "foo",
				"data2": map[string]interface{}{
					"child": "bar",
				},
				"data3": true,
				"data4": false,
				"data5": float64(7.9),
				"data6": json.Number("-12345"),
				"data7": int(42),
			},
			map[string]string{
				"data1":        "val1",
				"/data2/child": "val2",
				"data3":        "val3",
				"data4":        "val4",
				"data5":        "val5",
				"data6":        "val6",
				"data7":        "val7",
			},
			map[string]string{
				"val1": "foo",
				"val2": "bar",
				"val3": "true",
				"val4": "false",
				"val5": "7",
				"val6": "-12345",
				"val7": "42",
			},
			false,
		},

		"error: a struct isn't stringifiable": {
			map[string]interface{}{
				"data1": map[string]interface{}{
					"child": "bar",
				},
			},
			map[string]string{
				"data1": "val1",
			},
			nil,
			true,
		},
		"error: a slice isn't stringifiable": {
			map[string]interface{}{
				"data1": []interface{}{
					"child", "bar",
				},
			},
			map[string]string{
				"data1": "val1",
			},
			nil,
			true,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			actual, err := extractStringMetadata(nil, test.allClaims, test.claimMappings)
			if test.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, actual)
			}
		})
	}
}

func TestExtractListMetadata(t *testing.T) {
	emptyMap := make(map[string][]string)

	tests := map[string]struct {
		allClaims     map[string]interface{}
		claimMappings map[string]string
		expected      map[string][]string
		errExpected   bool
	}{
		"empty": {nil, nil, emptyMap, false},
		"all - singular": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data2": "val2",
			},
			map[string][]string{
				"val1": {"foo"},
				"val2": {"bar"},
			},
			false,
		},
		"some - singular": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data3": "val2",
			},
			map[string][]string{
				"val1": {"foo"},
			},
			false,
		},
		"none - singular": {
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data8": "val1",
				"data9": "val2",
			},
			emptyMap,
			false,
		},

		"nested data - singular": {
			map[string]interface{}{
				"data1": "foo",
				"data2": map[string]interface{}{
					"child": "bar",
				},
				"data3": true,
				"data4": false,
				"data5": float64(7.9),
				"data6": json.Number("-12345"),
				"data7": int(42),
				"data8": []interface{}{ // mixed
					"foo", true, float64(7.9), json.Number("-12345"), int(42),
				},
			},
			map[string]string{
				"data1":        "val1",
				"/data2/child": "val2",
				"data3":        "val3",
				"data4":        "val4",
				"data5":        "val5",
				"data6":        "val6",
				"data7":        "val7",
				"data8":        "val8",
			},
			map[string][]string{
				"val1": {"foo"},
				"val2": {"bar"},
				"val3": {"true"},
				"val4": {"false"},
				"val5": {"7"},
				"val6": {"-12345"},
				"val7": {"42"},
				"val8": {
					"foo", "true", "7", "-12345", "42",
				},
			},
			false,
		},

		"error: a struct isn't stringifiable (singular)": {
			map[string]interface{}{
				"data1": map[string]interface{}{
					"child": map[string]interface{}{
						"inner": "bar",
					},
				},
			},
			map[string]string{
				"data1": "val1",
			},
			nil,
			true,
		},
		"error: a slice isn't stringifiable (singular)": {
			map[string]interface{}{
				"data1": []interface{}{
					"child", []interface{}{"bar"},
				},
			},
			map[string]string{
				"data1": "val1",
			},
			nil,
			true,
		},

		"non-string-slice data (string)": {
			map[string]interface{}{
				"data1": "foo",
			},
			map[string]string{
				"data1": "val1",
			},
			map[string][]string{
				"val1": {"foo"}, // singular values become lists
			},
			false,
		},

		"all - list": {
			map[string]interface{}{
				"data1": []interface{}{"foo", "otherFoo"},
				"data2": []interface{}{"bar", "otherBar"},
			},
			map[string]string{
				"data1": "val1",
				"data2": "val2",
			},
			map[string][]string{
				"val1": {"foo", "otherFoo"},
				"val2": {"bar", "otherBar"},
			},
			false,
		},
		"some - list": {
			map[string]interface{}{
				"data1": []interface{}{"foo", "otherFoo"},
				"data2": map[string]interface{}{
					"child": []interface{}{"bar", "otherBar"},
				},
			},
			map[string]string{
				"data1":        "val1",
				"/data2/child": "val2",
			},
			map[string][]string{
				"val1": {"foo", "otherFoo"},
				"val2": {"bar", "otherBar"},
			},
			false,
		},
		"none - list": {
			map[string]interface{}{
				"data1": []interface{}{"foo"},
				"data2": []interface{}{"bar"},
			},
			map[string]string{
				"data8": "val1",
				"data9": "val2",
			},
			emptyMap,
			false,
		},
		"list omits empty strings": {
			map[string]interface{}{
				"data1": []interface{}{"foo", "", "otherFoo", ""},
				"data2": "",
			},
			map[string]string{
				"data1": "val1",
				"data2": "val2",
			},
			map[string][]string{
				"val1": {"foo", "otherFoo"},
				"val2": {},
			},
			false,
		},

		"nested data - list": {
			map[string]interface{}{
				"data1": []interface{}{"foo"},
				"data2": map[string]interface{}{
					"child": []interface{}{"bar"},
				},
				"data3": []interface{}{true},
				"data4": []interface{}{false},
				"data5": []interface{}{float64(7.9)},
				"data6": []interface{}{json.Number("-12345")},
				"data7": []interface{}{int(42)},
				"data8": []interface{}{ // mixed
					"foo", true, float64(7.9), json.Number("-12345"), int(42),
				},
			},
			map[string]string{
				"data1":        "val1",
				"/data2/child": "val2",
				"data3":        "val3",
				"data4":        "val4",
				"data5":        "val5",
				"data6":        "val6",
				"data7":        "val7",
				"data8":        "val8",
			},
			map[string][]string{
				"val1": {"foo"},
				"val2": {"bar"},
				"val3": {"true"},
				"val4": {"false"},
				"val5": {"7"},
				"val6": {"-12345"},
				"val7": {"42"},
				"val8": {
					"foo", "true", "7", "-12345", "42",
				},
			},
			false,
		},

		"JSONPointer": {
			map[string]interface{}{
				"foo": "a",
				"bar": map[string]interface{}{
					"baz": []string{"x", "y", "z"},
				},
			},
			map[string]string{
				"foo":        "val1",
				"/bar/baz/1": "val2",
			},
			map[string][]string{
				"val1": {"a"},
				"val2": {"y"},
			},
			false,
		},
		"JSONPointer not found": {
			map[string]interface{}{
				"foo": "a",
				"bar": map[string]interface{}{
					"baz": []string{"x", "y", "z"},
				},
			},
			map[string]string{
				"foo":           "val1",
				"/bar/XXX/1243": "val2",
			},
			map[string][]string{
				"val1": {"a"},
			},
			false,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			actual, err := extractListMetadata(nil, test.allClaims, test.claimMappings)
			if test.errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, actual)
			}
		})
	}
}

func TestGetClaim(t *testing.T) {
	data := `{
		"a": 42,
		"b": "bar",
		"c": {
			"d": 95,
			"e": [
				"dog",
				"cat",
				"bird"
			],
			"f": {
				"g": "zebra"
			}
		},
		"h": true,
		"i": false
	}`
	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(data), &claims))

	tests := []struct {
		claim string
		value interface{}
	}{
		{"a", float64(42)},
		{"/a", float64(42)},
		{"b", "bar"},
		{"/c/d", float64(95)},
		{"/c/e/1", "cat"},
		{"/c/f/g", "zebra"},
		{"nope", nil},
		{"/c/f/h", nil},
		{"", nil},
		{"\\", nil},
		{"h", true},
		{"i", false},
		{"/c/e", []interface{}{"dog", "cat", "bird"}},
		{"/c/f", map[string]interface{}{"g": "zebra"}},
	}

	for _, test := range tests {
		t.Run(test.claim, func(t *testing.T) {
			v := getClaim(nil, claims, test.claim)
			require.Equal(t, test.value, v)
		})
	}
}

func TestNormalizeList(t *testing.T) {
	tests := []struct {
		raw        interface{}
		normalized []interface{}
		ok         bool
	}{
		{
			raw:        []interface{}{"green", 42},
			normalized: []interface{}{"green", 42},
			ok:         true,
		},
		{
			raw:        []interface{}{"green"},
			normalized: []interface{}{"green"},
			ok:         true,
		},
		{
			raw:        []interface{}{},
			normalized: []interface{}{},
			ok:         true,
		},
		{
			raw:        "green",
			normalized: []interface{}{"green"},
			ok:         true,
		},
		{
			raw:        "",
			normalized: []interface{}{""},
			ok:         true,
		},
		{
			raw:        42,
			normalized: []interface{}{42},
			ok:         true,
		},
		{
			raw:        struct{ A int }{A: 5},
			normalized: nil,
			ok:         false,
		},
		{
			raw:        nil,
			normalized: nil,
			ok:         false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%#v", tc.raw), func(t *testing.T) {
			normalized, ok := normalizeList(tc.raw)
			assert.Equal(t, tc.normalized, normalized)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

func TestStringifyMetadataValue(t *testing.T) {
	cases := map[string]struct {
		value         interface{}
		expect        string
		expectFailure bool
	}{
		"empty string": {"", "", false},
		"string":       {"foo", "foo", false},
		"true":         {true, "true", false},
		"false":        {false, "false", false},
		"json number":  {json.Number("-12345"), "-12345", false},
		"float64":      {float64(7.9), "7", false},
		//
		"float32": {float32(7.9), "7", false},
		"int8":    {int8(42), "42", false},
		"int16":   {int16(42), "42", false},
		"int32":   {int32(42), "42", false},
		"int64":   {int64(42), "42", false},
		"int":     {int(42), "42", false},
		"uint8":   {uint8(42), "42", false},
		"uint16":  {uint16(42), "42", false},
		"uint32":  {uint32(42), "42", false},
		"uint64":  {uint64(42), "42", false},
		"uint":    {uint(42), "42", false},
		// fail
		"string slice": {[]string{"a"}, "", true},
		"int slice":    {[]int64{99}, "", true},
		"map":          {map[string]int{"a": 99}, "", true},
		"nil":          {nil, "", true},
		"struct":       {struct{ A int }{A: 5}, "", true},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got, ok := stringifyMetadataValue(tc.value)
			if tc.expectFailure {
				require.False(t, ok)
			} else {
				require.True(t, ok)
				require.Equal(t, tc.expect, got)
			}
		})
	}
}

func TestValidateAudience(t *testing.T) {
	tests := []struct {
		boundAudiences    []string
		audience          []string
		errExpectedLax    bool
		errExpectedStrict bool
	}{
		{[]string{"a"}, []string{"a"}, false, false},
		{[]string{"a"}, []string{"b"}, true, true},
		{[]string{"a"}, []string{""}, true, true},
		{[]string{}, []string{"a"}, false, true},
		{[]string{"a", "b"}, []string{"a"}, false, false},
		{[]string{"a", "b"}, []string{"b"}, false, false},
		{[]string{"a", "b"}, []string{"a", "b", "c"}, false, false},
		{[]string{"a", "b"}, []string{"c", "d"}, true, true},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(fmt.Sprintf(
			"boundAudiences=%#v audience=%#v strict=false",
			tc.boundAudiences, tc.audience,
		), func(t *testing.T) {
			err := validateAudience(tc.boundAudiences, tc.audience, false)
			if tc.errExpectedLax {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})

		t.Run(fmt.Sprintf(
			"boundAudiences=%#v audience=%#v strict=true",
			tc.boundAudiences, tc.audience,
		), func(t *testing.T) {
			err := validateAudience(tc.boundAudiences, tc.audience, true)
			if tc.errExpectedStrict {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
