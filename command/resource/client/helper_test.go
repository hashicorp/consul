// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTValue(t *testing.T) {
	t.Run("String: set", func(t *testing.T) {
		var tv TValue[string]

		err := tv.Set("testString")
		assert.NoError(t, err)

		assert.Equal(t, *tv.v, "testString")
	})

	t.Run("String: merge", func(t *testing.T) {
		var tv TValue[string]
		var onto string

		testStr := "testString"
		tv.v = &testStr
		tv.Merge(&onto)

		assert.Equal(t, onto, "testString")
	})

	t.Run("String: merge nil", func(t *testing.T) {
		var tv TValue[string]
		var onto *string = nil

		testStr := "testString"
		tv.v = &testStr
		err := tv.Merge(onto)

		assert.Equal(t, err.Error(), "onto is nil")
	})

	t.Run("Get string", func(t *testing.T) {
		var tv TValue[string]
		testStr := "testString"
		tv.v = &testStr
		assert.Equal(t, tv.String(), "testString")
	})

	t.Run("Bool: set", func(t *testing.T) {
		var tv TValue[bool]

		err := tv.Set("true")
		assert.NoError(t, err)

		assert.Equal(t, *tv.v, true)
	})

	t.Run("Bool: merge", func(t *testing.T) {
		var tv TValue[bool]
		var onto bool

		testBool := true
		tv.v = &testBool
		tv.Merge(&onto)

		assert.Equal(t, onto, true)
	})
}

func Test_parseJson(t *testing.T) {
	tests := []struct {
		name    string
		js      string
		wantErr bool
	}{
		{"valid resource", "{\n    \"data\": {\n        \"genre\": \"GENRE_METAL\",\n        \"name\": \"Korn\"\n    },\n    \"generation\": \"01HAYWBPV1KMT2KWECJ6CEWDQ0\",\n    \"id\": {\n        \"name\": \"korn\",\n        \"tenancy\": {\n            \"namespace\": \"default\",\n            \"partition\": \"default\"            },\n        \"type\": {\n            \"group\": \"demo\",\n            \"groupVersion\": \"v2\",\n            \"kind\": \"Artist\"\n        },\n        \"uid\": \"01HAYWBPV1KMT2KWECJ4NW88S1\"\n    },\n    \"metadata\": {\n        \"foo\": \"bar\"\n    },\n    \"version\": \"18\"\n}", false},
		{"invalid resource", "{\n    \"data\": {\n        \"genre\": \"GENRE_METAL\",\n        \"name\": \"Korn\"\n    },\n    \"id\": {\n        \"name\": \"korn\",\n        \"tenancy\": {\n            \"namespace\": \"default\",\n            \"partition\": \"default\"            },\n        \"type\": \"\"\n    },\n    \"metadata\": {\n        \"foo\": \"bar\"\n    }\n}\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJson(tt.js)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
			}

		})
	}
}
