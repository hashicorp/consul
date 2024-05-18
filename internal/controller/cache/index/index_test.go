// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/index/indexmock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

type testSingleIndexer struct{}

func (testSingleIndexer) FromArgs(args ...any) ([]byte, error) {
	return ReferenceOrIDFromArgs(args...)
}

func (testSingleIndexer) FromResource(res *pbresource.Resource) (bool, []byte, error) {
	return true, IndexFromRefOrID(res.Id), nil
}

type testMultiIndexer struct{}

func (testMultiIndexer) FromArgs(args ...any) ([]byte, error) {
	return ReferenceOrIDFromArgs(args...)
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

func TestIndexReuse(t *testing.T) {
	rtype := &pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "fake",
	}
	id := resourcetest.Resource(rtype, "foo").ID()

	res1 := resourcetest.ResourceID(id).Build()
	res2 := resourcetest.ResourceID(id).WithStatus("foo", &pbresource.Status{
		ObservedGeneration: "woo",
	}).Build()

	indexer := testSingleIndexer{}

	// Verify that the indexer produces an identical index for both resources. If this
	// isn't true then the rest of the checks we do don't actually prove that the
	// two IndexedData objects have independent resource storage.
	_, idx1, _ := indexer.FromResource(res1)
	_, idx2, _ := indexer.FromResource(res2)
	require.Equal(t, idx1, idx2)

	// Create the index and two indepent indexed data storage objects
	idx := New("test", indexer)
	data1 := idx.IndexedData()
	data2 := idx.IndexedData()

	// Push 1 resource into each
	txn := data1.Txn()
	txn.Insert(res1)
	txn.Commit()

	txn = data2.Txn()
	txn.Insert(res2)
	txn.Commit()

	// Verify that querying the first indexed data can only return the first resource
	iter, err := data1.Txn().ListIterator(id)
	require.NoError(t, err)
	res := iter.Next()
	prototest.AssertDeepEqual(t, res1, res)
	require.Nil(t, iter.Next())

	// Verify that querying the second indexed data can only return the second resource
	iter, err = data2.Txn().ListIterator(id)
	require.NoError(t, err)
	res = iter.Next()
	prototest.AssertDeepEqual(t, res2, res)
	require.Nil(t, iter.Next())
}
