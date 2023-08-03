// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package prototest

import (
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

type TestingT interface {
	Helper()
	Fatalf(string, ...any)
}

func AssertDeepEqual(t TestingT, x, y interface{}, opts ...cmp.Option) {
	t.Helper()

	opts = append(opts, protocmp.Transform())

	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}

// AssertElementsMatch asserts that the specified listX(array, slice...) is
// equal to specified listY(array, slice...) ignoring the order of the
// elements. If there are duplicate elements, the number of appearances of each
// of them in both lists should match.
//
// prototest.AssertElementsMatch(t, [1, 3, 2, 3], [1, 3, 3, 2])
func AssertElementsMatch[V any](
	t TestingT, listX, listY []V, opts ...cmp.Option,
) {
	diff := diffElements(listX, listY, opts...)
	if diff != "" {
		t.Fatalf("assertion failed: slices do not have matching elements\n--- expected\n+++ actual\n%v", diff)
	}
}

func diffElements[V any](
	listX, listY []V, opts ...cmp.Option,
) string {
	if len(listX) == 0 && len(listY) == 0 {
		return ""
	}

	opts = append(opts, protocmp.Transform())

	if len(listX) != len(listY) {
		return cmp.Diff(listX, listY, opts...)
	}

	// dump into a map keyed by sliceID
	mapX := make(map[int]V)
	for i, val := range listX {
		mapX[i] = val
	}

	mapY := make(map[int]V)
	for i, val := range listY {
		mapY[i] = val
	}

	var outX, outY []V
	for i, itemX := range mapX {
		for j, itemY := range mapY {
			if diff := cmp.Diff(itemX, itemY, opts...); diff == "" {
				outX = append(outX, itemX)
				outY = append(outY, itemY)
				delete(mapX, i)
				delete(mapY, j)
			}
		}
	}

	if len(outX) == len(listX) && len(outY) == len(listY) {
		return "" // matches
	}

	// dump remainder into the slice so we can generate a useful error
	for _, itemX := range mapX {
		outX = append(outX, itemX)
	}
	for _, itemY := range mapY {
		outY = append(outY, itemY)
	}

	return cmp.Diff(outX, outY, opts...)
}

func AssertContainsElement[V any](t TestingT, list []V, element V, opts ...cmp.Option) {
	t.Helper()

	opts = append(opts, protocmp.Transform())

	for _, e := range list {
		if cmp.Equal(e, element, opts...) {
			return
		}
	}

	t.Fatalf("assertion failed: list does not contain element\n--- list\n%#v\n--- element: %#v", list, element)
}
