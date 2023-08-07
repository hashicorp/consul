// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package prototest

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type wrap struct {
	V int
	O string
}

func (w *wrap) String() string {
	return strconv.Itoa(w.V)
}

func (w *wrap) GoString() string {
	return w.String()
}

func TestDiffElements_noProtobufs(t *testing.T) {
	// NOTE: this test only tests non-protobuf slices initially

	type testcase struct {
		a, b    []*wrap
		notSame bool
	}

	run := func(t *testing.T, tc testcase) {
		diff := diffElements(tc.a, tc.b)
		if tc.notSame {
			require.False(t, diff == "", "expected not to be the same")
		} else {
			require.True(t, diff == "", "expected to be the same")
		}
	}

	w := func(v int) *wrap {
		return &wrap{V: v}
	}

	cases := map[string]testcase{
		"nil":           {},
		"empty":         {a: []*wrap{}, b: []*wrap{}},
		"nil and empty": {a: []*wrap{}, b: nil},
		"ordered match": {
			a: []*wrap{w(1), w(22), w(303), w(43004), w(-5)},
			b: []*wrap{w(1), w(22), w(303), w(43004), w(-5)},
		},
		"permuted match": {
			a: []*wrap{w(1), w(22), w(303), w(43004), w(-5)},
			b: []*wrap{w(-5), w(43004), w(303), w(22), w(1)},
		},
		"duplicates": {
			a: []*wrap{w(1), w(2), w(2), w(3)},
			b: []*wrap{w(2), w(1), w(3), w(2)},
		},
		// no match
		"1 vs nil": {
			a:       []*wrap{w(1)},
			b:       nil,
			notSame: true,
		},
		"1 vs 2": {
			a:       []*wrap{w(1)},
			b:       []*wrap{w(2)},
			notSame: true,
		},
		"1,2 vs 2,3": {
			a:       []*wrap{w(1), w(2)},
			b:       []*wrap{w(2), w(3)},
			notSame: true,
		},
		"1,2 vs 1,2,3": {
			a:       []*wrap{w(1), w(2)},
			b:       []*wrap{w(1), w(2), w(3)},
			notSame: true,
		},
		"duplicates omitted": {
			a:       []*wrap{w(1), w(2), w(2), w(3)},
			b:       []*wrap{w(1), w(3), w(2)},
			notSame: true,
		},
	}

	allCases := make(map[string]testcase)
	for name, tc := range cases {
		allCases[name] = tc
		allCases[name+" (flipped)"] = testcase{
			a:       tc.b,
			b:       tc.a,
			notSame: tc.notSame,
		}
	}

	for name, tc := range allCases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
