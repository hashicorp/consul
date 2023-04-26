// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func TestFilter_DirEnt(t *testing.T) {
	t.Parallel()
	policy, _ := acl.NewPolicyFromSource(testFilterRules, nil, nil)
	aclR, _ := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)

	type tcase struct {
		in  []string
		out []string
	}
	cases := []tcase{
		{
			in:  []string{"foo/test", "foo/priv/nope", "foo/other", "zoo"},
			out: []string{"foo/test", "foo/other"},
		},
		{
			in:  []string{"abe", "lincoln"},
			out: nil,
		},
		{
			in:  []string{"abe", "foo/1", "foo/2", "foo/3", "nope"},
			out: []string{"foo/1", "foo/2", "foo/3"},
		},
	}

	for _, tc := range cases {
		ents := structs.DirEntries{}
		for _, in := range tc.in {
			ents = append(ents, &structs.DirEntry{Key: in})
		}

		ents = FilterDirEnt(aclR, ents)
		var outL []string
		for _, e := range ents {
			outL = append(outL, e.Key)
		}

		if !reflect.DeepEqual(outL, tc.out) {
			t.Fatalf("bad: %#v %#v", outL, tc.out)
		}
	}
}

func TestFilter_TxnResults(t *testing.T) {
	t.Parallel()
	policy, _ := acl.NewPolicyFromSource(testFilterRules, nil, nil)
	aclR, _ := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)

	type tcase struct {
		in  []string
		out []string
	}
	cases := []tcase{
		{
			in:  []string{"foo/test", "foo/priv/nope", "foo/other", "zoo"},
			out: []string{"foo/test", "foo/other"},
		},
		{
			in:  []string{"abe", "lincoln"},
			out: nil,
		},
		{
			in:  []string{"abe", "foo/1", "foo/2", "foo/3", "nope"},
			out: []string{"foo/1", "foo/2", "foo/3"},
		},
	}

	for _, tc := range cases {
		results := structs.TxnResults{}
		for _, in := range tc.in {
			results = append(results, &structs.TxnResult{KV: &structs.DirEntry{Key: in}})
		}

		results = FilterTxnResults(aclR, results)
		var outL []string
		for _, r := range results {
			outL = append(outL, r.KV.Key)
		}

		if !reflect.DeepEqual(outL, tc.out) {
			t.Fatalf("bad: %#v %#v", outL, tc.out)
		}
	}

	// Run a non-KV result.
	results := structs.TxnResults{}
	results = append(results, &structs.TxnResult{})
	results = FilterTxnResults(aclR, results)
	if len(results) != 1 {
		t.Fatalf("should not have filtered non-KV result")
	}
}

var testFilterRules = `
key_prefix "" {
	policy = "deny"
}
key_prefix "foo/" {
	policy = "read"
}
key_prefix "foo/priv/" {
	policy = "deny"
}
key_prefix "zip/" {
	policy = "read"
}
`
