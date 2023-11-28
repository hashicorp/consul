// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/testing/errors"
)

var (
	fakeWrappedErr = fmt.Errorf("fake test error")
)

func TestErrorStrings(t *testing.T) {
	errors.TestErrorStrings(t, map[string]error{
		"IndexNotFound": IndexNotFoundError{name: "fake"},
		"QueryNotFound": QueryNotFoundError{name: "fake"},
		"QueryRequired": QueryRequired,
		"CacheTypeError": CacheTypeError{
			err: fakeWrappedErr,
			it: unversionedType{
				Group: "something",
				Kind:  "else",
			},
		},
		"IndexError": IndexError{
			err:  fakeWrappedErr,
			name: "foo",
		},
		"DuplicateIndexError": DuplicateIndexError{
			name: "addresses",
		},
		"DuplicateQueryError": DuplicateQueryError{
			name: "addresses",
		},
	})
}

func TestErrorUnwrap(t *testing.T) {
	errors.TestErrorUnwrap(t, map[string]errors.UnwrapErrorTestCase{
		"IndexError": {
			Err: IndexError{
				name: "blah",
				err:  fakeWrappedErr,
			},
			Expected: fakeWrappedErr,
		},
		"CacheTypeError": {
			Err: CacheTypeError{
				it:  unversionedType{Group: "something", Kind: "else"},
				err: fakeWrappedErr,
			},
			Expected: fakeWrappedErr,
		},
	})
}
