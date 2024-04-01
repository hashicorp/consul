// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/testing/golden"
)

func goldenError(t *testing.T, name string, actual string) {
	t.Helper()

	expected := golden.Get(t, actual, name+".golden")
	require.Equal(t, expected, actual)
}

func TestErrorStrings(t *testing.T, cases map[string]error) {
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			goldenError(t, name, err.Error())
		})
	}
}

type UnwrapErrorTestCase struct {
	Err      error
	Expected error
}

func TestErrorUnwrap(t *testing.T, cases map[string]UnwrapErrorTestCase) {
	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.Expected, errors.Unwrap(tcase.Err))
		})
	}
}
