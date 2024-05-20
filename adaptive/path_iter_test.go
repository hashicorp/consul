// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"reflect"
	"sort"
	"testing"
)

func TestPathIterator(t *testing.T) {
	r := NewRadixTree[any]()

	keys := []string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/baz/bar",
		"foo/zip/zap",
		"zipzap",
	}
	for _, k := range keys {
		_ = r.Insert([]byte(k), nil)
	}
	if int(r.size) != len(keys) {
		t.Fatalf("bad len: %v %v", r.size, len(keys))
	}

	type exp struct {
		inp string
		out []string
	}
	cases := []exp{
		{
			"f",
			[]string{},
		},
		{
			"foo",
			[]string{"foo"},
		},
		{
			"foo/",
			[]string{"foo"},
		},
		{
			"foo/ba",
			[]string{"foo"},
		},
		{
			"foo/bar",
			[]string{"foo", "foo/bar"},
		},
		{
			"foo/bar/baz",
			[]string{"foo", "foo/bar", "foo/bar/baz"},
		},
		{
			"foo/bar/bazoo",
			[]string{"foo", "foo/bar", "foo/bar/baz"},
		},
		{
			"z",
			[]string{},
		},
	}

	for _, test := range cases {
		iter := r.GetPathIterator([]byte(test.inp))

		// Radix tree iteration on our string indices will return values with keys in
		// ascending order. So before we check the iteration ordering we must sort
		// the expected outputs.
		sort.Strings(test.out)

		// verify that all the expected values come out in ascending order.
		for idx, expected := range test.out {
			actual, _, found := iter.Next()
			// ensure we found a value (i.e. iteration is finding the correct number of values)
			if !found {
				t.Fatalf("iteration returned fewer values than expected: %d, actual: %d", len(test.out), idx)
			}

			// ensure that the values are coming out in the sorted order.
			if !reflect.DeepEqual([]byte(expected), actual) {
				t.Errorf("expected: %#v", []byte(expected))
				t.Errorf("actual:   %#v", actual)
				t.Fatalf("value returned during iteration doesn't match our expectation (iteration num: %d)", idx)
			}
		}

		// Now ensure there are no trailing values that the tree will output.
		_, _, found := iter.Next()
		if found {
			t.Fatalf("iteration returned more values than expected: %d, actual: %d+", len(test.out), len(test.out)+1)
		}

		// Verify that continued calls to next on a completed Iterator do not panic or return values.
		_, _, found = iter.Next()
		if found {
			t.Fatalf("iteration returned a value after previously indicating iteration was complete")
		}
	}
}
