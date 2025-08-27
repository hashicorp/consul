// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/testing/errors"
)

var (
	errFakeWrapped = fmt.Errorf("fake test error")
)

func TestErrorStrings(t *testing.T) {
	errors.TestErrorStrings(t, map[string]error{
		"IndexNotFound": IndexNotFoundError{name: "fake"},
		"QueryNotFound": QueryNotFoundError{name: "fake"},
		"QueryRequired": QueryRequired,
		"CacheTypeError": CacheTypeError{
			err: errFakeWrapped,
			it: unversionedType{
				Group: "something",
				Kind:  "else",
			},
		},
		"IndexError": IndexError{
			err:  errFakeWrapped,
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
				err:  errFakeWrapped,
			},
			Expected: errFakeWrapped,
		},
		"CacheTypeError": {
			Err: CacheTypeError{
				it:  unversionedType{Group: "something", Kind: "else"},
				err: errFakeWrapped,
			},
			Expected: errFakeWrapped,
		},
	})
}
