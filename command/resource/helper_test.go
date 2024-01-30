// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseJson(t *testing.T) {
	tests := []struct {
		name    string
		js      string
		wantErr bool
	}{
		{"valid resource", "{\n    \"data\": {\n        \"genre\": \"GENRE_METAL\",\n        \"name\": \"Korn\"\n    },\n    \"generation\": \"01HAYWBPV1KMT2KWECJ6CEWDQ0\",\n    \"id\": {\n        \"name\": \"korn\",\n        \"tenancy\": {\n            \"namespace\": \"default\",\n            \"partition\": \"default\"\n            },\n        \"type\": {\n            \"group\": \"demo\",\n            \"groupVersion\": \"v2\",\n            \"kind\": \"Artist\"\n        },\n        \"uid\": \"01HAYWBPV1KMT2KWECJ4NW88S1\"\n    },\n    \"metadata\": {\n        \"foo\": \"bar\"\n    },\n    \"version\": \"18\"\n}", false},
		{"invalid resource", "{\n    \"data\": {\n        \"genre\": \"GENRE_METAL\",\n        \"name\": \"Korn\"\n    },\n    \"id\": {\n        \"name\": \"korn\",\n        \"tenancy\": {\n            \"namespace\": \"default\",\n            \"partition\": \"default\"\n        },\n        \"type\": \"\"\n    },\n    \"metadata\": {\n        \"foo\": \"bar\"\n    }\n}\n", true},
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
