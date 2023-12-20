// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStructs_PreparedQuery_GetACLPrefix(t *testing.T) {
	ephemeral := &PreparedQuery{}
	if prefix, ok := ephemeral.GetACLPrefix(); ok {
		t.Fatalf("bad: %s", prefix)
	}

	named := &PreparedQuery{
		Name: "hello",
	}
	if prefix, ok := named.GetACLPrefix(); !ok || prefix != "hello" {
		t.Fatalf("bad: ok=%v, prefix=%#v", ok, prefix)
	}

	tmpl := &PreparedQuery{
		Name: "",
		Template: QueryTemplateOptions{
			Type: QueryTemplateTypeNamePrefixMatch,
		},
	}
	if prefix, ok := tmpl.GetACLPrefix(); !ok || prefix != "" {
		t.Fatalf("bad: ok=%v prefix=%#v", ok, prefix)
	}
}

func TestPreparedQueryExecuteRequest_CacheInfoKey(t *testing.T) {
	// TODO: should these fields be included in the key?
	ignored := []string{"Agent", "QueryOptions"}
	assertCacheInfoKeyIsComplete(t, &PreparedQueryExecuteRequest{}, ignored...)
}

func TestQueryFailoverOptions_IsEmpty(t *testing.T) {
	tests := []struct {
		name            string
		query           QueryFailoverOptions
		isExpectedEmpty bool
	}{
		{
			name:            "expect empty",
			query:           QueryFailoverOptions{},
			isExpectedEmpty: true,
		},
		{
			name: "expect not empty NearestN",
			query: QueryFailoverOptions{
				NearestN: 1,
			},
			isExpectedEmpty: false,
		},
		{
			name: "expect not empty NearestN negative",
			query: QueryFailoverOptions{
				NearestN: -1,
			},
			isExpectedEmpty: false,
		},
		{
			name: "expect not empty datacenters",
			query: QueryFailoverOptions{
				Datacenters: []string{"dc"},
			},
			isExpectedEmpty: false,
		},
		{
			name: "expect not empty targets",
			query: QueryFailoverOptions{
				Targets: []QueryFailoverTarget{
					{
						Peer: "peer",
					},
				},
			},
			isExpectedEmpty: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isExpectedEmpty, tt.query.IsEmpty())
		})
	}
}
