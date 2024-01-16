// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/index/indexmock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

type testSingleIndexer struct{}

func (testSingleIndexer) FromArgs(args ...any) ([]byte, error) {
	return ReferenceOrIDFromArgs(args)
}

func (testSingleIndexer) FromResource(*pbresource.Resource) (bool, []byte, error) {
	return false, nil, nil
}

type testMultiIndexer struct{}

func (testMultiIndexer) FromArgs(args ...any) ([]byte, error) {
	return ReferenceOrIDFromArgs(args)
}

func (testMultiIndexer) FromResource(*pbresource.Resource) (bool, [][]byte, error) {
	return false, nil, nil
}

type argsOnlyIdx struct{}

func (argsOnlyIdx) FromArgs(args ...any) ([]byte, error) {
	return nil, nil
}

func TestNew(t *testing.T) {
	t.Run("no name", func(t *testing.T) {
		require.Panics(t, func() {
			New("", testSingleIndexer{})
		})
	})

	t.Run("nil indexer", func(t *testing.T) {
		require.Panics(t, func() {
			New("test", nil)
		})
	})

	t.Run("indexer interface not satisfied", func(t *testing.T) {
		require.Panics(t, func() {
			New("test", argsOnlyIdx{})
		})
	})

	t.Run("single indexer", func(t *testing.T) {
		require.NotNil(t, New("test", testSingleIndexer{}))
	})

	t.Run("multi indexer", func(t *testing.T) {
		require.NotNil(t, New("test", testMultiIndexer{}))
	})

	t.Run("required", func(t *testing.T) {
		idx := New("test", testSingleIndexer{}, IndexRequired)
		require.NotNil(t, idx)
		require.True(t, idx.required)
		require.Equal(t, "test", idx.Name())
	})
}

func TestSingleIndexWrapper(t *testing.T) {
	injectedError := errors.New("injected")
	rtype := &pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "fake",
	}
	res := resourcetest.Resource(rtype, "foo").Build()

	t.Run("FromArgs ok", func(t *testing.T) {
		m := indexmock.NewSingleIndexer(t)
		wrapper := singleIndexWrapper{indexer: m}

		m.On("FromArgs", 1).Return([]byte{1, 2, 3}, nil)
		vals, err := wrapper.FromArgs(1)
		require.NoError(t, err)
		require.Equal(t, []byte{1, 2, 3}, vals)
	})

	t.Run("FromArgs err", func(t *testing.T) {
		m := indexmock.NewSingleIndexer(t)
		wrapper := singleIndexWrapper{indexer: m}

		m.On("FromArgs", 1).Return([]byte(nil), injectedError)
		vals, err := wrapper.FromArgs(1)
		require.Error(t, err)
		require.ErrorIs(t, err, injectedError)
		require.Nil(t, vals)
	})

	t.Run("FromResource err", func(t *testing.T) {
		m := indexmock.NewSingleIndexer(t)
		wrapper := singleIndexWrapper{indexer: m}
		m.On("FromResource", res).Return(false, []byte(nil), injectedError)
		indexed, vals, err := wrapper.FromResource(res)
		require.False(t, indexed)
		require.Nil(t, vals)
		require.ErrorIs(t, err, injectedError)
	})

	t.Run("FromResource not indexed", func(t *testing.T) {
		m := indexmock.NewSingleIndexer(t)
		wrapper := singleIndexWrapper{indexer: m}
		m.On("FromResource", res).Return(false, []byte(nil), nil)
		indexed, vals, err := wrapper.FromResource(res)
		require.False(t, indexed)
		require.Nil(t, vals)
		require.Nil(t, err)
	})

	t.Run("FromResource ok", func(t *testing.T) {
		m := indexmock.NewSingleIndexer(t)
		wrapper := singleIndexWrapper{indexer: m}
		m.On("FromResource", res).Return(true, []byte{1, 2, 3}, nil)
		indexed, vals, err := wrapper.FromResource(res)
		require.NoError(t, err)
		require.True(t, indexed)
		require.Len(t, vals, 1)
		require.Equal(t, []byte{1, 2, 3}, vals[0])
	})
}
