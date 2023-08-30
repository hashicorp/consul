// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

func goldenError(t *testing.T, name string, actual string) {
	t.Helper()

	fpath := filepath.Join("testdata", name+".golden")

	if *update {
		require.NoError(t, os.WriteFile(fpath, []byte(actual), 0644))
	} else {
		expected, err := os.ReadFile(fpath)
		require.NoError(t, err)
		require.Equal(t, string(expected), actual)
	}
}

func TestErrorStrings(t *testing.T) {
	type testCase struct {
		err      error
		expected string
	}

	fakeWrappedErr := fmt.Errorf("fake test error")

	cond := &pbresource.Condition{}

	cases := map[string]error{
		"ErrDataParse": NewErrDataParse(cond, fakeWrappedErr),
		"ErrInvalidField": ErrInvalidField{
			Name:    "host",
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidListElement": ErrInvalidListElement{
			Name:    "addresses",
			Index:   42,
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidMapKey": ErrInvalidMapKey{
			Map:     "ports",
			Key:     "http",
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidMapValue": ErrInvalidMapValue{
			Map:     "ports",
			Key:     "http",
			Wrapped: fakeWrappedErr,
		},
		"ErrOwnerInvalid": ErrOwnerTypeInvalid{
			ResourceType: &pbresource.Type{Group: "foo", GroupVersion: "v1", Kind: "bar"},
			OwnerType:    &pbresource.Type{Group: "other", GroupVersion: "v2", Kind: "something"},
		},
		"ErrInvalidReferenceType": ErrInvalidReferenceType{
			AllowedType: &pbresource.Type{Group: "foo", GroupVersion: "v1", Kind: "bar"},
		},
		"ErrMissing":                  ErrMissing,
		"ErrEmpty":                    ErrEmpty,
		"ErrReferenceTenancyNotEqual": ErrReferenceTenancyNotEqual,
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			goldenError(t, name, err.Error())
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	type testCase struct {
		err      error
		expected string
	}

	fakeWrappedErr := fmt.Errorf("fake test error")

	cases := map[string]error{
		"ErrDataParse": ErrDataParse{
			TypeName: "hashicorp.consul.catalog.v1alpha1.Service",
			Wrapped:  fakeWrappedErr,
		},
		"ErrInvalidField": ErrInvalidField{
			Name:    "host",
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidListElement": ErrInvalidListElement{
			Name:    "addresses",
			Index:   42,
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidMapKey": ErrInvalidMapKey{
			Map:     "ports",
			Key:     "http",
			Wrapped: fakeWrappedErr,
		},
		"ErrInvalidMapValue": ErrInvalidMapValue{
			Map:     "ports",
			Key:     "http",
			Wrapped: fakeWrappedErr,
		},
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, fakeWrappedErr, errors.Unwrap(err))
		})
	}
}
