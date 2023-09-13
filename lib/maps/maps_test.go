// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package maps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSliceOfKeys(t *testing.T) {
	t.Run("string to int", func(t *testing.T) {
		m := make(map[string]int)
		require.Equal(t, []string(nil), SliceOfKeys(m))
		m["foo"] = 5
		m["bar"] = 6
		require.ElementsMatch(t, []string{"foo", "bar"}, SliceOfKeys(m))
	})

	type blah struct {
		V string
	}

	t.Run("int to struct", func(t *testing.T) {
		m := make(map[int]blah)
		require.Equal(t, []int(nil), SliceOfKeys(m))
		m[5] = blah{V: "foo"}
		m[6] = blah{V: "bar"}
		require.ElementsMatch(t, []int{5, 6}, SliceOfKeys(m))
	})

	type id struct {
		Name string
	}

	t.Run("struct to struct pointer", func(t *testing.T) {
		m := make(map[id]*blah)
		require.Equal(t, []id(nil), SliceOfKeys(m))
		m[id{Name: "foo"}] = &blah{V: "oof"}
		m[id{Name: "bar"}] = &blah{V: "rab"}
		require.ElementsMatch(t, []id{{Name: "foo"}, {Name: "bar"}}, SliceOfKeys(m))
	})
}

func TestSliceOfValues(t *testing.T) {
	t.Run("string to int", func(t *testing.T) {
		m := make(map[string]int)
		require.Equal(t, []int(nil), SliceOfValues(m))
		m["foo"] = 5
		m["bar"] = 6
		require.ElementsMatch(t, []int{5, 6}, SliceOfValues(m))
	})

	type blah struct {
		V string
	}

	t.Run("int to struct", func(t *testing.T) {
		m := make(map[int]blah)
		require.Equal(t, []blah(nil), SliceOfValues(m))
		m[5] = blah{V: "foo"}
		m[6] = blah{V: "bar"}
		require.ElementsMatch(t, []blah{{V: "foo"}, {V: "bar"}}, SliceOfValues(m))
	})

	type id struct {
		Name string
	}

	t.Run("struct to struct pointer", func(t *testing.T) {
		m := make(map[id]*blah)
		require.Equal(t, []*blah(nil), SliceOfValues(m))
		m[id{Name: "foo"}] = &blah{V: "oof"}
		m[id{Name: "bar"}] = &blah{V: "rab"}
		require.ElementsMatch(t, []*blah{{V: "oof"}, {V: "rab"}}, SliceOfValues(m))
	})
}
