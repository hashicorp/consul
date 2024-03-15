// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTValue(t *testing.T) {
	t.Run("String: set", func(t *testing.T) {
		var tv TValue[string]

		err := tv.Set("testString")
		require.NoError(t, err)

		assert.Equal(t, "testString", *tv.v)
	})

	t.Run("String: merge", func(t *testing.T) {
		var tv TValue[string]
		var onto string

		testStr := "testString"
		tv.v = &testStr
		tv.Merge(&onto)

		assert.Equal(t, "testString", onto)
	})

	t.Run("String: merge nil", func(t *testing.T) {
		var tv TValue[string]
		var onto *string = nil

		testStr := "testString"
		tv.v = &testStr
		err := tv.Merge(onto)

		assert.Equal(t, "onto is nil", err.Error())
	})

	t.Run("Get string", func(t *testing.T) {
		var tv TValue[string]
		testStr := "testString"
		tv.v = &testStr
		assert.Equal(t, "testString", tv.String())
	})

	t.Run("Bool: set", func(t *testing.T) {
		var tv TValue[bool]

		err := tv.Set("true")
		require.NoError(t, err)

		assert.True(t, *tv.v)
	})

	t.Run("Bool: merge", func(t *testing.T) {
		var tv TValue[bool]
		var onto bool

		testBool := true
		tv.v = &testBool
		tv.Merge(&onto)

		assert.True(t, onto)
	})
}
