// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestFilterResourcesByMetadata(t *testing.T) {
	type testcase struct {
		in        []*pbresource.Resource
		filter    string
		expect    []*pbresource.Resource
		expectErr string
	}

	create := func(name string, kvs ...string) *pbresource.Resource {
		require.True(t, len(kvs)%2 == 0)

		meta := make(map[string]string)
		for i := 0; i < len(kvs); i += 2 {
			meta[kvs[i]] = kvs[i+1]
		}

		return &pbresource.Resource{
			Id: &pbresource.ID{
				Name: name,
			},
			Metadata: meta,
		}
	}

	run := func(t *testing.T, tc testcase) {
		got, err := FilterResourcesByMetadata(tc.in, tc.filter)
		if tc.expectErr != "" {
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, tc.expectErr)
		} else {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got)
		}
	}

	cases := map[string]testcase{
		"nil input": {},
		"no filter": {
			in: []*pbresource.Resource{
				create("one"),
				create("two"),
				create("three"),
				create("four"),
			},
			filter: "",
			expect: []*pbresource.Resource{
				create("one"),
				create("two"),
				create("three"),
				create("four"),
			},
		},
		"bad filter": {
			in: []*pbresource.Resource{
				create("one"),
				create("two"),
				create("three"),
				create("four"),
			},
			filter:    "garbage.value == zzz",
			expectErr: `Selector "garbage" is not valid`,
		},
		"filter everything out": {
			in: []*pbresource.Resource{
				create("one"),
				create("two"),
				create("three"),
				create("four"),
			},
			filter: "metadata.foo == bar",
		},
		"filter simply": {
			in: []*pbresource.Resource{
				create("one", "foo", "bar"),
				create("two", "foo", "baz"),
				create("three", "zim", "gir"),
				create("four", "zim", "gaz", "foo", "bar"),
			},
			filter: "metadata.foo == bar",
			expect: []*pbresource.Resource{
				create("one", "foo", "bar"),
				create("four", "zim", "gaz", "foo", "bar"),
			},
		},
		"filter prefix": {
			in: []*pbresource.Resource{
				create("one", "foo", "bar"),
				create("two", "foo", "baz"),
				create("three", "zim", "gir"),
				create("four", "zim", "gaz", "foo", "bar"),
				create("four", "zim", "zzz"),
			},
			filter: "(zim in metadata) and (metadata.zim matches `^g.`)",
			expect: []*pbresource.Resource{
				create("three", "zim", "gir"),
				create("four", "zim", "gaz", "foo", "bar"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestFilterMatchesResourceMetadata(t *testing.T) {
	type testcase struct {
		res       *pbresource.Resource
		filter    string
		expect    bool
		expectErr string
	}

	create := func(name string, kvs ...string) *pbresource.Resource {
		require.True(t, len(kvs)%2 == 0)

		meta := make(map[string]string)
		for i := 0; i < len(kvs); i += 2 {
			meta[kvs[i]] = kvs[i+1]
		}

		return &pbresource.Resource{
			Id: &pbresource.ID{
				Name: name,
			},
			Metadata: meta,
		}
	}

	run := func(t *testing.T, tc testcase) {
		got, err := FilterMatchesResourceMetadata(tc.res, tc.filter)
		if tc.expectErr != "" {
			require.Error(t, err)
			testutil.RequireErrorContains(t, err, tc.expectErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expect, got)
		}
	}

	cases := map[string]testcase{
		"nil input": {},
		"no filter": {
			res:    create("one"),
			filter: "",
			expect: true,
		},
		"bad filter": {
			res:       create("one"),
			filter:    "garbage.value == zzz",
			expectErr: `Selector "garbage" is not valid`,
		},
		"no match": {
			res:    create("one"),
			filter: "metadata.foo == bar",
		},
		"match simply": {
			res:    create("one", "foo", "bar"),
			filter: "metadata.foo == bar",
			expect: true,
		},
		"match via prefix": {
			res:    create("four", "zim", "gaz", "foo", "bar"),
			filter: "(zim in metadata) and (metadata.zim matches `^g.`)",
			expect: true,
		},
		"no match via prefix": {
			res:    create("four", "zim", "zzz", "foo", "bar"),
			filter: "(zim in metadata) and (metadata.zim matches `^g.`)",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
